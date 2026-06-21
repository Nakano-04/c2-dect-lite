using System;
using System.Windows;
using System.Windows.Threading;
using C2Dect.Lite.Services;

namespace C2Dect.Lite;

public partial class App : Application
{
    public static ApiService? ApiService { get; private set; }
    public static AuthService? AuthService { get; private set; }

    protected override void OnStartup(StartupEventArgs e)
    {
        base.OnStartup(e);

        Logger.Init();
        Logger.Log("App starting...");

        DispatcherUnhandledException += OnDispatcherUnhandledException;
        AppDomain.CurrentDomain.UnhandledException += OnUnhandledException;

        try
        {
            ApiService = new ApiService();
            AuthService = new AuthService(ApiService);
            Logger.Log("Services initialized");
        }
        catch (Exception ex)
        {
            Logger.LogFatal("Service init", ex);
            MessageBox.Show($"Failed to initialize services:\n{ex.Message}", "Error", MessageBoxButton.OK, MessageBoxImage.Error);
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
            Logger.LogFatal("UnhandledException", ex);
    }

    protected override void OnExit(ExitEventArgs e)
    {
        Logger.Log("App exiting");
        base.OnExit(e);
    }
}
