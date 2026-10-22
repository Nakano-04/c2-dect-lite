using System;
using System.Diagnostics;
using System.IO;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Threading;
using C2Dect.Services;

namespace C2Dect;

/// <summary>
/// Arranque de la consola Lite.
///
/// A diferencia de la consola completa, aqui no se arranca ningun componente
/// ofensivo: solo el servidor Lite (build/server.exe) si el operador lo pide
/// desde la ventana de login.
/// </summary>
public partial class App : Application
{
    public static ApiService? ApiService { get; private set; }
    public static AuthService? AuthService { get; private set; }
    public static TenantService? TenantService { get; private set; }

    protected override void OnStartup(StartupEventArgs e)
    {
        base.OnStartup(e);

        Logger.Init();
        Logger.Log("C2-DECT Lite 2.0 starting...");

        DispatcherUnhandledException += OnDispatcherUnhandledException;
        AppDomain.CurrentDomain.UnhandledException += OnUnhandledException;

        try
        {
            ApiService = new ApiService();
            AuthService = new AuthService(ApiService);
            TenantService = new TenantService(ApiService, AuthService);
            Logger.Log("Services initialized");
        }
        catch (Exception ex)
        {
            Logger.LogFatal("Service init", ex);
            MessageBox.Show($"Failed to initialize services:\n{ex.Message}", "Error",
                MessageBoxButton.OK, MessageBoxImage.Error);
            Shutdown();
        }
    }

    private void OnDispatcherUnhandledException(object sender, DispatcherUnhandledExceptionEventArgs e)
    {
        Logger.LogError("DispatcherUnhandledException", e.Exception);
        e.Handled = true;
    }

    private void OnUnhandledException(object sender, UnhandledExceptionEventArgs e)
    {
        if (e.ExceptionObject is Exception ex)
        {
            Logger.LogFatal("UnhandledException", ex);
        }
    }

    protected override void OnExit(ExitEventArgs e)
    {
        try { Logger.Log("App exiting"); } catch { }
        base.OnExit(e);
    }

    /// <summary>
    /// Arranca el servidor Lite ya compilado (build/server.exe). La consola no
    /// compila Go: si el binario no esta, el operador lo construye con
    /// build.bat y arranca el servidor por su cuenta.
    /// </summary>
    public static bool StartLiteServer(out string message)
    {
        try
        {
            var exe = FindServerExe();
            if (exe == null)
            {
                message = "No se encontro build/server.exe. Compila el servidor con build.bat.";
                return false;
            }
            var psi = new ProcessStartInfo(exe)
            {
                UseShellExecute = false,
                CreateNoWindow = true,
                WorkingDirectory = Path.GetDirectoryName(exe) ?? AppDomain.CurrentDomain.BaseDirectory,
            };
            var proc = Process.Start(psi);
            message = proc != null
                ? $"Servidor Lite arrancado en segundo plano (PID {proc.Id})."
                : "No se pudo arrancar el servidor.";
            return proc != null;
        }
        catch (Exception ex)
        {
            message = $"No se pudo arrancar el servidor: {ex.Message}";
            return false;
        }
    }

    /// <summary>Busca build/server.exe subiendo niveles desde el binario de la GUI.</summary>
    public static string? FindServerExe()
    {
        var root = AppDomain.CurrentDomain.BaseDirectory;
        for (int i = 0; i < 6 && root != null; i++)
        {
            var candidate = Path.Combine(root, "build", "server.exe");
            if (File.Exists(candidate)) return candidate;
            root = Directory.GetParent(root)?.FullName;
        }
        return null;
    }
}
