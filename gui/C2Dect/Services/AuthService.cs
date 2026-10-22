using System;
using System.IO;
using System.Net.Http;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using System.Threading.Tasks;
using C2Dect.Models;

namespace C2Dect.Services;

/// <summary>
/// Sesion del operador.
///
/// El access token vive solo en memoria. El refresh token se guarda en disco
/// PROTEGIDO con DPAPI (CurrentUser): solo la cuenta de Windows que inicio
/// sesion puede leerlo, y no se puede reutilizar en otra maquina.
///
/// La sesion se restaura sola al abrir la consola: si el refresh token sigue
/// vivo, se pide un access token nuevo y se entra sin volver a escribir la
/// clave.
/// </summary>
public class AuthService
{
    private readonly ApiService _api;
    private string? _token;
    private string? _refreshToken;
    private Operator? _currentOperator;
    private Tenant? _currentTenant;
    private System.Timers.Timer? _refreshTimer;

    public AuthService(ApiService api)
    {
        _api = api;
    }

    public bool IsAuthenticated => !string.IsNullOrEmpty(_token);
    public Operator? CurrentOperator => _currentOperator;
    public Tenant? CurrentTenant => _currentTenant;
    public string? Token => _token;
    public string? RefreshToken => _refreshToken;

    /// <summary>
    /// Rol efectivo del operador. OJO: el servidor NO particiona operators.role
    /// por tenant, asi que un "admin" ve y opera sobre TODOS los tenants. Es
    /// deuda del backend, no de la GUI: la GUI lo dice en voz alta en vez de
    /// simular un aislamiento que el servidor no aplica.
    /// </summary>
    public bool IsGlobalAdmin =>
        string.Equals(CurrentOperator?.Role, "admin", StringComparison.OrdinalIgnoreCase);

    /// <summary>Tenant contra el que resuelve el servidor el token.</summary>
    public long TokenTenantId
    {
        get
        {
            long tid = CurrentOperator?.TenantId ?? 0;
            return tid > 0 ? tid : 1;
        }
    }

    public string GlobalDataNotice =>
        !IsAuthenticated
            ? "Metricas globales (no acotadas por tenant): el servidor las calcula sobre TODOS los tenants."
            : IsGlobalAdmin
                ? "Metricas globales (no acotadas por tenant): incluyen los datos de TODOS los tenants (rol admin global)."
                : $"Metricas globales (no acotadas por tenant): el servidor las calcula sobre TODOS los tenants aunque tu token este acotado al tenant #{TokenTenantId}.";

    public string TokenScopeNotice =>
        IsGlobalAdmin
            ? $"Rol admin global (operators.role no esta particionado): alcance sobre el tenant #{TokenTenantId} y el resto de tenants."
            : $"Operador acotado al tenant #{TokenTenantId} del token.";

    // ------------------------------------------------------------- login

    public async Task<(bool ok, string error)> Login(string username, string password, string? totpCode = null)
    {
        try
        {
            var result = await _api.Login(username, password, totpCode);
            if (result == null) return (false, "Respuesta vacia del servidor.");

            _token = result.Value.token;
            _refreshToken = result.Value.refreshToken;
            _currentOperator = result.Value.op;
            _api.Configure(_api.BaseUrl ?? "", _token);
            StartRefreshTimer();
            SaveSession();
            return (true, "");
        }
        catch (HttpRequestException ex)
        {
            string msg = ex.Message;
            if (msg.Contains("invalid credentials")) return (false, "Credenciales invalidas (usuario/clave).");
            if (msg.Contains("account locked")) return (false, "Cuenta bloqueada temporalmente.");
            if (msg.Contains("2fa required") || msg.Contains("totp")) return (false, "TOTP requerido: ingresa codigo de 6 digitos");
            if (msg.Contains("invalid 2fa")) return (false, "Codigo TOTP invalido");
            return (false, msg.Length > 200 ? msg[..200] : msg);
        }
        catch (Exception ex)
        {
            return (false, ex.Message);
        }
    }

    /// <summary>
    /// Restaura la sesion guardada. Devuelve false si no hay sesion, si el
    /// refresh token ya no vale o si el servidor no responde.
    /// </summary>
    public async Task<bool> TryRestoreAsync()
    {
        try
        {
            var stored = ReadSession();
            if (stored == null || string.IsNullOrEmpty(stored.RefreshToken)) return false;

            if (!string.IsNullOrEmpty(stored.ServerUrl)) _api.BaseUrl = stored.ServerUrl;

            var refreshed = await _api.Refresh(stored.RefreshToken);
            if (string.IsNullOrEmpty(refreshed)) return false;

            _token = refreshed;
            _refreshToken = stored.RefreshToken;
            _api.Configure(_api.BaseUrl ?? "", _token);

            var me = await _api.GetMe();
            _currentOperator = me?.Operator;
            _currentTenant = me?.Tenant;
            if (_currentOperator != null) _api.ActiveTenantId = _currentOperator.TenantId;

            StartRefreshTimer();
            Logger.Log("Session restored from the local store");
            return true;
        }
        catch (Exception ex)
        {
            Logger.Log("Session restore failed: " + ex.Message);
            DeleteSessionFile();
            return false;
        }
    }

    // ------------------------------------------------------------- logout

    /// <summary>
    /// Logout REAL: revoca el refresh token en el servidor y despues limpia
    /// el estado local (incluido el fichero protegido con DPAPI).
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
            }
            catch (Exception ex)
            {
                Logger.LogError("Server logout failed", ex);
            }
        }
        Logout();
        return ok;
    }

    /// <summary>Logout SOLO local: descarta token, timer y fichero guardado.</summary>
    public void Logout()
    {
        StopRefreshTimer();
        _token = null;
        _refreshToken = null;
        _currentOperator = null;
        _currentTenant = null;
        _api.ClearToken();
        DeleteSessionFile();
    }

    // ------------------------------------------------------------ refresh

    private void StartRefreshTimer()
    {
        StopRefreshTimer();
        _refreshTimer = new System.Timers.Timer { Interval = 10 * 60 * 1000, AutoReset = true };
        _refreshTimer.Elapsed += async (s, e) => await RefreshSessionAsync();
        _refreshTimer.Start();
    }

    private void StopRefreshTimer()
    {
        if (_refreshTimer == null) return;
        _refreshTimer.Stop();
        _refreshTimer.Dispose();
        _refreshTimer = null;
    }

    private async Task RefreshSessionAsync()
    {
        if (string.IsNullOrEmpty(_refreshToken)) return;
        try
        {
            var refreshed = await _api.Refresh(_refreshToken);
            if (string.IsNullOrEmpty(refreshed)) return;
            _token = refreshed;
            _api.Configure(_api.BaseUrl ?? "", _token);
            SaveSession();
        }
        catch (Exception ex)
        {
            Logger.Log("Session refresh failed: " + ex.Message);
        }
    }

    // ------------------------------------------------- persistencia DPAPI

    private sealed class StoredSession
    {
        public string ServerUrl { get; set; } = "";
        public string RefreshToken { get; set; } = "";
    }

    private static string SessionFile => Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
        "C2Dect", "session.bin");

    private static byte[] Entropy => Encoding.UTF8.GetBytes("c2-dect-lite/2.0.0");

    /// <summary>Guarda el refresh token protegido con DPAPI.</summary>
    private void SaveSession()
    {
        try
        {
            if (string.IsNullOrEmpty(_refreshToken)) return;
            var payload = new StoredSession
            {
                ServerUrl = _api.BaseUrl ?? "",
                RefreshToken = _refreshToken,
            };
            var plain = JsonSerializer.SerializeToUtf8Bytes(payload);
            var protectedBytes = ProtectedData.Protect(plain, Entropy, DataProtectionScope.CurrentUser);
            Directory.CreateDirectory(Path.GetDirectoryName(SessionFile)!);
            File.WriteAllBytes(SessionFile, protectedBytes);
        }
        catch (Exception ex)
        {
            Logger.Log("Could not store the session: " + ex.Message);
        }
    }

    private static StoredSession? ReadSession()
    {
        try
        {
            if (!File.Exists(SessionFile)) return null;
            var protectedBytes = File.ReadAllBytes(SessionFile);
            var plain = ProtectedData.Unprotect(protectedBytes, Entropy, DataProtectionScope.CurrentUser);
            return JsonSerializer.Deserialize<StoredSession>(plain);
        }
        catch
        {
            return null;
        }
    }

    private static void DeleteSessionFile()
    {
        try { if (File.Exists(SessionFile)) File.Delete(SessionFile); } catch { }
    }
}
