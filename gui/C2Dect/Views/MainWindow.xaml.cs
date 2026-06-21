using System;
using System.Windows;
using System.Windows.Input;
using C2Dect.Lite.ViewModels;

namespace C2Dect.Lite.Views;

public partial class MainWindow : Window
{
    private MainViewModel ViewModel => (MainViewModel)DataContext;

    public MainWindow()
    {
        try
        {
            Logger.Log("MainWindow constructing...");
            InitializeComponent();
            DataContext = new MainViewModel(App.ApiService!, App.AuthService!);
            Logger.Log("MainWindow DataContext set");

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
            await ViewModel.LoadSessionsCommand.ExecuteAsync(null);
        }
        catch (Exception ex)
        {
            Logger.LogError("Window_Loaded", ex);
        }
    }

    private async void CommandInput_KeyDown(object sender, KeyEventArgs e)
    {
        if (e.Key == Key.Enter)
        {
            var command = CommandInput.Text;
            if (!string.IsNullOrWhiteSpace(command))
            {
                await ViewModel.SendCommandCommand.ExecuteAsync(command);
                CommandInput.Text = "";
                CommandInput.Focus();
            }
        }
    }

    private async void SendCommand_Click(object sender, RoutedEventArgs e)
    {
        var command = CommandInput.Text;
        if (!string.IsNullOrWhiteSpace(command))
        {
            await ViewModel.SendCommandCommand.ExecuteAsync(command);
            CommandInput.Text = "";
            CommandInput.Focus();
        }
    }

    private void ClearConsole_Click(object sender, RoutedEventArgs e)
    {
        ViewModel.ConsoleOutput.Clear();
        ViewModel.LogInfo("Console cleared");
    }

    private async void QuickCommand_Click(object sender, RoutedEventArgs e)
    {
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
        await ViewModel.SendCommandCommand.ExecuteAsync("help");
    }
}
