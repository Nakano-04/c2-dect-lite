using CommunityToolkit.Mvvm.ComponentModel;
using C2Dect.Models;
using Newtonsoft.Json;
using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;

namespace C2Dect.Services;

/// <summary>
/// Captura la generaciÃ³n de tenant vigente en el momento de lanzar un fetch.
/// Si el operador cambia de tenant mientras la peticiÃ³n estÃ¡ en vuelo, IsCurrent
/// pasa a false y la vista descarta la respuesta en lugar de pintar datos del
/// tenant anterior.
/// </summary>
public sealed class TenantScopeToken
{
    private readonly TenantService _owner;

    internal TenantScopeToken(TenantService owner, long generation, long tenantId)
    {
        _owner = owner;
        Generation = generation;
        TenantId = tenantId;
    }

    public long Generation { get; }
    public long TenantId { get; }
    public bool IsCurrent => _owner.IsGeneration(Generation);
    public bool CanPaint => IsCurrent && _owner.CanOperate;
}

/// <summary>
/// Contexto de tenant de la GUI. El servidor es multi-tenant desde el claim JWT;
/// esta clase es la Ãºnica que decide quÃ© tenant estÃ¡ activo y la que publica el
/// filtro a <see cref="ApiService.ActiveTenantId"/>.
///
/// Contrato con el servidor Go (leÃ­do, no asumido):
///  - El tenant viaja en el claim "tid" del JWT (server/auth/jwt.go:39), nunca
///    en un header.
///  - callerTenantID (server/api/handlers.go:899) usa ese claim; sin claim
///    resuelve por DB y cae en 1.
///  - callerIsAdmin (server/api/handlers.go:915) es role == "admin", y
///    operators.role NO estÃ¡ particionado por tenant: un admin de un tenant es
///    admin de todos. Deuda del servidor, no de la GUI.
///  - "?tenant_id=" solo lo honra un admin (server/api/handlers.go:956 para
///    /sessions, :1259 para /loot). Para un no-admin el parÃ¡metro se ignora en
///    silencio y el listado queda acotado al tenant del token.
///  - resolveTenantScope (server/api/multitenancy.go:14) devuelve 404 (no
///    oracular) y audita cross_tenant_access_attempt si un no-admin pide otro
///    tenant en categories/permissions/roles/users.
///  - GET /api/tenants no estÃ¡ scopeado: cualquier operador autenticado ve el
///    nombre y el id de todos los tenants.
///  - allow_impact_modules se evalÃºa contra el tenant del TOKEN, no contra el
///    seleccionado (server/api/handlers.go:1500).
/// </summary>
public sealed partial class TenantService : ObservableObject
{
    private readonly ApiService _api;
    private readonly AuthService _auth;
    private readonly SemaphoreSlim _gate = new(1, 1);
    private long _generation;

    public ObservableCollection<Tenant> Tenants { get; } = new();

    [ObservableProperty] private Tenant? _current;
    [ObservableProperty] private TenantScopeState _state = TenantScopeState.Unresolved;
    [ObservableProperty] private string _faultReason = "";

    public event EventHandler<TenantChangedEventArgs>? TenantChanged;

    /// <summary>requireRole("admin") y callerIsAdmin leen el mismo campo global.</summary>
    public bool IsGlobalAdmin =>
        string.Equals(_auth.CurrentOperator?.Role, "admin", StringComparison.OrdinalIgnoreCase);

    /// <summary>
    /// Solo un admin puede pedir ?tenant_id= y que el servidor lo aplique. Para
    /// un operador el selector es de solo lectura: cambiarlo no cambiarÃ­a nada
    /// en las respuestas, y ofrecerlo serÃ­a un selector cosmÃ©tico.
    /// </summary>
    public bool CanSwitchTenant => IsGlobalAdmin;

    /// <summary>Claim "tid" del JWT activo: el tenant contra el que el servidor
    /// resuelve requireSessionAccess, allow_impact_modules y las colecciones del token.</summary>
    public long TokenTenantId
    {
        get
        {
            long tid = _auth.CurrentOperator?.TenantId ?? 0;
            return tid > 0 ? tid : 1;
        }
    }

    public bool CanOperate => State == TenantScopeState.Resolved && Current is not null;

    public string ScopeLabel => Current is null ? "SIN TENANT" : $"#{Current.Id} {Current.Name}";

    public bool SelectedAllowsImpact => Current?.AllowImpactModules ?? false;

    /// <summary>
    /// allow_impact_modules del tenant del token. Es lo que el servidor consulta
    /// al pedir el listado de sesiones, con independencia del tenant seleccionado.
    /// </summary>
    public bool TokenTenantAllowsImpact =>
        Tenants.FirstOrDefault(t => t.Id == TokenTenantId)?.AllowImpactModules ?? false;

    /// <summary>
    /// El servidor refleja la deuda de operators.role global: un admin es admin
    /// para todos los tenants. La GUI lo dice en voz alta en vez de simular un
    /// aislamiento que el backend no hace.
    /// </summary>
    public string AccessNotice
    {
        get
        {
            if (State == TenantScopeState.Faulted) return "OperaciÃ³n bloqueada: el tenant no se pudo resolver.";
            if (!CanOperate) return "OperaciÃ³n bloqueada hasta resolver el tenant.";
            if (!IsGlobalAdmin)
                return $"Operador acotado al tenant #{TokenTenantId} del token. Cambiar de sesiÃ³n no amplÃ­a alcance.";
            return IsCrossTenantViewing
                ? $"Rol admin GLOBAL (operators.role no estÃ¡ particionado): puedes ver y operar en los {Tenants.Count} tenants."
                : $"Rol admin global. EstÃ¡s en #{Current!.Id}; los listados se filtran por ?tenant_id (server lo honra solo a admin).";
        }
    }

    public bool IsCrossTenantViewing => IsGlobalAdmin && Current is not null && Current.Id != TokenTenantId;

    public TenantService(ApiService api, AuthService auth)
    {
        _api = api;
        _auth = auth;
    }

    public TenantScopeToken BeginScope() =>
        new(this, Interlocked.Read(ref _generation), Current?.Id ?? TokenTenantId);

    internal bool IsGeneration(long generation) => Interlocked.Read(ref _generation) == generation;

    /// <summary>
    /// Fail-closed: si no se puede saber quÃ© tenants existen, la GUI queda en
    /// Faulted y CanOperate == false. No hay camino de "ya veremos".
    /// </summary>
    public async Task<bool> LoadAllAsync()
    {
        await _gate.WaitAsync().ConfigureAwait(false);
        try
        {
            Interlocked.Increment(ref _generation);
            var previous = Current;
            Dispatch(() =>
            {
                Current = null;
                _api.ActiveTenantId = null;
                State = TenantScopeState.Unresolved;
                FaultReason = "Cargando tenantsâ€¦";
                RaiseNotice();
            });

            var res = await _api.GetResultAsync<TenantEnvelope>("/api/tenants").ConfigureAwait(false);

            if (!res.Ok)
            {
                Fault(previous, $"GET /api/tenants devolviÃ³ {res.Error?.StatusCode}: {res.ErrorMessage}. No se puede acotar el alcance, asÃ­ que no se opera.");
                return false;
            }

            var list = res.Value?.Tenants;
            if (list is null || list.Count == 0)
            {
                Fault(previous, "El servidor no devolviÃ³ ningÃºn tenant ({\"tenants\": null}). Sin tenant conocido no se opera.");
                return false;
            }

            // Llenar Tenants de forma SÃNCRONA (Invoke, no BeginInvoke) para que
            // ApplyAsync pueda buscar el tenant inmediatamente despuÃ©s. Usar
            // BeginInvoke aquÃ­ causarÃ­a una condiciÃ³n de carrera: ApplyAsync
            // buscarÃ­a en Tenants antes de que BeginInvoke termine de llenarlo.
            DispatchSync(() =>
            {
                Tenants.Clear();
                foreach (var t in list) Tenants.Add(t);
                FaultReason = "";
                RaiseNotice();
            });

            long target = IsGlobalAdmin ? list[0].Id : TokenTenantId;
            if (list.All(t => t.Id != target))
            {
                Fault(previous, $"El tenant #{target} de tu token no aparece en GET /api/tenants. Token y servidor no coinciden; no se opera.");
                return false;
            }

            return await ApplyAsync(target, previous, "carga inicial").ConfigureAwait(false);
        }
        finally
        {
            _gate.Release();
        }
    }

    /// <summary>Cambio de tenant explÃ­cito. Serializado: una respuesta tardÃ­a de
    /// una selecciÃ³n anterior no puede pintar sobre la nueva.</summary>
    public async Task<bool> SelectAsync(long tenantId)
    {
        if (tenantId <= 0) return false;

        await _gate.WaitAsync().ConfigureAwait(false);
        try
        {
            var previous = Current;

            if (Current is not null && Current.Id == tenantId && State == TenantScopeState.Resolved)
                return true;

            if (!CanSwitchTenant && TokenTenantId != tenantId)
            {
                Fault(previous, $"Tu token estÃ¡ acotado al tenant #{TokenTenantId}; el servidor ignora ?tenant_id= para no-admin y responde 404 en los endpoints con resolveTenantScope.");
                return false;
            }

            if (Tenants.All(t => t.Id != tenantId))
            {
                Fault(previous, $"El tenant #{tenantId} no estÃ¡ en la lista del servidor. No se cambia de alcance.");
                return false;
            }

            return await ApplyAsync(tenantId, previous, "selecciÃ³n").ConfigureAwait(false);
        }
        finally
        {
            _gate.Release();
        }
    }

    private async Task<bool> ApplyAsync(long tenantId, Tenant? previous, string reason)
    {
        Interlocked.Increment(ref _generation);

        var target = Tenants.FirstOrDefault(t => t.Id == tenantId);
        if (target is null)
        {
            Fault(previous, $"Tenant #{tenantId} desaparecido de la lista.");
            return false;
        }

        Dispatch(() =>
        {
            Current = target;
            _api.ActiveTenantId = tenantId;
            State = TenantScopeState.Resolved;
            FaultReason = "";
            RaiseNotice();
        });

        // Se verifica contra el servidor, no contra memoria: GET /sessions de
        // este tenant tiene que traer filas marcadas con este tenant_id.
        // El probe es informativo: si falla no se faulta el tenant, porque
        // el servidor puede estar ocupado o la sesiÃ³n aÃºn no existe.
        var probe = await _api.GetResultAsync<SessionsProbe>(_api.ScopedSessionsPath()).ConfigureAwait(false);
        if (!probe.Ok)
        {
            // Probe fallÃ³: no se faulta el tenant, las sesiones se cargarÃ¡n en el prÃ³ximo refresh.
        }
        else
        {
            var rows = probe.Value?.Sessions;
            if (rows is not null)
            {
                var foreign = rows.Where(s => s.TenantId != 0 && s.TenantId != tenantId).ToList();
                if (foreign.Count > 0)
                {
                    Dispatch(() => LastScopeWarning =
                        $"{foreign.Count} de {rows.Count} filas de /sessions?tenant_id={tenantId} vienen de otros tenants " +
                        $"({string.Join(", ", foreign.Take(3).Select(s => $"#{s.TenantId}:{s.Id}"))}). El servidor no acotÃ³ el listado.");
                }
            }
        }

        Raise(previous, target, reason);
        return true;
    }

    [ObservableProperty] private string _lastScopeWarning = "";

    public void ClearScopeWarning() => LastScopeWarning = "";

    private void Fault(Tenant? previous, string reason)
    {
        Interlocked.Increment(ref _generation);
        Dispatch(() =>
        {
            Current = null;
            _api.ActiveTenantId = null;
            State = TenantScopeState.Faulted;
            FaultReason = reason;
            LastScopeWarning = "";
            RaiseNotice();
        });
        Raise(previous, null, reason);
    }

    private void Raise(Tenant? previous, Tenant? current, string reason)
    {
        var handler = TenantChanged;
        if (handler is null) return;
        Dispatch(() => handler(this, new TenantChangedEventArgs(previous, current, State, reason)));
    }

    private void RaiseNotice()
    {
        OnPropertyChanged(nameof(ScopeLabel));
        OnPropertyChanged(nameof(CanOperate));
        OnPropertyChanged(nameof(CanSwitchTenant));
        OnPropertyChanged(nameof(SelectedAllowsImpact));
        OnPropertyChanged(nameof(TokenTenantAllowsImpact));
        OnPropertyChanged(nameof(TokenTenantId));
        OnPropertyChanged(nameof(IsGlobalAdmin));
        OnPropertyChanged(nameof(IsCrossTenantViewing));
        OnPropertyChanged(nameof(AccessNotice));
    }

    private void Dispatch(Action action)
    {
        var dispatcher = App.Current?.Dispatcher;
        if (dispatcher is null || dispatcher.CheckAccess())
            action();
        else
            dispatcher.BeginInvoke(action);
    }

    /// <summary>SÃ­ncrono: usa Invoke para que el caller espere a que termine.
    /// Usar solo en operaciones donde no hay riesgo de reentrancia de colecciÃ³n
    /// (p. ej. llenado inicial de Tenants en LoadAllAsync).</summary>
    private void DispatchSync(Action action)
    {
        var dispatcher = App.Current?.Dispatcher;
        if (dispatcher is null || dispatcher.CheckAccess())
            action();
        else
            dispatcher.Invoke(action);
    }
}

/// <summary>Sonda mÃ­nima para confirmar que /sessions respetÃ³ el filtro.</summary>
public sealed class SessionsProbe
{
    [JsonProperty("sessions")] public List<Session>? Sessions { get; set; }
}
