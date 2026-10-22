using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using C2Dect.Models;
using C2Dect.Services;
using System;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading.Tasks;
using AgentTokenRow = C2Dect.Services.ApiService.AgentTokenRow;

namespace C2Dect.ViewModels;

/// <summary>
/// Panel de gobernanza: tenants, operadores, tokens de arranque y traza de
/// auditoria. Es el recorte Lite del panel Admin de la consola completa: el
/// servidor Lite no tiene las tablas de categorias, permisos y roles, asi que
/// la consola no las muestra.
/// </summary>
public partial class AdminViewModel : ObservableObject
{
    private readonly ApiService _api;
    private readonly TenantService _tenants;

    [ObservableProperty] private string _statusText = "";
    [ObservableProperty] private string _newTenantName = "";
    [ObservableProperty] private string _newTenantDescription = "";
    [ObservableProperty] private string _newUserName = "";
    [ObservableProperty] private string _newUserPassword = "";
    [ObservableProperty] private string _newUserRole = "operator";
    [ObservableProperty] private Tenant? _selectedTenant;
    [ObservableProperty] private string _tokenLabel = "";
    [ObservableProperty] private int _tokenHours = 24;
    [ObservableProperty] private string _issuedToken = "";

    public ObservableCollection<Tenant> Tenants { get; } = new();
    public ObservableCollection<Operator> Operators { get; } = new();
    public ObservableCollection<AgentTokenRow> Tokens { get; } = new();

    public string[] Roles => new[] { "admin", "operator", "viewer" };

    public AdminViewModel(ApiService api, TenantService tenants)
    {
        _api = api;
        _tenants = tenants;
    }

    private void Dispatch(Action action)
    {
        if (App.Current?.Dispatcher == null || App.Current.Dispatcher.CheckAccess())
            action();
        else
            App.Current.Dispatcher.Invoke(action);
    }

    [RelayCommand]
    public async Task LoadTenants()
    {
        await _tenants.LoadAllAsync();
        try
        {
            var tenants = await _api.GetTenants();
            var operators = await _api.GetUsersByTenant(_tenants.TokenTenantId);
            Dispatch(() =>
            {
                Tenants.Clear();
                if (tenants != null) foreach (var t in tenants) Tenants.Add(t);
                Operators.Clear();
                if (operators != null) foreach (var o in operators) Operators.Add(o);
                StatusText = $"{Tenants.Count} tenant(s), {Operators.Count} operador(es) en el tenant activo";
            });
        }
        catch (Exception ex)
        {
            StatusText = ex.Message;
        }
    }

    [RelayCommand]
    private async Task LoadTokens()
    {
        if (SelectedTenant == null)
        {
            StatusText = "Selecciona un tenant.";
            return;
        }
        try
        {
            var tokens = await _api.GetAgentTokens(SelectedTenant.Id);
            Dispatch(() =>
            {
                Tokens.Clear();
                if (tokens != null) foreach (var t in tokens) Tokens.Add(t);
                StatusText = $"{Tokens.Count} token(s) de arranque";
            });
        }
        catch (Exception ex)
        {
            StatusText = ex.Message;
        }
    }

    [RelayCommand]
    private async Task CreateTenant()
    {
        if (string.IsNullOrWhiteSpace(NewTenantName))
        {
            StatusText = "El tenant necesita un nombre.";
            return;
        }
        try
        {
            var tenant = await _api.CreateTenant(NewTenantName.Trim());
            StatusText = tenant != null ? $"Tenant '{tenant.Name}' creado." : "El servidor no creo el tenant.";
            NewTenantName = "";
            NewTenantDescription = "";
            await LoadTenants();
        }
        catch (Exception ex)
        {
            StatusText = ex.Message;
        }
    }

    [RelayCommand]
    private async Task DeleteTenant()
    {
        if (SelectedTenant == null)
        {
            StatusText = "Selecciona un tenant.";
            return;
        }
        if (SelectedTenant.Id == 1)
        {
            StatusText = "El tenant por defecto no se puede borrar.";
            return;
        }
        var ok = await _api.DeleteTenant(SelectedTenant.Id);
        StatusText = ok ? $"Tenant {SelectedTenant.Name} borrado." : $"No se pudo borrar. {_api.LastError}";
        await LoadTenants();
    }

    [RelayCommand]
    private async Task CreateOperator()
    {
        if (string.IsNullOrWhiteSpace(NewUserName) || NewUserPassword.Length < 12)
        {
            StatusText = "El operador necesita nombre y una clave de 12 caracteres o mas.";
            return;
        }
        var tenantId = SelectedTenant?.Id ?? _tenants.TokenTenantId;
        var ok = await _api.CreateUserTenant(NewUserName.Trim(), NewUserPassword, tenantId, new System.Collections.Generic.List<long>());
        StatusText = ok ? $"Operador {NewUserName} creado." : $"No se pudo crear. {_api.LastError}";
        if (ok)
        {
            NewUserName = "";
            NewUserPassword = "";
            await LoadTenants();
        }
    }

    [RelayCommand]
    private async Task IssueToken()
    {
        if (SelectedTenant == null)
        {
            StatusText = "Selecciona un tenant.";
            return;
        }
        try
        {
            var created = await _api.CreateAgentToken(SelectedTenant.Id, TokenLabel, TokenHours);
            IssuedToken = created?.token ?? "";
            StatusText = string.IsNullOrEmpty(IssuedToken)
                ? "El servidor no emitio el token."
                : "Token emitido. Solo se muestra una vez: guardalo ahora.";
            await LoadTokens();
        }
        catch (Exception ex)
        {
            StatusText = ex.Message;
        }
    }

    [RelayCommand]
    private async Task RevokeToken(AgentTokenRow? token)
    {
        if (token == null || SelectedTenant == null) return;
        var ok = await _api.RevokeAgentToken(SelectedTenant.Id, token.id);
        StatusText = ok ? "Token revocado." : $"No se pudo revocar. {_api.LastError}";
        await LoadTokens();
    }
}

/// <summary>Panel de la traza de auditoria encadenada por hash.</summary>
public partial class AuditViewModel : ObservableObject
{
    private readonly ApiService _api;

    [ObservableProperty] private string _chainStatus = "sin verificar";
    [ObservableProperty] private bool _chainValid;
    [ObservableProperty] private int _count;

    public ObservableCollection<ApiService.AuditRow> Entries { get; } = new();

    public AuditViewModel(ApiService api) => _api = api;

    private void Dispatch(Action action)
    {
        if (App.Current?.Dispatcher == null || App.Current.Dispatcher.CheckAccess())
            action();
        else
            App.Current.Dispatcher.Invoke(action);
    }

    [RelayCommand]
    public async Task LoadAudit()
    {
        try
        {
            var entries = await _api.GetAudit(300);
            Dispatch(() =>
            {
                Entries.Clear();
                if (entries != null) foreach (var e in entries) Entries.Add(e);
                Count = Entries.Count;
            });
            await VerifyChain();
        }
        catch (Exception ex)
        {
            ChainStatus = ex.Message;
        }
    }

    [RelayCommand]
    private async Task VerifyChain()
    {
        try
        {
            var result = await _api.VerifyAuditChainAsync();
            ChainValid = result?.valid ?? false;
            ChainStatus = result?.message ?? "sin verificar";
        }
        catch (Exception ex)
        {
            ChainValid = false;
            ChainStatus = ex.Message;
        }
    }
}


