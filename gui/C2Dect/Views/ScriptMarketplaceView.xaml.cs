using System;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Threading;
using C2Dect.ViewModels;

namespace C2Dect.Views;

/// <summary>
/// Script Market (vista). La logica vive en ScriptMarketplaceViewModel; aqui
/// solo se engancha la carga inicial: al entrar en la vista se pide el listado
/// de scripts al servidor para que el operador no tenga que pulsar REFRESCAR
/// a mano.
/// </summary>
public partial class ScriptMarketplaceView : UserControl
{
    /// <summary>Evita repetir la carga automatica cada vez que se muestra la vista.</summary>
    private bool _autoLoaded;

    public ScriptMarketplaceView()
    {
        InitializeComponent();
        Loaded += OnLoaded;
    }

    private void OnLoaded(object sender, RoutedEventArgs e)
    {
        if (_autoLoaded) return;
        _autoLoaded = true;

        // El DataContext lo inyecta el DataTemplate de MainWindow.xaml; se
        // aplaza un tick para que ya este puesto cuando se lee.
        Dispatcher.BeginInvoke(new Action(() =>
        {
            if (DataContext is ScriptMarketplaceViewModel vm)
                _ = vm.RefreshCommand.ExecuteAsync(null);
        }), DispatcherPriority.Background);
    }
}