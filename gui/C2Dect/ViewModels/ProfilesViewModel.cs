using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using C2Dect.Models;
using C2Dect.Services;
using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading.Tasks;

namespace C2Dect.ViewModels;

/// <summary>
/// Perfiles malleable del C2 (GET/POST /api/profiles).
///
/// Antes la GUI no tenÃ­a ninguna vista de perfiles: el endpoint existÃ­a en el
/// servidor y el ApiService lo tenÃ­a, pero ningÃºn ViewModel lo consumÃ­a, asÃ­
/// que el operador no podÃ­a ver ni crear el perfil de trÃ¡fico que usa un
/// listener.
/// </summary>
public partial class ProfilesViewModel : ObservableObject
{
    private readonly ApiService _api;

    [ObservableProperty] private MalleableProfile? _selectedProfile;
    [ObservableProperty] private string _statusText = "Perfiles malleable";
    [ObservableProperty] private string _newName = "";
    [ObservableProperty] private string _newDescription = "";
    [ObservableProperty] private string _newMethod = "POST";
    [ObservableProperty] private string _newContentType = "application/json";
    [ObservableProperty] private string _newEncodeMode = "base64";
    [ObservableProperty] private int _newJitter = 30;
    [ObservableProperty] private int _newDefaultSleep = 10;
    [ObservableProperty] private string _newUris = "/submit.php,/index.php";
    [ObservableProperty] private string _newUserAgents = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36";
    [ObservableProperty] private string _newPostTemplate = "";

    public ObservableCollection<MalleableProfile> Profiles { get; } = new();

    public string[] Methods => new[] { "GET", "POST" };
    public string[] EncodeModes => new[] { "base64", "base64url", "netbios", "netbiosu", "hex" };

    public ProfilesViewModel(ApiService api)
    {
        _api = api;
    }

    private void Dispatch(Action action)
    {
        if (App.Current?.Dispatcher == null || App.Current.Dispatcher.CheckAccess())
            action();
        else
            App.Current.Dispatcher.BeginInvoke(action);
    }

    /// <summary>GET /api/profiles - perfiles que el C2 tiene publicados.</summary>
    [RelayCommand]
    public async Task LoadProfiles()
    {
        List<MalleableProfile>? list;
        string? err;
        try
        {
            _api.ClearLastError();
            list = await _api.GetProfiles();
            err = _api.LastError;
        }
        catch (Exception ex)
        {
            Logger.Log($"[profiles] fallo GetProfiles: {ex.Message}");
            list = null;
            err = $"fallo de transporte: {ex.Message}";
        }
        Dispatch(() =>
        {
            Profiles.Clear();
            if (list != null) foreach (var p in list) Profiles.Add(p);
            StatusText = err != null
                ? $"No se pudieron cargar los perfiles. {err}"
                : $"{Profiles.Count} perfil(es) malleable en el servidor";
        });
    }

    /// <summary>
    /// POST /api/profiles - crea un perfil nuevo (solo role=admin en el
    /// servidor). El servidor responde {"status":"created"}, no el perfil: por
    /// eso la lista se recarga despuÃ©s de crear en lugar de usar el retorno.
    /// </summary>
    [RelayCommand]
    private async Task CreateProfile()
    {
        if (string.IsNullOrWhiteSpace(NewName)) { StatusText = "Nombre de perfil requerido."; return; }

        var profile = new MalleableProfile
        {
            Name = NewName.Trim(),
            Description = NewDescription,
            Method = NewMethod,
            ContentType = NewContentType,
            EncodeMode = NewEncodeMode,
            Jitter = NewJitter,
            DefaultSleep = NewDefaultSleep,
            PostTemplate = NewPostTemplate,
        };
        profile.URIs = SplitList(NewUris);
        profile.UserAgents = SplitList(NewUserAgents);

        _api.ClearLastError();
        bool ok;
        string? err;
        try
        {
            var created = await _api.CreateProfile(profile);
            ok = created != null;
            err = _api.LastError;
        }
        catch (Exception ex)
        {
            ok = false;
            err = ex.Message;
        }

        Dispatch(() =>
        {
            if (ok)
            {
                StatusText = $"Perfil '{NewName}' creado en el servidor.";
                NewName = "";
                NewDescription = "";
                NewPostTemplate = "";
            }
            else
            {
                StatusText = $"No se pudo crear el perfil. {err ?? "el servidor lo rechazÃ³"}";
            }
        });

        await LoadProfiles();
    }

    private static System.Collections.Generic.List<string> SplitList(string? raw)
    {
        var list = new System.Collections.Generic.List<string>();
        if (string.IsNullOrWhiteSpace(raw)) return list;
        foreach (var part in raw.Split(new[] { ',', ';', '\n' }, StringSplitOptions.RemoveEmptyEntries))
        {
            var t = part.Trim();
            if (t.Length > 0 && !list.Contains(t)) list.Add(t);
        }
        return list;
    }
}
