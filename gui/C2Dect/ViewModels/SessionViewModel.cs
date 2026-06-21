using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using C2Dect.Lite.Models;
using C2Dect.Lite.Services;
using System.Collections.ObjectModel;
using System.Threading.Tasks;

namespace C2Dect.Lite.ViewModels;

public partial class SessionViewModel : ObservableObject
{
    private readonly ApiService _api;

    [ObservableProperty] private Session? _selectedSession;
    [ObservableProperty] private string _commandInput = "";
    [ObservableProperty] private string _filterStatus = "";
    [ObservableProperty] private string _searchText = "";

    public ObservableCollection<Session> FilteredSessions { get; } = new();
    public ObservableCollection<ConsoleMessage> ConsoleOutput { get; } = new();
    public ObservableCollection<C2Task> Tasks { get; } = new();

    public SessionViewModel(ApiService api)
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
    private async Task LoadSessions()
    {
        var sessions = await _api.GetSessions(FilterStatus);
        Dispatch(() =>
        {
            FilteredSessions.Clear();
            if (sessions != null)
                foreach (var s in sessions)
                    FilteredSessions.Add(s);
        });
    }

    [RelayCommand]
    private async Task ExecuteCommand()
    {
        if (SelectedSession == null || string.IsNullOrWhiteSpace(CommandInput)) return;

        Dispatch(() =>
        {
            ConsoleOutput.Add(new ConsoleMessage
            {
                Direction = "sent",
                Content = CommandInput
            });
        });

        var task = await _api.SubmitTask(SelectedSession.Id, CommandInput);
        if (task != null)
        {
            Dispatch(() =>
            {
                Tasks.Insert(0, task);
                CommandInput = "";
            });
        }
    }

    [RelayCommand]
    private async Task ChangeSleep(string seconds)
    {
        if (SelectedSession == null) return;
        if (int.TryParse(seconds, out int sec))
        {
            await _api.SetSleep(SelectedSession.Id, sec);
            Dispatch(() =>
            {
                ConsoleOutput.Add(new ConsoleMessage
                {
                    Direction = "info",
                    Content = $"Sleep changed to {sec}s"
                });
            });
        }
    }

    [RelayCommand]
    private async Task KillSession()
    {
        if (SelectedSession == null) return;
        await _api.KillSession(SelectedSession.Id);
        Dispatch(() =>
        {
            ConsoleOutput.Add(new ConsoleMessage
            {
                Direction = "info",
                Content = $"Kill command sent to {SelectedSession.DisplayName}"
            });
        });
    }

    [RelayCommand]
    private async Task TagSession(string tags)
    {
        if (SelectedSession == null) return;
        await _api.TagSession(SelectedSession.Id, tags);
    }
}
