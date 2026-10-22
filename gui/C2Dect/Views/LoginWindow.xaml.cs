using System;
using System.IO;
using System.Net.Sockets;
using System.Threading.Tasks;
using System.Windows;
using C2Dect.Services;

namespace C2Dect.Views;

/// <summary>
/// Ventana de acceso de la consola Lite.
///
/// Misma pantalla que la consola completa (servidor, usuario, clave, TOTP
/// opcional y arranque del servidor en segundo plano), con el flujo adaptado al
/// servidor Lite 2.0 y el restaurado automatico de la sesion.
/// </summary>
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

    /// <summary>
    /// Guarda que la consola ya se abrio. El login y el restaurado de sesion
    /// pueden correr a la vez (el operador hace clic mientras se restaura el
    /// refresh token) y sin este guardia se abrian dos consolas.
    /// </summary>
    private bool _consoleOpened;

    private void OpenConsole()
    {
        if (_consoleOpened) return;
        _consoleOpened = true;
        var main = new MainWindow();
        main.Show();
        Close();
    }

    /// <summary>
    /// Restaura la sesion guardada. El refresh token esta protegido con
    /// DPAPI, asi que solo la cuenta de Windows que inicio sesion puede
    /// reutilizarlo. Si no hay sesion o ya no vale, se muestra el login.
    /// </summary>
    private async void Window_Loaded(object sender, RoutedEventArgs e)
    {
        try
        {
            StatusText.Text = "Restaurando la sesion anterior...";
            if (await App.AuthService!.TryRestoreAsync())
            {
                Logger.Log("Session restored, opening the console");
                await App.TenantService!.LoadAllAsync();
                OpenConsole();
                return;
            }
            StatusText.Text = "";
        }
        catch (Exception ex)
        {
            Logger.LogError("Window_Loaded", ex);
            StatusText.Text = "";
        }
    }

    private async void BackgroundServerCheck_Checked(object sender, RoutedEventArgs e)
    {
        await EnsureBackgroundServerAsync();
    }

    private void BackgroundServerCheck_Unchecked(object sender, RoutedEventArgs e)
    {
        StatusText.Text = "Modo 2º plano desactivado.";
    }

    /// <summary>Arranca el servidor Lite si el puerto indicado esta libre.</summary>
    private async Task<bool> EnsureBackgroundServerAsync()
    {
        var url = ServerBox?.Text ?? "http://127.0.0.1:8443";
        var (host, port, _) = ParseUrl(url);
        if (await IsPortOpenAsync(host, port)) return true;

        if (App.StartLiteServer(out var message))
        {
            StatusText.Text = message;
            for (int i = 0; i < 20; i++)
            {
                await Task.Delay(300);
                if (await IsPortOpenAsync(host, port)) return true;
            }
            StatusText.Text = "El servidor arranco pero no responde todavia. Intenta entrar en unos segundos.";
            return false;
        }
        StatusText.Text = message;
        return false;
    }

    private static async Task<bool> IsPortOpenAsync(string host, int port)
    {
        using var tcp = new TcpClient();
        try
        {
            using var cts = new System.Threading.CancellationTokenSource(TimeSpan.FromSeconds(1));
            await tcp.ConnectAsync(host, port, cts.Token);
            return true;
        }
        catch
        {
            return false;
        }
    }

    private static (string host, int port, bool wantTls) ParseUrl(string url)
    {
        try
        {
            var u = new Uri(url);
            int pt = u.Port == -1 ? (u.Scheme == "https" ? 443 : 80) : u.Port;
            return (u.Host, pt, u.Scheme == "https");
        }
        catch
        {
            return ("127.0.0.1", 8443, false);
        }
    }

    private async void LoginButton_Click(object sender, RoutedEventArgs e)
    {
        var url = ServerBox.Text;
        var user = UsernameBox.Text;
        var pass = PasswordBox.Password;
        var totp = TotpBox.Text;

        if (string.IsNullOrWhiteSpace(user) || string.IsNullOrWhiteSpace(pass))
        {
            StatusText.Text = "Introduce usuario y clave.";
            return;
        }

        StatusText.Text = "Conectando...";
        Logger.Log($"Login attempt: {user}@{url}");

        try
        {
            App.ApiService!.BaseUrl = url.TrimEnd('/');
            App.ApiService.SetInsecure(InsecureBox.IsChecked == true);

            var (ok, error) = await App.AuthService!.Login(user, pass, string.IsNullOrWhiteSpace(totp) ? null : totp);
            if (!ok)
            {
                StatusText.Text = string.IsNullOrWhiteSpace(error) ? "No se pudo iniciar sesion." : error;
                return;
            }

            Logger.Log("Login successful");
            await App.TenantService!.LoadAllAsync();

            OpenConsole();
        }
        catch (Exception ex)
        {
            Logger.LogError("LoginButton_Click", ex);
            StatusText.Text = $"Error: {ex.Message}";
        }
    }
}
