using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using C2Dect.Models;
using C2Dect.Services;
using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Data;

namespace C2Dect.ViewModels;

/// <summary>Token de confirmacion que el servidor exige para las ordenes de impacto.</summary>
public static class ImpactConsent
{
    public const string Token = "AGENT-EXEC";
}

/// <summary>
/// Consola del operador de la ediciÃƒÂ³n Lite.
///
/// Es un recorte fiel de la consola completa: conserva la estructura (sesiones,
/// consola, rail lateral, workspaces, selector de tenant) y los paneles que la
/// ediciÃƒÂ³n Lite sÃƒÂ­ implementa. Los mÃƒÂ³dulos que la ediciÃƒÂ³n Lite no incluye
/// siguen estando en el menu, pero su panel explica que no estan disponibles:
/// no hay implementacion, configuracion ni payload que acompane a la entrada.
/// </summary>
public partial class MainViewModel : ObservableObject
{
    private readonly ApiService _api;
    private readonly AuthService _auth;
    private readonly TenantService _tenants;

    // --- estado general ---
    [ObservableProperty] private object? _currentView;
    [ObservableProperty] private Session? _selectedSession;
    [ObservableProperty] private string _statusText = "Sin conexion";
    [ObservableProperty] private string _currentOperator = "";
    [ObservableProperty] private int _activeSessions;
    [ObservableProperty] private int _totalSessions;
    [ObservableProperty] private int _operatorCount;
    [ObservableProperty] private int _tenantCount;
    [ObservableProperty] private string _sessionSearchText = "";
    [ObservableProperty] private bool _commandConsentGiven;
    [ObservableProperty] private string _workspaceName = "";
    [ObservableProperty] private string _workspaceStatus = "";

    // --- selector de tenant ---
    [ObservableProperty] private Tenant? _selectedTenantItem;

    public ObservableCollection<Session> Sessions { get; } = new();
    public ObservableCollection<C2Task> Tasks { get; } = new();
    public ObservableCollection<ConsoleMessage> ConsoleOutput { get; } = new();
    public ObservableCollection<string> AvailableWorkspaces { get; } = new();
    public ObservableCollection<Session> WorkspaceSessions { get; } = new();

    public ICollectionView SessionsView { get; private set; } = null!;

    // --- paneles ---
    /// <summary>
    /// Ventana propietaria de los modulos que se abren aparte (Lethal Script y
    /// su documentacion). La fija la MainWindow al construirse: sin esto, el
    /// Owner caeria en Application.MainWindow, que con varias ventanas
    /// abiertas no es necesariamente la consola.
    /// </summary>
    public Window? OwnerWindow { get; set; }

    public LootViewModel LootVm { get; }    public ProfilesViewModel ProfilesVm { get; }
    public ScriptMarketplaceViewModel ScriptMarketVm { get; }
    public AdminViewModel AdminVm { get; }
    public AuditViewModel AuditVm { get; }
    public ModuleInactiveViewModel InactiveVm { get; }

    public bool CanSwitchTenant => _tenants.CanSwitchTenant;
    public string TenantScopeLabel => _tenants.ScopeLabel;
    public string TenantAccessNotice => _tenants.AccessNotice;
    public string TenantFaultReason => _tenants.LastScopeWarning;
    public bool TenantScopeFaulted => _tenants.State == TenantScopeState.Faulted;

    /// <summary>Modulos declarados en la consola que la edicion Lite no incluye.</summary>
    private static readonly Dictionary<string, string> InactiveModules = new()
    {
        ["listeners"] = "Listeners",
        ["payloads"] = "Payloads",
        ["builder"] = "Payload Builder",
        ["implant-config"] = "Implant Config",
        ["dashboard"] = "Vulnerabilities",
        ["vuln-scan"] = "Vuln Scan (PC)",
        ["hunting-nvd"] = "Hunting NVD",
        ["recon-go"] = "Recon (Go)",
        ["segment-map"] = "Segment Map",
        ["ai-copilot"] = "IA Copilot",
        ["remote-desktop"] = "Remote Desktop",
        ["file-manager"] = "File Manager",
        ["executive"] = "Executive Summary",
        ["recon-vulns"] = "Recon + Vulns",
        ["operations"] = "Operations",
        ["tooling"] = "Recon & Fuzzing",
        ["bounty"] = "Bounty & Research",
    };

    public MainViewModel(ApiService api, AuthService auth, TenantService tenants)
    {
        _api = api;
        _auth = auth;
        _tenants = tenants;

        LootVm = new LootViewModel(api, tenants);
        ProfilesVm = new ProfilesViewModel(api);
        ScriptMarketVm = new ScriptMarketplaceViewModel(api);
        AdminVm = new AdminViewModel(api, tenants);
        AuditVm = new AuditViewModel(api);
        InactiveVm = new ModuleInactiveViewModel();

        SessionsView = CollectionViewSource.GetDefaultView(Sessions);
        SessionsView.Filter = FilterSession;

        CurrentOperator = auth.CurrentOperator?.Username ?? "desconocido";
        SelectedTenantItem = _tenants.Current;

        ShowWelcome();
    }

    private bool FilterSession(object item)
    {
        if (item is not Session s) return false;
        var needle = (SessionSearchText ?? "").Trim();
        if (needle.Length == 0) return true;
        return s.Id.Contains(needle, StringComparison.OrdinalIgnoreCase)
            || (s.Hostname ?? "").Contains(needle, StringComparison.OrdinalIgnoreCase)
            || (s.Username ?? "").Contains(needle, StringComparison.OrdinalIgnoreCase)
            || (s.InternalIp ?? "").Contains(needle, StringComparison.OrdinalIgnoreCase);
    }

    partial void OnSessionSearchTextChanged(string value) => SessionsView?.Refresh();

    // ------------------------------------------------------------- consola

    public void LogInfo(string msg) => SafeAdd(new ConsoleMessage { Direction = "info", Content = $"[*] {msg}" });
    public void LogSent(string msg) => SafeAdd(new ConsoleMessage { Direction = "sent", Content = $"> {msg}" });
    public void LogReceived(string msg) => SafeAdd(new ConsoleMessage { Direction = "received", Content = msg });
    public void LogWarn(string msg) => SafeAdd(new ConsoleMessage { Direction = "warn", Content = $"[!] {msg}" });
    public void LogError(string msg) => SafeAdd(new ConsoleMessage { Direction = "error", Content = $"[x] {msg}" });

    private void SafeAdd(ConsoleMessage msg)
    {
        if (App.Current?.Dispatcher == null || App.Current.Dispatcher.CheckAccess())
            ConsoleOutput.Add(msg);
        else
            App.Current.Dispatcher.Invoke(() => ConsoleOutput.Add(msg));
    }

    private void Dispatch(Action action)
    {
        if (App.Current?.Dispatcher == null || App.Current.Dispatcher.CheckAccess())
            action();
        else
            App.Current.Dispatcher.Invoke(action);
    }

    private void ShowWelcome()
    {
        LogInfo("=== C2-DECT LITE 2.0 (open source) ===");
        LogInfo($"Operador : {CurrentOperator} ({_auth.CurrentOperator?.Role ?? "?"})");
        LogInfo($"Tenant   : {TenantScopeLabel}");
        LogInfo($"Servidor : {_api.BaseUrl}");
        LogInfo("");
        LogInfo("La consola muestra la estructura completa de la suite. Los modulos que");
        LogInfo("no forman parte de la edicion Lite aparecen como inactivos: su panel");
        LogInfo("explica la situacion y no ejecuta nada. El marketplace es funcional.");
        LogInfo("");
        LogInfo("Escribe 'help' para la lista de comandos del agente.");
        LogInfo("=========================================");
    }

    // ----------------------------------------------------------- navegacion

    /// <summary>Abre un panel de modulo que la edicion Lite no incluye.</summary>
    [RelayCommand]
    private void NavigateToModule(string? key)
    {
        if (string.IsNullOrWhiteSpace(key)) return;
        var title = InactiveModules.TryGetValue(key, out var name) ? name : key;
        InactiveVm.Show(key, title);
        CurrentView = InactiveVm;
        LogInfo($"El modulo '{title}' no forma parte de la edicion Lite (open source).");
    }

    [RelayCommand]
    private async Task NavigateToSessions()
    {
        CurrentView = null;
        await LoadSessions();
    }

    [RelayCommand]
    private async Task NavigateToLoot()
    {
        CurrentView = LootVm;
        await LootVm.LoadLoot();
    }

    [RelayCommand]
    private async Task NavigateToProfiles()
    {
        CurrentView = ProfilesVm;
        await ProfilesVm.LoadProfiles();
    }

    [RelayCommand]
    private async Task NavigateToScriptMarketplace()
    {
        CurrentView = ScriptMarketVm;
        await ScriptMarketVm.RefreshCommand.ExecuteAsync(null);
    }

    /// <summary>Abre el editor de guiones (Lethal Script Lite).</summary>
    [RelayCommand]
    private void NavigateToLethalScript()
    {
        try
        {
            var window = new Views.LethalScriptWindow();
            // el Owner se asigna DESPUES de construir: al crearla, WPF puede
            // convertirla en Application.MainWindow y asignarse a si misma
            try
            {
                var owner = OwnerWindow ?? Application.Current?.MainWindow;
                if (owner != null && owner != window) window.Owner = owner;
            }
            catch { }
            window.Show();
        }
        catch (Exception ex)
        {
            Logger.LogError("NavigateToLethalScript", ex);
            LogError($"No se pudo abrir Lethal Script: {ex.Message}");
        }
    }

    /// <summary>Abre la documentacion del modulo de guiones.</summary>
    [RelayCommand]
    private void NavigateToLethalScriptDocs()
    {
        try
        {
            var window = new Views.LethalScriptDocsWindow();
            try
            {
                var owner = OwnerWindow ?? Application.Current?.MainWindow;
                if (owner != null && owner != window) window.Owner = owner;
            }
            catch { }
            window.Show();
        }
        catch (Exception ex)
        {
            Logger.LogError("NavigateToLethalScriptDocs", ex);
            LogError($"No se pudo abrir la documentacion: {ex.Message}");
        }
    }

    [RelayCommand]
    private async Task NavigateToAdmin()
    {
        CurrentView = AdminVm;
        await AdminVm.LoadTenants();
    }

    [RelayCommand]
    private async Task NavigateToAudit()
    {
        CurrentView = AuditVm;
        await AuditVm.LoadAudit();
    }

    // ------------------------------------------------------------- sesiones

    [RelayCommand]
    private async Task LoadSessions()
    {
        try
        {
            var sessions = await _api.GetSessions();
            Dispatch(() =>
            {
                Sessions.Clear();
                if (sessions != null)
                    foreach (var s in sessions)
                        Sessions.Add(s);
                ActiveSessions = Sessions.Count(s => s.Status == "active");
                TotalSessions = Sessions.Count;
                StatusText = $"{Sessions.Count} sesion(es)";
            });
        }
        catch (Exception ex)
        {
            LogError($"No se pudieron cargar las sesiones: {ex.Message}");
        }
    }

    [RelayCommand]
    private async Task LoadTasks()
    {
        if (SelectedSession == null) return;
        try
        {
            var tasks = await _api.GetTasks(SelectedSession.Id);
            Dispatch(() =>
            {
                Tasks.Clear();
                if (tasks == null) return;
                foreach (var t in tasks) Tasks.Add(t);
                foreach (var t in tasks.Where(t => t.Status == "completed" && !string.IsNullOrEmpty(t.Result)))
                    LogReceived($"[{t.Command}] {Trim(t.Result, 400)}");
            });
        }
        catch (Exception ex)
        {
            LogError($"No se pudieron cargar las tareas: {ex.Message}");
        }
    }

    private static string Trim(string value, int max)
        => value.Length <= max ? value : value[..max] + "...";

    // ------------------------------------------------------------- tasking

    [RelayCommand]
    private async Task SendCommand(string? command)
    {
        if (string.IsNullOrWhiteSpace(command)) return;
        var text = command.Trim();

        switch (text.ToLowerInvariant())
        {
            case "help":
                ShowHelp();
                return;
            case "clear":
                ConsoleOutput.Clear();
                ShowWelcome();
                return;
            case "refresh":
                await RefreshAll();
                return;
            case "cleanup":
                await _api.CleanupStaleSessions(30);
                await LoadSessions();
                return;
            case "deleteall":
                await _api.DeleteAllSessions();
                Dispatch(() => { Sessions.Clear(); SelectedSession = null; Tasks.Clear(); });
                return;
        }

        if (SelectedSession == null)
        {
            LogError("Selecciona primero una sesion.");
            return;
        }
        if (SelectedSession.Status != "active")
        {
            LogWarn($"La sesion {SelectedSession.DisplayName} no esta activa (estado: {SelectedSession.Status}).");
            return;
        }

        LogSent(text);
        try
        {
            // Las ordenes de nivel de impacto (shell, kill, rm, mv) solo se
            // encolan si el operador ha marcado el consentimiento explicito.
            var confirm = CommandConsentGiven ? ImpactConsent.Token : "";
            var task = await _api.SubmitTask(SelectedSession.Id, text, "", confirm);
            if (task == null)
            {
                LogError($"El servidor no acepto la orden. {_api.LastError}");
                return;
            }
            Dispatch(() =>
            {
                Tasks.Insert(0, task);
                StatusText = $"Tarea {task.Id} enviada a {SelectedSession!.DisplayName}";
                LogInfo($"Tarea {task.Id} en cola, esperando resultado...");
            });
        }
        catch (Exception ex)
        {
            LogError($"No se pudo enviar la orden: {ex.Message}");
        }
    }

    private void ShowHelp()
    {
        LogInfo("=== COMANDOS DEL AGENTE ===");
        LogInfo("LECTURA  : sysinfo, netinfo, ps, services, env, connections");
        LogInfo("           hostname, whoami, uptime, ls, cat, stat, hashfile, find, exit");
        LogInfo("ESCRITURA: mkdir, upload, download, cp, sleep");
        LogInfo("IMPACTO  : shell, kill, rm, mv");
        LogInfo("           (necesitan la casilla de consentimiento y un agente");
        LogInfo("            arrancado con -allow-impact)");
        LogInfo("");
        LogInfo("CONSOLA  : refresh, clear, cleanup, deleteall, help");
        LogInfo("=========================================");
    }

    // ------------------------------------------------------------- acciones

    [RelayCommand]
    private async Task RefreshAll()
    {
        await LoadSessions();
        if (SelectedSession != null)
            await LoadTasks();
        await LoadTenants();
        StatusText = $"Actualizado {DateTime.Now:HH:mm:ss}";
    }

    [RelayCommand]
    private void ClearConsole()
    {
        ConsoleOutput.Clear();
        ShowWelcome();
    }

    /// <summary>Arranca el agente Lite local si el binario esta disponible.</summary>
    [RelayCommand]
    private void LaunchAgent()
    {
        try
        {
            var exe = FindAgentExecutable();
            if (exe == null)
            {
                LogError("No se encontro build/agent.exe. Compila el agente con build.bat.");
                return;
            }
            var host = _api.BaseUrl ?? "http://127.0.0.1:8443";
            var uri = new Uri(host);
            var args = $"-s {uri.Host} -p {uri.Port}";
            var psi = new ProcessStartInfo(exe, args) { UseShellExecute = false, CreateNoWindow = true };
            var proc = Process.Start(psi);
            LogInfo(proc != null
                ? $"Agente lanzado (PID {proc.Id}) apuntando a {host}."
                : "No se pudo lanzar el agente.");
        }
        catch (Exception ex)
        {
            LogError($"No se pudo lanzar el agente: {ex.Message}");
        }
    }

    private static string? FindAgentExecutable()
    {
        var root = AppDomain.CurrentDomain.BaseDirectory;
        for (int i = 0; i < 6 && root != null; i++)
        {
            var candidate = Path.Combine(root, "build", "agent.exe");
            if (File.Exists(candidate)) return candidate;
            var parent = Directory.GetParent(root);
            root = parent?.FullName;
        }
        return null;
    }

    // ------------------------------------------------------------ workspaces

    [RelayCommand]
    private async Task ApplyWorkspace()
    {
        var ws = (WorkspaceName ?? "").Trim();
        if (SelectedSession == null)
        {
            WorkspaceStatus = "Selecciona una sesion en la tabla.";
            return;
        }
        if (!IsValidWorkspace(ws))
        {
            WorkspaceStatus = "Workspace invalido: [a-z0-9-]{1,32}.";
            return;
        }
        var ok = await _api.SetWorkspace(SelectedSession.Id, ws);
        WorkspaceStatus = ok
            ? $"Sesion {SelectedSession.DisplayName} asignada al workspace '{ws}'."
            : $"No se pudo asignar el workspace. {_api.LastError}";
        if (ok) await LoadSessions();
    }

    [RelayCommand]
    private async Task LoadWorkspaceSessions()
    {
        var ws = (WorkspaceName ?? "").Trim();
        if (ws.Length == 0)
        {
            WorkspaceStatus = "Escribe el nombre del workspace.";
            return;
        }
        var list = await _api.GetSessionsByWorkspace(ws);
        Dispatch(() =>
        {
            WorkspaceSessions.Clear();
            if (list != null) foreach (var s in list) WorkspaceSessions.Add(s);
            WorkspaceStatus = $"Workspace '{ws}': {WorkspaceSessions.Count} sesion(es)";
        });
    }

    private static bool IsValidWorkspace(string ws)
        => ws.Length is >= 1 and <= 32 && ws.All(c => char.IsLower(c) || char.IsDigit(c) || c == '-');

    partial void OnWorkspaceNameChanged(string value)
    {
        if (string.IsNullOrWhiteSpace(value)) return;
        if (!AvailableWorkspaces.Contains(value))
        {
            AvailableWorkspaces.Add(value);
            OnPropertyChanged(nameof(AvailableWorkspaces));
        }
    }

    // --------------------------------------------------------------- tenants

    [RelayCommand]
    private async Task LoadTenants()
    {
        await _tenants.LoadAllAsync();
        Dispatch(() =>
        {
            TenantCount = _tenants.Tenants.Count;
            SelectedTenantItem = _tenants.Current;
            OnPropertyChanged(nameof(TenantScopeLabel));
            OnPropertyChanged(nameof(TenantAccessNotice));
            OnPropertyChanged(nameof(CanSwitchTenant));
        });
        await LoadOperatorCount();
    }

    private async Task LoadOperatorCount()
    {
        try
        {
            var users = await _api.GetUsersByTenant(_tenants.TokenTenantId);
            Dispatch(() => OperatorCount = users?.Count ?? 0);
        }
        catch
        {
            Dispatch(() => OperatorCount = 0);
        }
    }

    partial void OnSelectedTenantItemChanged(Tenant? value)
    {
        if (value == null) return;
        _ = SelectTenantAsync(value.Id);
    }

    private async Task SelectTenantAsync(long tenantId)
    {
        if (!_tenants.CanSwitchTenant) return;
        if (await _tenants.SelectAsync(tenantId))
        {
            Dispatch(() =>
            {
                SelectedTenantItem = _tenants.Current;
                OnPropertyChanged(nameof(TenantScopeLabel));
                OnPropertyChanged(nameof(TenantAccessNotice));
            });
            await LoadSessions();
        }
    }

    partial void OnSelectedSessionChanged(Session? value)
    {
        if (value == null) return;
        LogInfo($"Sesion seleccionada {value.DisplayName} ({value.OS} {value.Arch})");
        _ = LoadTasks();
    }
}

/// <summary>
/// Panel de un modulo que la edicion Lite no incluye. El panel explica la
/// situacion y no ejecuta nada.
/// </summary>
public partial class ModuleInactiveViewModel : ObservableObject
{
    [ObservableProperty] private string _moduleKey = "";
    [ObservableProperty] private string _moduleTitle = "";
    [ObservableProperty] private string _notice =
        "Este modulo no forma parte de la edicion Lite (open source). La consola conserva "
        + "la entrada para que el operador vea la estructura completa de la suite, pero no se "
        + "implementa, no se configura y no se ejecuta nada desde aqui.";

    public void Show(string key, string title)
    {
        ModuleKey = key;
        ModuleTitle = title;
    }
}


