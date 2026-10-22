using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using C2Dect.Models;
using C2Dect.Services;
using System.Collections.ObjectModel;
using System.Threading.Tasks;

namespace C2Dect.ViewModels;

public partial class LootViewModel : ObservableObject
{
    private readonly ApiService _api;
    private readonly TenantService _tenants;

    [ObservableProperty] private Loot? _selectedLoot;
    [ObservableProperty] private string _filterSession = "";
    [ObservableProperty] private string _scopeNote = "";

    public ObservableCollection<Loot> LootItems { get; } = new();

    /// <summary>Tenant contra el que estÃ¡ cargado LootItems ahora mismo.</summary>
    public long LoadedTenantId { get; private set; }

    public LootViewModel(ApiService api, TenantService tenants)
    {
        _api = api;
        _tenants = tenants;
    }

    private void Dispatch(System.Action action)
    {
        if (App.Current?.Dispatcher == null || App.Current.Dispatcher.CheckAccess())
            action();
        else
            App.Current.Dispatcher.BeginInvoke(action);
    }

    [RelayCommand]
    public async Task LoadLoot()
    {
        var scope = _tenants.BeginScope();
        if (!scope.CanPaint)
        {
            ScopeNote = "Sin tenant resuelto: no se carga loot.";
            return;
        }

        ScopeNote = "";
        var loots = await _api.GetLoot(FilterSession);

        if (!scope.CanPaint)
        {
            ScopeNote = $"Respuesta de loot del tenant #{scope.TenantId} descartada: el activo es {_tenants.ScopeLabel}.";
            return;
        }

        Dispatch(() =>
        {
            LootItems.Clear();
            if (loots != null)
                foreach (var l in loots)
                    LootItems.Add(l);
            LoadedTenantId = scope.TenantId;
            ScopeNote = $"Loot del tenant #{scope.TenantId} ({LootItems.Count}).";
        });
    }
}

