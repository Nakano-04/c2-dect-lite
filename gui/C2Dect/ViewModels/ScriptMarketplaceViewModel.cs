using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using C2Dect.Models;
using C2Dect.Services;
using System;
using System.Collections.ObjectModel;
using System.Threading.Tasks;

namespace C2Dect.ViewModels;

/// <summary>
/// Script Market: marketplace de scripts Lua/WASM del C2.
///
/// Endpoints que consume (todos YA existen en ApiService; no se anade ninguna
/// llamada HTTP nueva):
///   GET  /api/scripts/list      -> {"scripts":[nombres], "count":n}
///   GET  /api/scripts/get?name= -> {name, source, scope, sig?, kid?}
///   GET  /api/scripts/pubkey    -> {pubkey, kid} (404 en modo lab sin firma)
///   POST /api/scripts/upload    -> {name, status, script_id, scope, sig?, kid?}
///   GET  /api/sessions          -> contexto de agentes activos del lab.
///
/// DETALLE CRITICO DEL SERVIDOR (server/api/scripts_marketplace.go y
/// server/api/script_trust.go; leer el Go antes de tocar esta clase):
///   - El scope NO viaja en el JSON body: el servidor lo deduce de la
///     cabecera "-- scope:" de las primeras 5 lineas del source
///     (declaredScope, scripts_marketplace.go:86) y responde 400 si no
///     existe. Si el operador escribe un scope y el source no lo declara,
///     la GUI lo antepone (ApiService.ScriptUploadAsync).
///   - Los scripts SIN firma solo se aceptan si declaran alcance "lab-only"
///     (o si el tenant tiene claves de confianza): requireLabOnlyScope
///     (script_trust.go:231).
///   - /api/scripts/pubkey responde 404 en lab sin SCRIPT_SIGN_KEY: eso es
///     "firma no disponible", NO un error fatal.
///   - Cuando una subida falla se muestra el error EXACTO del servidor
///     (ApiService.LastError), sin reescribirlo ni resumirlo.
/// </summary>
public partial class ScriptMarketplaceViewModel : ObservableObject
{
    /// <summary>Servicio API compartido (lo usan las llamadas del marketplace).</summary>
    public ApiService Api { get; }

    /// <summary>Scripts publicados en el marketplace (GET /api/scripts/list).</summary>
    public ObservableCollection<ScriptEntry> Scripts { get; } = new();

    /// <summary>Nombre del script seleccionado en la lista.</summary>
    [ObservableProperty] private string _selectedScriptName = "";

    /// <summary>Source completo del script seleccionado (GET /api/scripts/get).</summary>
    [ObservableProperty] private string _scriptSource = "";

    /// <summary>Nombre con el que se publica un script nuevo.</summary>
    [ObservableProperty] private string _uploadName = "";

    /// <summary>Source del script que se va a publicar.</summary>
    [ObservableProperty] private string _uploadSource = "";

    /// <summary>
    /// Alcance declarado del script a publicar. OJO: no viaja en el JSON; se
    /// materializa como cabecera "-- scope:" dentro del source.
    /// </summary>
    [ObservableProperty] private string _uploadScope = "lab-only";

    /// <summary>Pubkey Ed25519 (hex) con la que firma el servidor (si firma).</summary>
    [ObservableProperty] private string _pubkey = "";

    /// <summary>True si el servidor expone pubkey de firma (false en modo lab).</summary>
    [ObservableProperty] private bool _pubkeyAvailable = false;

    /// <summary>Ultima accion ejecutada y su resultado (visible en la cabecera).</summary>
    [ObservableProperty] private string _statusText = "Ready";

    /// <summary>Ultimo error del servidor al publicar (texto exacto, sin retocar).</summary>
    [ObservableProperty] private string _uploadError = "";

    /// <summary>True si hay un error de subida que mostrar.</summary>
    [ObservableProperty] private bool _hasUploadError = false;

    /// <summary>Explicacion de que va a pasar con el alcance del source.</summary>
    [ObservableProperty] private string _scopeNote = "";

    /// <summary>Estado de la firma (pubkey) del servidor.</summary>
    [ObservableProperty] private string _pubkeyStatus = "Firma: sin consultar (pulsa CARGAR PUBKEY).";

    /// <summary>Numero de scripts publicados (GET /api/scripts/list).</summary>
    [ObservableProperty] private int _scriptCount = 0;

    /// <summary>Agentes activos del lab (GET /api/sessions?status=active).</summary>
    [ObservableProperty] private int _activeSessionCount = 0;

    // =====================================================================
    // Estado de CONFIANZA del script (firmado / sin firma / fuera de scope /
    // tenant incorrecto).
    //
    // POR QUE se calcula en la GUI y no en el servidor: el Go ya tiene la
    // politica (server/api/script_trust.go) pero NO expone un endpoint que
    // de el veredicto por script. La GUI lo deduce de lo que el servidor SI
    // expone (GET /api/scripts/get y GET /api/scripts/pubkey) aplicando la
    // misma politica. Lo que no puede hacer (y por eso lo dice) es verificar
    // la firma criptograficamente.
    // =====================================================================

    /// <summary>Veredicto de confianza: FIRMADO | SIN_FIRMA | FUERA_DE_SCOPE | TENANT_INCORRECTO.</summary>
    [ObservableProperty] private string _trustVerdict = "sin consultar";

    /// <summary>Motivo del veredicto (texto del calculo, sin reinterpretar).</summary>
    [ObservableProperty] private string _trustReason = "Pulsa VER CONFIANZA para consultar el estado de confianza del script seleccionado.";

    /// <summary>Alcance declarado por el autor del script.</summary>
    [ObservableProperty] private string _trustScope = "";

    /// <summary>kid de la clave con la que se firmo (vacio si no va firmado).</summary>
    [ObservableProperty] private string _trustKid = "";

    /// <summary>True si el servidor adjunto firma al script.</summary>
    [ObservableProperty] private bool _trustSigned = false;

    public ScriptMarketplaceViewModel(ApiService api)
    {
        Api = api;
        UpdateScopeNote();
    }

    // =====================================================================
    // Comandos
    // =====================================================================

    /// <summary>Recarga la lista de scripts y el numero de agentes activos.</summary>
    [RelayCommand]
    private async Task Refresh()
    {
        try
        {
            StatusText = "Consultando el marketplace...";
            if (!Api.IsConfigured)
            {
                StatusText = "Servidor no configurado: faltan URL o token.";
                return;
            }

            // /api/sessions se consulta primero a proposito: usa Get<T>, que deja
            // el motivo real del fallo en Api.LastError (a diferencia de
            // /api/scripts/list, que solo devuelve una lista vacia). Asi el
            // operador ve POR QUE no hay scripts y no un "0 scripts" mudo.
            ActiveSessionCount = 0;
            var sessions = await Api.GetSessions("active");
            if (sessions == null)
            {
                var err = Api.LastError;
                if (!string.IsNullOrWhiteSpace(err))
                {
                    StatusText = $"Error contacting server: {err}";
                    Logger.Log($"ScriptMarketplace.Refresh: /api/sessions -> {err}");
                    return;
                }
                // El servidor respondio bien y no hay sesiones (lista vacia).
            }
            else
            {
                ActiveSessionCount = sessions.Count;
            }

            await LoadScriptsAsync();
            StatusText = $"{ScriptCount} script(s) publicados | {ActiveSessionCount} agente(s) activo(s).";
        }
        catch (Exception ex)
        {
            StatusText = $"Error: {ex.Message}";
            Logger.LogError("ScriptMarketplace.Refresh", ex);
        }
    }

    /// <summary>
    /// Descarga el source del script seleccionado (GET /api/scripts/get?name=).
    /// </summary>
    [RelayCommand]
    private async Task LoadScript()
    {
        var name = (SelectedScriptName ?? "").Trim();
        if (string.IsNullOrWhiteSpace(name))
        {
            StatusText = "Selecciona un script de la lista.";
            return;
        }

        try
        {
            StatusText = $"Descargando '{name}'...";
            var source = await Api.GetScriptAsync(name);
            if (source == null)
            {
                ScriptSource = "";
                StatusText = $"No se pudo obtener '{name}' (no existe en el servidor o la llamada fallo).";
                Logger.Log($"ScriptMarketplace.LoadScript: '{name}' no disponible");
                return;
            }

            ScriptSource = source;
            StatusText = $"'{name}' cargado ({source.Length} caracteres).";

            // El estado de confianza se consulta con el script: el operador
            // tiene que ver si lo que va a ejecutar esta firmado y por quien.
            await LoadTrustAsync(name);
        }
        catch (Exception ex)
        {
            ScriptSource = "";
            StatusText = $"Error: {ex.Message}";
            Logger.LogError("ScriptMarketplace.LoadScript", ex);
        }
    }

    /// <summary>
    /// Consulta el veredicto de confianza del script (firma + scope + tenant).
    /// El veredicto lo calcula ApiService con la misma politica que el
    /// servidor Go; la GUI lo enseña tal cual.
    /// </summary>
    [RelayCommand]
    private async Task LoadTrust()
    {
        var name = (SelectedScriptName ?? "").Trim();
        if (string.IsNullOrWhiteSpace(name))
        {
            TrustVerdict = "sin consultar";
            TrustReason = "Selecciona un script de la lista.";
            return;
        }
        await LoadTrustAsync(name);
    }

    /// <summary>Consulta y pinta el veredicto de confianza de un script.</summary>
    private async Task LoadTrustAsync(string name)
    {
        try
        {
            var trust = await Api.GetScriptTrustAsync(name);
            if (trust == null)
            {
                TrustVerdict = "DESCONOCIDO";
                TrustReason = $"El servidor no ha devuelto el detalle de '{name}' (no existe o la llamada fallo). {Api.LastError}";
                TrustScope = "";
                TrustKid = "";
                TrustSigned = false;
                return;
            }

            TrustVerdict = trust.Verdict ?? "DESCONOCIDO";
            TrustReason = trust.Reason ?? "";
            TrustScope = string.IsNullOrWhiteSpace(trust.Scope) ? "(sin alcance declarado)" : trust.Scope!;
            TrustKid = string.IsNullOrWhiteSpace(trust.Kid) ? "(sin firma)" : trust.Kid!;
            TrustSigned = trust.Signed;
            Logger.Log($"ScriptMarketplace.Trust '{name}': {TrustVerdict}");
        }
        catch (Exception ex)
        {
            TrustVerdict = "ERROR";
            TrustReason = ex.Message;
            Logger.LogError("ScriptMarketplace.LoadTrust", ex);
        }
    }

    /// <summary>
    /// Publica el script del formulario (POST /api/scripts/upload). El scope
    /// viaja dentro del source (cabecera "-- scope:"), no en el JSON.
    /// </summary>
    [RelayCommand]
    private async Task Upload()
    {
        var name = (UploadName ?? "").Trim();
        var source = UploadSource ?? "";
        var scope = (UploadScope ?? "").Trim();

        if (string.IsNullOrWhiteSpace(name))
        {
            ShowUploadError("Falta el nombre del script.");
            StatusText = "Subida cancelada: falta el nombre.";
            return;
        }
        if (string.IsNullOrWhiteSpace(source))
        {
            ShowUploadError("Falta el source del script.");
            StatusText = "Subida cancelada: falta el source.";
            return;
        }

        try
        {
            StatusText = $"Publicando '{name}'...";
            HasUploadError = false;
            UploadError = "";
            Api.ClearLastError();

            // El servicio antepone la cabecera "-- scope:" si el source no la
            // declara; si ya la declara, manda la del source (el servidor la
            // deduce del source, nunca del JSON).
            var ok = await Api.ScriptUploadAsync(name, source, string.IsNullOrWhiteSpace(scope) ? null : scope);
            if (!ok)
            {
                // LastError lleva el mensaje EXACTO del servidor Go, p.ej.
                // "400: scope declaration required (add '-- scope: lab-only' header)".
                var err = Api.LastError;
                ShowUploadError(string.IsNullOrWhiteSpace(err)
                    ? "El servidor rechazo la subida sin detalle."
                    : err);
                StatusText = $"Subida rechazada: {UploadError}";
                Logger.Log($"ScriptMarketplace.Upload rechazado: {UploadError}");
                return;
            }

            await LoadScriptsAsync();
            StatusText = $"'{name}' publicado ({ScriptCount} scripts en el marketplace).";
        }
        catch (Exception ex)
        {
            ShowUploadError(ex.Message);
            StatusText = $"Error: {ex.Message}";
            Logger.LogError("ScriptMarketplace.Upload", ex);
        }
    }

    /// <summary>
    /// Carga la pubkey de firma (GET /api/scripts/pubkey). Un 404 en modo lab
    /// (sin SCRIPT_SIGN_KEY) es "no disponible", no un fallo.
    /// </summary>
    [RelayCommand]
    private async Task LoadPubkey()
    {
        try
        {
            StatusText = "Consultando la pubkey de firma...";
            Pubkey = "";
            PubkeyAvailable = false;

            var key = await Api.GetScriptPubkeyAsync();
            if (string.IsNullOrWhiteSpace(key))
            {
                // 404 en lab: el servidor no firma. No es un error fatal.
                PubkeyStatus = "Firma no disponible: /api/scripts/pubkey devuelve 404 (modo lab sin SCRIPT_SIGN_KEY). Los scripts sin firma solo se aceptan con alcance 'lab-only'.";
                StatusText = "Firma no disponible (404): el servidor no expone pubkey.";
                return;
            }

            Pubkey = key ?? "";
            PubkeyAvailable = true;
            PubkeyStatus = "Firma disponible: aprovisiona los agentes con esta pubkey (C2_SCRIPT_PUBKEY).";
            StatusText = "Pubkey de firma cargada.";
        }
        catch (Exception ex)
        {
            Pubkey = "";
            PubkeyAvailable = false;
            PubkeyStatus = $"No se pudo consultar la pubkey: {ex.Message}";
            StatusText = $"Error: {ex.Message}";
            Logger.LogError("ScriptMarketplace.LoadPubkey", ex);
        }
    }

    /// <summary>Copia el source del script seleccionado al portapapeles.</summary>
    [RelayCommand]
    private void CopySource()
    {
        if (string.IsNullOrWhiteSpace(ScriptSource))
        {
            StatusText = "No hay source cargado para copiar.";
            return;
        }

        try
        {
            System.Windows.Clipboard.SetText(ScriptSource);
            StatusText = "Source copiado al portapapeles.";
        }
        catch (Exception ex)
        {
            StatusText = $"No se pudo copiar: {ex.Message}";
            Logger.LogError("ScriptMarketplace.CopySource", ex);
        }
    }

    // =====================================================================
    // Auxiliares
    // =====================================================================

    /// <summary>
    /// Rellena <see cref="Scripts"/> con el listado del servidor. No toca el
    /// estado para que quien la llama decide el mensaje final.
    /// </summary>
    private async Task LoadScriptsAsync()
    {
        var list = await Api.GetScriptListAsync();
        Scripts.Clear();
        foreach (var entry in list)
            Scripts.Add(entry);
        ScriptCount = Scripts.Count;
    }

    /// <summary>Muestra el error de subida (texto del servidor, sin retocar).</summary>
    private void ShowUploadError(string message)
    {
        UploadError = message;
        HasUploadError = true;
    }

    /// <summary>
    /// Explica que va a hacer el servidor con el alcance: si el source ya
    /// declara "-- scope:" manda esa, si no la GUI antepone la del formulario.
    /// </summary>
    private void UpdateScopeNote()
    {
        var declared = DeclaredScope(UploadSource);
        if (!string.IsNullOrEmpty(declared))
        {
            ScopeNote = $"El source ya declara '-- scope: {declared}': manda esa (el campo de alcance se ignora).";
            return;
        }
        if (string.IsNullOrWhiteSpace(UploadScope))
        {
            ScopeNote = "Sin alcance: el servidor rechazara la subida (400: 'scope declaration required').";
            return;
        }
        ScopeNote = $"Se antepondra la cabecera '-- scope: {UploadScope.Trim()}' al source antes de subirlo.";
    }

    /// <summary>
    /// Lee el alcance DECLARADO por el autor del source. Mismo criterio que
    /// declaredScope del servidor (5 primeras lineas, "-- scope:").
    /// </summary>
    private static string DeclaredScope(string? source)
    {
        if (string.IsNullOrEmpty(source)) return "";
        var lines = source.Split('\n');
        for (int i = 0; i < lines.Length && i < 5; i++)
        {
            var t = lines[i].Trim();
            if (t.Length > 9 && string.Equals(t.Substring(0, 9), "-- scope:", StringComparison.OrdinalIgnoreCase))
                return t.Substring(9).Trim();
        }
        return "";
    }

    partial void OnUploadSourceChanged(string value) => UpdateScopeNote();

    partial void OnUploadScopeChanged(string value) => UpdateScopeNote();
}