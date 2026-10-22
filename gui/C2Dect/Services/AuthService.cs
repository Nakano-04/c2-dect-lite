using System;
using System.Net.Http;
using System.Threading.Tasks;
using C2Dect.Models;

namespace C2Dect.Services;

public class AuthService
{
    private readonly ApiService _api;
    private string? _token;
    private string? _refreshToken;
    private Operator? _currentOperator;
    private System.Timers.Timer? _refreshTimer;

    public AuthService(ApiService api)
    {
        _api = api;
    }

    public bool IsAuthenticated => !string.IsNullOrEmpty(_token);
    public Operator? CurrentOperator => _currentOperator;
    public string? Token => _token;
    public string? RefreshToken => _refreshToken;

    /// <summary>
    /// Rol efectivo del operador. OJO: el servidor NO particiona operators.role
    /// por tenant (server/api/handlers.go:915, callerIsAdmin), asi que un
    /// "admin" ve y opera sobre TODOS los tenants. Es deuda del backend, no de
    /// la GUI: la GUI lo dice en voz alta en vez de simular un aislamiento que
    /// el servidor no aplica.
    /// </summary>
    public bool IsGlobalAdmin =>
        string.Equals(CurrentOperator?.Role, "admin", StringComparison.OrdinalIgnoreCase);

    /// <summary>
    /// Claim "tid" del token: el tenant contra el que el servidor resuelve
    /// requireSessionAccess (server/api/handlers.go:899, callerTenantID).
    /// Si el operador no trae tenant, el servidor cae en 1.
    /// </summary>
    public long TokenTenantId
    {
        get
        {
            long tid = CurrentOperator?.TenantId ?? 0;
            return tid > 0 ? tid : 1;
        }
    }

    /// <summary>
    /// Aviso que la UI muestra junto a los numeros que el servidor NO acota por
    /// tenant: GET /api/metrics (handleMetrics, server/api/metrics_reports.go:12)
    /// y GET /api/reports/executive (handleExecutiveReport,
    /// server/api/metrics_reports.go:67) iteran db.ListSessions("") y
    /// db.GetActivity(...) sin filtro de tenant, igual que GET /api/stats
    /// (handleStats, server/api/handlers.go:1366). Sin este aviso el operador
    /// leeria datos de otros tenants como si fueran suyos.
    /// </summary>
    public string GlobalDataNotice =>
        !IsAuthenticated
            ? "Métricas globales (no acotadas por tenant): el servidor las calcula sobre TODOS los tenants."
            : IsGlobalAdmin
                ? "Métricas globales (no acotadas por tenant): incluyen los datos de TODOS los tenants (rol admin global)."
                : $"Métricas globales (no acotadas por tenant): el servidor las calcula sobre TODOS los tenants aunque tu token esté acotado al tenant #{TokenTenantId}.";

    /// <summary>
    /// Descripcion corta del alcance efectivo del token, para los banners de
    /// la UI. No sustituye a TenantService.AccessNotice (que additionally
    /// distingue el tenant seleccionado del tenant del token).
    /// </summary>
    public string TokenScopeNotice =>
        IsGlobalAdmin
            ? $"Rol admin global (operators.role no está particionado): alcance sobre el tenant #{TokenTenantId} y el resto de tenants."
            : $"Operador acotado al tenant #{TokenTenantId} del token.";

    public async Task<(bool ok, string error)> Login(string username, string password, string? totpCode = null)
    {
        try
        {
            var result = await _api.Login(username, password, totpCode);
            if (result == null) return (false, "Respuesta vacía del servidor.");

            _token = result.Value.token;
            _refreshToken = result.Value.refreshToken;
            _currentOperator = result.Value.op;
            _api.Configure(_api.BaseUrl ?? "", _token);
            StartRefreshTimer();
            return (true, "");
        }
        catch (HttpRequestException ex)
        {
            string msg = ex.Message;
            if (msg.Contains("invalid credentials")) return (false, "Credenciales inválidas (usuario/clave). Usa admin / Admin123456!");
            if (msg.Contains("account locked"))
            {
                string wait = ParseLockedWait(msg);
                return (false, $"Cuenta bloqueada{wait}. Si eres el admin local, corre en server/: go run ./cmd --unlock admin y reintenta. No reintentes seguido.");
            }
            if (msg.Contains("2fa required") || msg.Contains("totp")) return (false, "TOTP requerido: ingresa código de 6 dígitos");
            if (msg.Contains("invalid 2fa")) return (false, "Código TOTP inválido");
            return (false, msg.Length > 200 ? msg[..200] : msg);
        }
        catch (Exception ex)
        {
            return (false, ex.Message);
        }
    }

    private void StartRefreshTimer()
    {
        _refreshTimer?.Dispose();
        _refreshTimer = new System.Timers.Timer(14 * 60 * 1000);
        _refreshTimer.Elapsed += async (s, e) => {
            if (_refreshToken != null) {
                var newToken = await _api.Refresh(_refreshToken);
                if (newToken != null) {
                    _token = newToken;
                    _api.Configure(_api.BaseUrl ?? "", _token);
                }
            }
        };
        _refreshTimer.AutoReset = true;
        _refreshTimer.Start();
    }

    /// <summary>
    /// Logout REAL: revoca en el servidor el refresh token y el access token
    /// (POST /api/auth/logout) y después limpia el estado local. Antes el
    /// logout era únicamente local: el token seguía vivo en el servidor y el
    /// refresh timer seguía corriendo.
    /// Devuelve false si el servidor no confirmó la revocación (el estado
    /// local se limpia igual: la sesión del operador se cierra en la GUI).
    /// </summary>
    public async Task<bool> LogoutAsync()
    {
        bool ok = false;
        string? rt = _refreshToken;
        if (!string.IsNullOrEmpty(rt))
        {
            try
            {
                ok = await _api.Logout(rt);
                if (!ok) Logger.Log("Server logout rejected: " + (_api.LastError ?? "sin detalle"));
            }
            catch (Exception ex)
            {
                Logger.LogError("Server logout failed", ex);
                ok = false;
            }
        }
        Logout();
        return ok;
    }

    /// <summary>
    /// Logout SOLO local: descarta token, refresh token y timer. No toca el
    /// servidor. Usar <see cref="LogoutAsync"/> para cerrar la sesión de verdad.
    /// </summary>
    public void Logout()
    {
        _refreshTimer?.Dispose();
        _refreshTimer = null;
        _token = null;
        _refreshToken = null;
        _currentOperator = null;
    }

    private static string ParseLockedWait(string msg)
    {
        try
        {
            int i = msg.IndexOf("locked_until", StringComparison.OrdinalIgnoreCase);
            if (i < 0) return " - espera ~15 min";
            int q1 = msg.IndexOf('"', i);
            int q2 = msg.IndexOf('"', q1 + 1);
            string ts = q1 >= 0 && q2 > q1 ? msg.Substring(q1 + 1, q2 - q1 - 1) : "";
            // también formato 2026-..T..Z sin comillas
            if (string.IsNullOrEmpty(ts))
            {
                var m = System.Text.RegularExpressions.Regex.Match(msg, @"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z?");
                ts = m.Success ? m.Value : "";
            }
            if (DateTimeOffset.TryParse(ts, out var until))
            {
                var mins = Math.Max(1, (int)Math.Ceiling((until - DateTimeOffset.UtcNow).TotalMinutes));
                return $" - reintenta en ~{mins} min (hasta {until:HH:mm} UTC)";
            }
        }
        catch { }
        return " - espera ~15 min";
    }
}