using System;
using System.Windows;
using C2Dect.Lite.Services;

namespace C2Dect.Lite.Views;

public partial class LoginWindow : Window
{
    public LoginWindow()
    {
        try
        {
            Logger.Log("LoginWindow constructing...");
            InitializeComponent();
            Logger.Log("LoginWindow constructed OK");
        }
        catch (Exception ex)
        {
            Logger.LogError("LoginWindow constructor", ex);
            throw;
        }
    }

    private async void LoginButton_Click(object sender, RoutedEventArgs e)
    {
        var serverUrl = ServerBox.Text;
        var username = UsernameBox.Text;
        var password = PasswordBox.Password;

        if (string.IsNullOrWhiteSpace(username) || string.IsNullOrWhiteSpace(password))
        {
            StatusText.Text = "Please enter credentials";
            return;
        }

        StatusText.Text = "Connecting...";
        Logger.Log($"Login attempt: {username}@{serverUrl}");

        try
        {
            App.ApiService!.BaseUrl = serverUrl;

            var success = await App.AuthService!.Login(username, password, serverUrl);
            if (success)
            {
                Logger.Log("Login successful");
                var mainWindow = new MainWindow();
                mainWindow.Show();
                Close();
            }
            else
            {
                Logger.Log("Login failed");
                StatusText.Text = "Login failed. Check credentials and server.";
            }
        }
        catch (Exception ex)
        {
            Logger.LogError("LoginButton_Click", ex);
            StatusText.Text = $"Error: {ex.Message}";
        }
    }
}
