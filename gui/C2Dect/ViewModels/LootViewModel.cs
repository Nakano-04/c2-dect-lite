using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using C2Dect.Lite.Models;
using C2Dect.Lite.Services;
using System.Collections.ObjectModel;
using System.Threading.Tasks;

namespace C2Dect.Lite.ViewModels;

public partial class LootViewModel : ObservableObject
{
    private readonly ApiService _api;

    [ObservableProperty] private Loot? _selectedLoot;
    [ObservableProperty] private string _filterSession = "";

    public ObservableCollection<Loot> LootItems { get; } = new();

    public LootViewModel(ApiService api)
    {
        _api = api;
    }

    private void Dispatch(System.Action action)
    {
        if (App.Current?.Dispatcher == null || App.Current.Dispatcher.CheckAccess())
            action();
        else
            App.Current.Dispatcher.Invoke(action);
    }

    [RelayCommand]
    private async Task LoadLoot()
    {
        var loots = await _api.GetLoot(FilterSession);
        Dispatch(() =>
        {
            LootItems.Clear();
            if (loots != null)
                foreach (var l in loots)
                    LootItems.Add(l);
        });
    }
}
