using System;
using System.Collections.Generic;
using System.Linq;

namespace C2Dect.Models;

// ---------------------------------------------------------------------------
// Catalogo de modulos servido por el servidor: GET /api/modules
// (server/api/module_catalog.go).
//
// POR QUE EXISTE. La GUI no codifica un formulario por modulo: consume este
// catalogo y construye la ficha y los parametros a partir de los datos. Anadir
// un modulo al servidor (descriptor + ruta + wrapper) lo hace aparecer en la
// GUI sin tocar una linea de C# ni de XAML.
//
// El catalogo local de la GUI (ver OffensiveModulesViewModel) es solo el
// fallback para servidores antiguos o sin catalogo: el del servidor manda.
// ---------------------------------------------------------------------------

/// <summary>Capacidad concreta de un modulo, tal como la declara el servidor.</summary>
public sealed class ModuleCapabilityDto
{
    [Newtonsoft.Json.JsonProperty("name")] public string? Name { get; set; }
    [Newtonsoft.Json.JsonProperty("detail")] public string? Detail { get; set; }
    /// <summary>real | partial | stub | info.</summary>
    [Newtonsoft.Json.JsonProperty("grade")] public string? Grade { get; set; }
    [Newtonsoft.Json.JsonProperty("trigger")] public string? Trigger { get; set; }
}

/// <summary>Parametro de un modulo, con su editor y su validacion.</summary>
public sealed class ModuleParameterDto
{
    [Newtonsoft.Json.JsonProperty("key")] public string? Key { get; set; }
    /// <summary>Posicion de lectura en la GUI: la decide el catalogo.</summary>
    [Newtonsoft.Json.JsonProperty("order")] public int Order { get; set; }
    [Newtonsoft.Json.JsonProperty("label")] public string? Label { get; set; }
    [Newtonsoft.Json.JsonProperty("hint")] public string? Hint { get; set; }
    [Newtonsoft.Json.JsonProperty("placeholder")] public string? Placeholder { get; set; }
    [Newtonsoft.Json.JsonProperty("default")] public string? Default { get; set; }
    /// <summary>text | number | boolean | choice.</summary>
    [Newtonsoft.Json.JsonProperty("kind")] public string? Kind { get; set; }
    [Newtonsoft.Json.JsonProperty("required")] public bool Required { get; set; }
    [Newtonsoft.Json.JsonProperty("options")] public List<string>? Options { get; set; }
    [Newtonsoft.Json.JsonProperty("min")] public double? Min { get; set; }
    [Newtonsoft.Json.JsonProperty("max")] public double? Max { get; set; }
}

/// <summary>Preset aplicable de un clic.</summary>
public sealed class ModulePresetDto
{
    [Newtonsoft.Json.JsonProperty("name")] public string? Name { get; set; }
    [Newtonsoft.Json.JsonProperty("description")] public string? Description { get; set; }
    [Newtonsoft.Json.JsonProperty("values")] public Dictionary<string, string>? Values { get; set; }
}

/// <summary>Ficha completa de un modulo.</summary>
public sealed class ModuleDescriptorDto
{
    [Newtonsoft.Json.JsonProperty("key")] public string? Key { get; set; }
    /// <summary>Posicion de lectura en la GUI: la decide el catalogo.</summary>
    [Newtonsoft.Json.JsonProperty("order")] public int Order { get; set; }
    [Newtonsoft.Json.JsonProperty("title")] public string? Title { get; set; }
    [Newtonsoft.Json.JsonProperty("category")] public string? Category { get; set; }
    [Newtonsoft.Json.JsonProperty("grade")] public string? Grade { get; set; }
    [Newtonsoft.Json.JsonProperty("summary")] public string? Summary { get; set; }
    [Newtonsoft.Json.JsonProperty("what_it_does")] public string? WhatItDoes { get; set; }
    [Newtonsoft.Json.JsonProperty("what_it_does_not")] public string? WhatItDoesNot { get; set; }
    [Newtonsoft.Json.JsonProperty("warning")] public string? Warning { get; set; }
    [Newtonsoft.Json.JsonProperty("notice")] public string? Notice { get; set; }
    [Newtonsoft.Json.JsonProperty("host_notice")] public string? HostNotice { get; set; }
    [Newtonsoft.Json.JsonProperty("network_notice")] public string? NetworkNotice { get; set; }
    [Newtonsoft.Json.JsonProperty("output_hint")] public string? OutputHint { get; set; }
    [Newtonsoft.Json.JsonProperty("technique")] public string? Technique { get; set; }
    [Newtonsoft.Json.JsonProperty("endpoint")] public string? Endpoint { get; set; }
    /// <summary>low | medium | high | critical.</summary>
    [Newtonsoft.Json.JsonProperty("risk")] public string? Risk { get; set; }
    [Newtonsoft.Json.JsonProperty("host_execution")] public bool HostExecution { get; set; }
    [Newtonsoft.Json.JsonProperty("network")] public bool Network { get; set; }
    [Newtonsoft.Json.JsonProperty("requires_consent")] public bool RequiresConsent { get; set; }
    /// <summary>Como se ejecuta: http-module (funcion Go) | cli (proceso externo).</summary>
    [Newtonsoft.Json.JsonProperty("kind")] public string? Kind { get; set; }
    /// <summary>Si la IA puede proponer este modulo en un plan.</summary>
    [Newtonsoft.Json.JsonProperty("orquestable")] public bool Orquestable { get; set; }
    /// <summary>Si el modulo tiene modo en seco.</summary>
    [Newtonsoft.Json.JsonProperty("dry_run")] public bool DryRun { get; set; }
    /// <summary>Si las acciones destructivas exigen confirmacion explicita.</summary>
    [Newtonsoft.Json.JsonProperty("requires_confirm")] public bool RequiresConfirm { get; set; }
    /// <summary>Si los efectos se pueden deshacer.</summary>
    [Newtonsoft.Json.JsonProperty("reversible")] public bool Reversible { get; set; }
    /// <summary>Condiciones verificables que el operador tiene que cumplir.</summary>
    [Newtonsoft.Json.JsonProperty("requires")] public List<string>? Requires { get; set; }
    /// <summary>Por que la IA no puede proponer este modulo.</summary>
    [Newtonsoft.Json.JsonProperty("not_orchestrable_reason")] public string? NotOrchestrableReason { get; set; }
    /// <summary>Comandos concretos del agente que declara esta capacidad.</summary>
    [Newtonsoft.Json.JsonProperty("agent_commands")] public List<string>? AgentCommands { get; set; }
    [Newtonsoft.Json.JsonProperty("preconditions")] public List<string>? Preconditions { get; set; }
    [Newtonsoft.Json.JsonProperty("artifacts")] public List<string>? Artifacts { get; set; }
    [Newtonsoft.Json.JsonProperty("validation")] public List<string>? Validation { get; set; }
    [Newtonsoft.Json.JsonProperty("execution")] public string? Execution { get; set; }
    [Newtonsoft.Json.JsonProperty("capabilities")] public List<ModuleCapabilityDto>? Capabilities { get; set; }
    [Newtonsoft.Json.JsonProperty("parameters")] public List<ModuleParameterDto>? Parameters { get; set; }
    [Newtonsoft.Json.JsonProperty("presets")] public List<ModulePresetDto>? Presets { get; set; }
}

/// <summary>Respuesta de GET /api/modules.</summary>
public sealed class ModuleCatalogResponse
{
    [Newtonsoft.Json.JsonProperty("count")] public int Count { get; set; }
    [Newtonsoft.Json.JsonProperty("modules")] public List<ModuleDescriptorDto>? Modules { get; set; }
    [Newtonsoft.Json.JsonProperty("note")] public string? Note { get; set; }

    /// <summary>
    /// Traduce el catalogo del servidor a las fichas que consume la vista.
    /// Es el UNICO punto donde se mapea el formato del servidor al modelo
    /// interno: anadir un campo al catalogo es tocar solo esta funcion.
    /// </summary>
    public IReadOnlyList<ModuleSpec> ToSpecs()
    {
        if (Modules is null) return Array.Empty<ModuleSpec>();
        return Modules
            .Where(m => !string.IsNullOrWhiteSpace(m.Key))
            .Select(m => m.ToSpec())
            .ToList();
    }
}

/// <summary>Traduce un descriptor del servidor a la ficha interna de la GUI.</summary>
public static class ModuleDescriptorMapper
{
    public static ModuleSpec ToSpec(this ModuleDescriptorDto d)
    {
        return new ModuleSpec
        {
            Key = d.Key ?? string.Empty,
            Order = d.Order,
            Title = string.IsNullOrWhiteSpace(d.Title) ? d.Key ?? string.Empty : d.Title!,
            Category = d.Category ?? string.Empty,
            Grade = d.Grade ?? string.Empty,
            Summary = d.Summary ?? string.Empty,
            WhatItDoes = d.WhatItDoes ?? string.Empty,
            WhatItDoesNot = d.WhatItDoesNot ?? string.Empty,
            Warning = d.Warning ?? string.Empty,
            Notice = d.Notice ?? string.Empty,
            HostNotice = d.HostNotice ?? string.Empty,
            NetworkNotice = d.NetworkNotice ?? string.Empty,
            OutputHint = d.OutputHint ?? string.Empty,
            Technique = d.Technique ?? string.Empty,
            Endpoint = d.Endpoint ?? string.Empty,
            Risk = RiskOf(d.Risk),
            HostExecution = d.HostExecution,
            Network = d.Network,
            RequiresConsent = d.RequiresConsent,
            Kind = d.Kind ?? string.Empty,
            Orquestable = d.Orquestable,
            DryRun = d.DryRun,
            RequiresConfirm = d.RequiresConfirm,
            Reversible = d.Reversible,
            Requires = d.Requires?.ToList() ?? new List<string>(),
            NotOrchestrableReason = d.NotOrchestrableReason ?? string.Empty,
            AgentCommands = d.AgentCommands?.ToList() ?? new List<string>(),
            Preconditions = d.Preconditions?.ToList() ?? new List<string>(),
            Artifacts = d.Artifacts?.ToList() ?? new List<string>(),
            Validation = d.Validation?.ToList() ?? new List<string>(),
            Execution = d.Execution ?? string.Empty,
            Capabilities = (d.Capabilities ?? new List<ModuleCapabilityDto>())
                .Select(c => new ModuleCapability
                {
                    Name = c.Name ?? string.Empty,
                    Detail = c.Detail ?? string.Empty,
                    Trigger = c.Trigger ?? string.Empty,
                    Grade = GradeOf(c.Grade),
                })
                .ToList(),
            Parameters = (d.Parameters ?? new List<ModuleParameterDto>())
                .Select(p => new ModuleParameterSpec
                {
                    Key = p.Key ?? string.Empty,
                    Label = string.IsNullOrWhiteSpace(p.Label) ? p.Key ?? string.Empty : p.Label!,
                    Hint = p.Hint ?? string.Empty,
                    Placeholder = p.Placeholder ?? string.Empty,
                    DefaultValue = p.Default ?? string.Empty,
                    Kind = KindOf(p.Kind),
                    Required = p.Required,
                    Options = p.Options?.ToList() ?? new List<string>(),
                    Min = p.Min,
                    Max = p.Max,
                })
                .ToList(),
            Presets = (d.Presets ?? new List<ModulePresetDto>())
                .Select(p => new ModulePreset
                {
                    Name = p.Name ?? string.Empty,
                    Description = p.Description ?? string.Empty,
                    Values = p.Values ?? new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase),
                })
                .ToList(),
        };
    }

    private static ModuleRisk RiskOf(string? raw) => (raw ?? "").Trim().ToLowerInvariant() switch
    {
        "low" => ModuleRisk.Low,
        "medium" => ModuleRisk.Medium,
        "high" => ModuleRisk.High,
        "critical" => ModuleRisk.Critical,
        _ => ModuleRisk.Low,
    };

    /// <summary>
    /// Grado de una capacidad. Solo dos valores: "real" ejecuta la acción,
    /// "simulado" no la ejecuta: solo describe. Cualquier otro valor se
    /// trata como "simulado" (que es lo prudente).
    /// </summary>
    private static ModuleCapabilityGrade GradeOf(string? raw) => (raw ?? "").Trim().ToLowerInvariant() switch
    {
        "real" => ModuleCapabilityGrade.Real,
        _ => ModuleCapabilityGrade.Simulado,
    };

    private static ModuleParamKind KindOf(string? raw) => (raw ?? "").Trim().ToLowerInvariant() switch
    {
        "number" => ModuleParamKind.Number,
        "boolean" => ModuleParamKind.Boolean,
        "choice" => ModuleParamKind.Choice,
        _ => ModuleParamKind.Text,
    };
}
