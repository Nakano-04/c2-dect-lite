using CommunityToolkit.Mvvm.ComponentModel;
using Newtonsoft.Json;
using System;

namespace C2Dect.Models;

/// <summary>
/// Estado del selector de tenant. La GUI solo opera en <see cref="Resolved"/>:
/// cualquier otro valor significa "no se sabe sobre qué tenant se opera".
/// </summary>
public enum TenantScopeState
{
    Unresolved,
    Resolved,
    Faulted
}

public sealed class TenantChangedEventArgs : EventArgs
{
    public TenantChangedEventArgs(Tenant? previous, Tenant? current, TenantScopeState state, string reason)
    {
        Previous = previous;
        Current = current;
        State = state;
        Reason = reason;
    }

    public Tenant? Previous { get; }
    public Tenant? Current { get; }
    public TenantScopeState State { get; }
    public string Reason { get; }
}

/// <summary>
/// db.Tenant tal cual lo serializa server/db/database.go:1162-1167.
/// </summary>
public partial class Tenant : ObservableObject
{
    [ObservableProperty] private long _id;
    [ObservableProperty] private string _name = "";
    [ObservableProperty] [JsonProperty("allow_impact_modules")] private bool _allowImpactModules;
    [ObservableProperty] [JsonProperty("created_at")] private string _createdAt = "";

    [JsonIgnore] public bool IsDefault => Id == 1;

    [JsonIgnore]
    public string ImpactText => AllowImpactModules ? "impact: permitido" : "impact: bloqueado";

    [JsonIgnore]
    public string Display => $"#{Id} {Name}{(AllowImpactModules ? "  [impact]" : "")}";

    public override string ToString() => Display;

    partial void OnNameChanged(string value) => RaiseDisplay();
    partial void OnAllowImpactModulesChanged(bool value) => RaiseDisplay();

    private void RaiseDisplay()
    {
        OnPropertyChanged(nameof(Display));
        OnPropertyChanged(nameof(ImpactText));
    }
}

/// <summary>handleListTenants responde {"tenants": [...]}. El envoltorio puede venir null.</summary>
public sealed class TenantEnvelope
{
    [JsonProperty("tenants")]
    public System.Collections.Generic.List<Tenant>? Tenants { get; set; }
}
