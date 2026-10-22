using Newtonsoft.Json;
using System.Collections.Generic;

namespace C2Dect.Models;

/// <summary>db.Category (server/db/database.go:1169-1174).</summary>
public sealed class Category
{
    [JsonProperty("id")] public long Id { get; set; }
    [JsonProperty("tenant_id")] public long TenantId { get; set; }
    [JsonProperty("name")] public string? Name { get; set; }
    [JsonProperty("created_at")] public string? CreatedAt { get; set; }
}

/// <summary>db.Permission (server/db/database.go:1176-1182).</summary>
public sealed class Permission
{
    [JsonProperty("id")] public long Id { get; set; }
    [JsonProperty("category_id")] public long CategoryId { get; set; }
    [JsonProperty("key")] public string? Key { get; set; }
    [JsonProperty("description")] public string? Description { get; set; }
    [JsonProperty("category")] public string? Category { get; set; }

    /// <summary>Etiqueta legible para las listas de permisos (rol y tenant).</summary>
    [JsonIgnore] public string Display => $"{Category}:{Key} - {Description}";
}

/// <summary>db.Role (server/db/database.go:1184-1191).</summary>
public sealed class Role
{
    [JsonProperty("id")] public long Id { get; set; }
    [JsonProperty("tenant_id")] public long TenantId { get; set; }
    [JsonProperty("name")] public string? Name { get; set; }
    [JsonProperty("view")] public string? View { get; set; }
    [JsonProperty("is_system")] public bool IsSystem { get; set; }
    [JsonProperty("created_at")] public string? CreatedAt { get; set; }
}

public sealed class CategoriesEnvelope
{
    [JsonProperty("categories")] public List<Category>? Categories { get; set; }
}

public sealed class PermissionsEnvelope
{
    [JsonProperty("permissions")] public List<Permission>? Permissions { get; set; }
}

public sealed class RolesEnvelope
{
    [JsonProperty("roles")] public List<Role>? Roles { get; set; }
}

public sealed class UsersEnvelope
{
    [JsonProperty("users")] public List<Models.Operator>? Users { get; set; }
}

/// <summary>PUT /api/tenants/:id/impact responde {"status","tenant_id","allow_impact_modules"}.</summary>
public sealed class TenantImpactResult
{
    [JsonProperty("status")] public string? Status { get; set; }
    [JsonProperty("tenant_id")] public long TenantId { get; set; }
    [JsonProperty("allow_impact_modules")] public bool AllowImpactModules { get; set; }
}
