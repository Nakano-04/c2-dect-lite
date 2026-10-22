using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Input;
using C2Dect.ViewModels;

namespace C2Dect.Views;

/// <summary>
/// Code-behind de la consola Lite.
///
/// Es el mismo de la consola completa con los manejadores de los modulos
/// retirados: la edicion Lite no abre ventanas de escritorio remoto, gestor de
/// ficheros, motor de exploits ni campanas de phishing. El resto (historial
/// de la consola, completado por Tab, busqueda de comandos rapidos, copiado y
/// exportacion) se conserva igual.
/// </summary>
public partial class MainWindow : Window
{
    private MainViewModel? ViewModel => DataContext as MainViewModel;

    private readonly List<string> _cmdHistory = new();
    private int _historyIndex = -1;

    /// <summary>Comandos del agente Lite: los que el servidor acepta.</summary>
    private readonly string[] _allCommands =
    {
        "whoami", "hostname", "uptime", "ps", "sysinfo", "netinfo", "connections",
        "services", "env", "ls .", "stat .", "find . *.txt", "hashfile .",
        "sleep 5", "sleep 30", "exit",
    };

    public MainWindow()
    {
        try
        {
            Logger.Log("MainWindow constructing...");
            InitializeComponent();
            var vm = new MainViewModel(App.ApiService!, App.AuthService!, App.TenantService!) { OwnerWindow = this };
            DataContext = vm;
            Logger.Log("MainWindow DataContext set");

            if (ViewModel?.ConsoleOutput != null)
                ViewModel.ConsoleOutput.CollectionChanged += (s, e) =>
                {
                    if (ConsoleScroller != null)
                        ConsoleScroller.ScrollToEnd();
                };
            Logger.Log("MainWindow constructed OK");
        }
        catch (Exception ex)
        {
            Logger.LogError("MainWindow constructor", ex);
            throw;
        }
    }

    private async void Window_Loaded(object sender, RoutedEventArgs e)
    {
        try
        {
            Logger.Log("Window_Loaded");
            if (ViewModel == null) { Logger.LogError("Window_Loaded", new Exception("ViewModel null")); return; }
            await ViewModel.LoadTenantsCommand.ExecuteAsync(null);
            await ViewModel.LoadSessionsCommand.ExecuteAsync(null);
        }
        catch (Exception ex)
        {
            Logger.LogError("Window_Loaded", ex);
        }
    }

    private async void CommandInput_KeyDown(object sender, KeyEventArgs e)
    {
        // Atajos globales mientras se escribe: Ctrl+R refresca, Ctrl+L limpia, F1 ayuda.
        if (e.KeyboardDevice.Modifiers.HasFlag(ModifierKeys.Control) && e.Key == Key.R)
        {
            e.Handled = true;
            if (ViewModel?.RefreshAllCommand != null) await ViewModel.RefreshAllCommand.ExecuteAsync(null);
            return;
        }
        if (e.KeyboardDevice.Modifiers.HasFlag(ModifierKeys.Control) && e.Key == Key.L)
        {
            e.Handled = true;
            ViewModel?.ClearConsoleCommand?.Execute(null);
            return;
        }
        if (e.Key == Key.F1)
        {
            e.Handled = true;
            await (ViewModel?.SendCommandCommand?.ExecuteAsync("help") ?? Task.CompletedTask);
            return;
        }
        if (e.Key == Key.Tab)
        {
            e.Handled = true;
            string cur = CommandInput.Text ?? "";
            var matches = _allCommands.Where(c => c.StartsWith(cur, StringComparison.OrdinalIgnoreCase)).ToArray();
            if (matches.Length > 0)
            {
                CommandInput.Text = matches[0];
                CommandInput.CaretIndex = CommandInput.Text.Length;
            }
            return;
        }
        if (e.Key == Key.Up)
        {
            if (_cmdHistory.Count > 0)
            {
                _historyIndex = Math.Min(_historyIndex + 1, _cmdHistory.Count - 1);
                CommandInput.Text = _cmdHistory[_historyIndex];
                CommandInput.CaretIndex = CommandInput.Text.Length;
                e.Handled = true;
            }
            return;
        }
        if (e.Key == Key.Down)
        {
            if (_historyIndex > 0) { _historyIndex--; CommandInput.Text = _cmdHistory[_historyIndex]; }
            else { _historyIndex = -1; CommandInput.Text = ""; }
            e.Handled = true;
            return;
        }
        if (e.Key == Key.Enter)
        {
            if (ViewModel == null) return;
            var command = CommandInput.Text;
            if (!string.IsNullOrWhiteSpace(command))
            {
                _cmdHistory.Insert(0, command);
                if (_cmdHistory.Count > 100) _cmdHistory.RemoveAt(100);
                _historyIndex = -1;
                await ViewModel.SendCommandCommand.ExecuteAsync(command);
                CommandInput.Text = "";
                CommandInput.Focus();
            }
        }
    }

    private async void SendCommand_Click(object sender, RoutedEventArgs e)
    {
        if (ViewModel == null) return;
        var command = CommandInput.Text;
        if (string.IsNullOrWhiteSpace(command)) return;
        await ViewModel.SendCommandCommand.ExecuteAsync(command);
        CommandInput.Text = "";
        CommandInput.Focus();
    }

    private void ClearConsole_Click(object sender, RoutedEventArgs e)
    {
        if (ViewModel == null) return;
        ViewModel.ConsoleOutput.Clear();
        ViewModel.LogInfo("Console limpiada (Ctrl+L)");
    }

    private void CopyConsole_Click(object sender, RoutedEventArgs e)
    {
        if (ViewModel == null) return;
        var txt = string.Join("\n", ViewModel.ConsoleOutput.Select(m => $"[{m.Timestamp:HH:mm:ss}][{m.Direction}] {m.Content}"));
        try
        {
            System.Windows.Clipboard.SetText(txt);
            ViewModel.LogInfo("Consola copiada al portapapeles.");
        }
        catch (Exception ex)
        {
            ViewModel.LogError($"No se pudo copiar: {ex.Message}");
        }
    }

    private void ExportLogs_Click(object sender, RoutedEventArgs e)
    {
        if (ViewModel == null) return;
        try
        {
            var path = Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                "C2Dect", "logs",
                $"consola-{DateTime.Now:yyyyMMdd-HHmmss}.txt");
            Directory.CreateDirectory(Path.GetDirectoryName(path)!);
            File.WriteAllLines(path, ViewModel.ConsoleOutput.Select(m => $"[{m.Timestamp:yyyy-MM-dd HH:mm:ss}][{m.Direction}] {m.Content}"));
            System.Windows.MessageBox.Show($"Consola exportada:\n{path}", "Exportar consola",
                System.Windows.MessageBoxButton.OK, System.Windows.MessageBoxImage.Information);
        }
        catch (Exception ex)
        {
            ViewModel.LogError($"No se pudo exportar: {ex.Message}");
        }
    }

    private void QuickSearch_TextChanged(object sender, System.Windows.Controls.TextChangedEventArgs e)
    {
        var q = (QuickSearch.Text ?? "").Trim().ToLowerInvariant();
        if (QuickCommandsHost == null) return;
        foreach (var child in FindVisualChildren<System.Windows.Controls.Button>(QuickCommandsHost))
        {
            var content = (child.Content?.ToString() ?? "").ToLowerInvariant();
            child.Visibility = string.IsNullOrEmpty(q) || content.Contains(q)
                ? Visibility.Visible : Visibility.Collapsed;
        }
    }

    private static IEnumerable<T> FindVisualChildren<T>(System.Windows.DependencyObject dep) where T : System.Windows.DependencyObject
    {
        if (dep == null) yield break;
        for (int i = 0; i < System.Windows.Media.VisualTreeHelper.GetChildrenCount(dep); i++)
        {
            var child = System.Windows.Media.VisualTreeHelper.GetChild(dep, i);
            if (child is T t) yield return t;
            foreach (var sub in FindVisualChildren<T>(child)) yield return sub;
        }
    }

    private async void QuickCommand_Click(object sender, RoutedEventArgs e)
    {
        if (ViewModel == null) return;
        if (sender is System.Windows.Controls.Button btn)
        {
            var command = btn.Content.ToString();
            if (!string.IsNullOrEmpty(command))
            {
                CommandInput.Text = command;
                await ViewModel.SendCommandCommand.ExecuteAsync(command);
                CommandInput.Text = "";
            }
        }
    }

    private async void HelpButton_Click(object sender, RoutedEventArgs e)
    {
        if (ViewModel == null) return;
        await ViewModel.SendCommandCommand.ExecuteAsync("help");
    }

    // --- modulos que la edicion Lite no incluye -----------------------------

    private void NotInLite(string module)
    {
        ViewModel?.LogWarn($"El modulo '{module}' no forma parte de la edicion Lite (open source).");
    }

    private void AiAsk_Click(object sender, RoutedEventArgs e)
    {
        NotInLite("IA Copilot");
        if (AiOutput != null) AiOutput.Text = "El asistente IA no forma parte de la edicion Lite (open source).";
    }

    private void AiSuggest_Click(object sender, RoutedEventArgs e)
    {
        NotInLite("IA Copilot");
        if (AiOutput != null) AiOutput.Text = "El asistente IA no forma parte de la edicion Lite (open source).";
    }

    private void AiAnalyze_Click(object sender, RoutedEventArgs e)
    {
        NotInLite("IA Copilot");
        if (AiOutput != null) AiOutput.Text = "El asistente IA no forma parte de la edicion Lite (open source).";
    }
}
