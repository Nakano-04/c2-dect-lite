using System;
using System.Collections.Generic;

namespace C2Dect.Models;

// ---------------------------------------------------------------------------
// Contrato del "Module Studio": fichas profesionales de cada módulo
// cableado al C2 (GET /api/modules/:module).
//
// REGLA DE HONESTIDAD (heredada de server/api/offensive_modules.go):
// una ficha declara que GRADO tiene cada capacidad, que RIESGO tiene para el
// operador y que NO hace el módulo. Nada se presenta como capacidad real si
// no lo es.
// ---------------------------------------------------------------------------

/// <summary>Riesgo que supone ejecutar el módulo en el host del servidor C2.</summary>
public enum ModuleRisk
{
    /// <summary>Solo lectura o emulacion sin salida a la red.</summary>
    Low = 0,
    /// <summary>Ejecucion local acotada (ficheros, procesos) sin salir a la red.</summary>
    Medium = 1,
    /// <summary>Salida a la red, escritura en disco o acciones sobre terceros.</summary>
    High = 2,
    /// <summary>Capacidad destructiva o de impacto (DoS, ransomware, despliegue).</summary>
    Critical = 3,
}

/// <summary>Tipo de editor que usa un parámetro.</summary>
public enum ModuleParamKind
{
    /// <summary>Texto libre.</summary>
    Text = 0,
    /// <summary>Número con rango opcional.</summary>
    Number = 1,
    /// <summary>Sí/No.</summary>
    Boolean = 2,
    /// <summary>Lista cerrada de valores.</summary>
    Choice = 3,
}

/// <summary>
/// Grado de una capacidad concreta del módulo. DOS VALORES, y solo dos.
///
///   real -> la capacidad ejecuta la acción de verdad.
///   simulado -> la capacidad NO ejecuta la acción: describe lo que haría.
///
/// No hay grados intermedios: un módulo que hace una parte y describe otra
/// declara una capacidad "real" y una "simulado", y la ficha lo dice capability por
/// capability. La palabra "simulado" es la que ve el operador: un módulo que se
/// presenta como real y no lo es es el fallo que hay que hacer imposible.
/// </summary>
public enum ModuleCapabilityGrade
{
    /// <summary>Ejecuta la acción de verdad.</summary>
    Real = 0,
    /// <summary>No ejecuta la acción: solo devuelve texto o descripción.</summary>
    Simulado = 1,
}

/// <summary>Una capacidad concreta del módulo, con su grado.</summary>
public sealed class ModuleCapability
{
    /// <summary>Nombre de la función o capacidad.</summary>
    public string Name { get; set; } = string.Empty;

    /// <summary>Qué hace exactamente, en una frase.</summary>
    public string Detail { get; set; } = string.Empty;

    /// <summary>Grado declarado: real, partial, stub o info.</summary>
    public ModuleCapabilityGrade Grade { get; set; } = ModuleCapabilityGrade.Simulado;

    /// <summary>Comando o query param que la activa (opcional).</summary>
    public string Trigger { get; set; } = string.Empty;
}

/// <summary>Definición de un parámetro con su editor y su validación.</summary>
public sealed class ModuleParameterSpec
{
    /// <summary>Nombre de la clave en el query string.</summary>
    public string Key { get; set; } = string.Empty;

    /// <summary>Etiqueta visible en la GUI.</summary>
    public string Label { get; set; } = string.Empty;

    /// <summary>Explicación de qué hace y qué se espera.</summary>
    public string Hint { get; set; } = string.Empty;

    /// <summary>Texto de ejemplo que se ve en el campo vacío.</summary>
    public string Placeholder { get; set; } = string.Empty;

    /// <summary>Valor por defecto que se precarga.</summary>
    public string DefaultValue { get; set; } = string.Empty;

    /// <summary>Tipo de editor.</summary>
    public ModuleParamKind Kind { get; set; } = ModuleParamKind.Text;

    /// <summary>Si el módulo no puede ejecutarse sin este parámetro.</summary>
    public bool Required { get; set; }

    /// <summary>Valores admitidos cuando Kind es Choice.</summary>
    public IReadOnlyList<string> Options { get; set; } = Array.Empty<string>();

    /// <summary>Mínimo numérico (Kind = Number).</summary>
    public double? Min { get; set; }

    /// <summary>Máximo numérico (Kind = Number).</summary>
    public double? Max { get; set; }
}

/// <summary>Conjunto de valores predefinidos que se pueden aplicar de un clic.</summary>
public sealed class ModulePreset
{
    /// <summary>Nombre del preset.</summary>
    public string Name { get; set; } = string.Empty;

    /// <summary>Qué hace este preset y cuándo usarlo.</summary>
    public string Description { get; set; } = string.Empty;

    /// <summary>Valores por clave de parámetro.</summary>
    public IReadOnlyDictionary<string, string> Values { get; set; } =
        new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase);
}

/// <summary>
/// Ficha completa de un módulo: grado, riesgo, qué hace, qué NO hace,
/// parámetros tipados, presets y qué esperar en la salida.
/// </summary>
public sealed class ModuleSpec
{
    /// <summary>Clave del módulo (segmento de la URL).</summary>
    public string Key { get; set; } = string.Empty;

    /// <summary>Posicion de lectura en la GUI: la decide el catalogo.</summary>
    public int Order { get; set; }

    /// <summary>Nombre para mostrar.</summary>
    public string Title { get; set; } = string.Empty;

    /// <summary>Categoría funcional (recon, credenciales, validación, impacto...).</summary>
    public string Category { get; set; } = string.Empty;

    /// <summary>Grado global declarado por el servidor (texto literal).</summary>
    public string Grade { get; set; } = string.Empty;

    /// <summary>Una línea: qué es este módulo.</summary>
    public string Summary { get; set; } = string.Empty;

    /// <summary>Qué hace, en un párrafo honesto.</summary>
    public string WhatItDoes { get; set; } = string.Empty;

    /// <summary>Qué NO hace. Nunca vacío si el módulo tiene límites.</summary>
    public string WhatItDoesNot { get; set; } = string.Empty;

    /// <summary>Warning declarado por el servidor (texto literal).</summary>
    public string Warning { get; set; } = string.Empty;

    /// <summary>Explicación operator-facing del módulo.</summary>
    public string Notice { get; set; } = string.Empty;

    /// <summary>Aviso extra: el módulo ejecuta binarios en el host del C2.</summary>
    public string HostNotice { get; set; } = string.Empty;

    /// <summary>Aviso extra: el módulo sale a la red desde el host del C2.</summary>
    public string NetworkNotice { get; set; } = string.Empty;

    /// <summary>Qué esperar en la salida (interpretación de resultados).</summary>
    public string OutputHint { get; set; } = string.Empty;

    /// <summary>Técnica MITRE ATT&amp;CK asociada, si la hay.</summary>
    public string Technique { get; set; } = string.Empty;

    /// <summary>Endpoint completo que se va a llamar.</summary>
    public string Endpoint { get; set; } = string.Empty;

    /// <summary>Riesgo para el operador.</summary>
    public ModuleRisk Risk { get; set; } = ModuleRisk.Low;

    /// <summary>El módulo ejecuta binarios en el host del servidor C2.</summary>
    public bool HostExecution { get; set; }

    /// <summary>El módulo sale a la red desde el host del servidor C2.</summary>
    public bool Network { get; set; }

    /// <summary>El módulo exige que el operador confirme la autorización del objetivo.</summary>
    public bool RequiresConsent { get; set; }

    /// <summary>
    /// Como se ejecuta el módulo: "http-module" (función Go dentro del
    /// servidor), "cli" (proceso externo de modules/) o "agent-command" /
    /// "native-binary" (capacidades declaradas, que se ejecutan en la
    /// víctima o como binario, no desde este panel).
    /// </summary>
    public string Kind { get; set; } = string.Empty;

    /// <summary>
    /// Capacidad DECLARADA: existe y es real, pero no se ejecuta desde aquí.
    ///
    /// Los módulos del plano del agente (comandos del implante) y los binarios
    /// nativos se declaran para que el inventario sea completo y auditable,
    /// pero su ejecución la decide una persona: no hay endpoint de módulo, no
    /// hay allowlist de comandos, ni dry-run, ni confirmación, ni evidencia.
    /// </summary>
    public bool IsDeclaredOnly =>
        !string.IsNullOrWhiteSpace(Kind)
        && !string.Equals(Kind, "http-module", StringComparison.OrdinalIgnoreCase)
        && !string.Equals(Kind, "cli", StringComparison.OrdinalIgnoreCase);

    /// <summary>
    /// Por qué la IA no puede proponer este módulo. Vacio en los orquestables.
    /// </summary>
    public string NotOrchestrableReason { get; set; } = string.Empty;

    /// <summary>
    /// Comandos concretos del agente que declara esta capacidad (solo en las
    /// fichas del plano del agente).
    ///
    /// Es la allowlist de la puerta del plano del agente: lo que no está aquí
    /// no se encola. Vive en el catálogo, no en el código del gate, para que
    /// la lista de capacidades y la de comandos ejecutables no diverjan.
    /// </summary>
    public IReadOnlyList<string> AgentCommands { get; set; } = Array.Empty<string>();

    /// <summary>
    /// Si la IA puede proponer este módulo en un plan. Un módulo visible pero
    /// no orquestable está DECLARADO, no prohibido: el copiloto lo ve y puede
    /// responder por él, pero el orquestador no lo ejecuta.
    /// </summary>
    public bool Orquestable { get; set; }

    /// <summary>Si el módulo tiene modo en seco (listar sin ejecutar).</summary>
    public bool DryRun { get; set; }

    /// <summary>
    /// Si las acciones destructivas exigen una confirmación explícita del
    /// operador en el momento de ejecutarlas.
    /// </summary>
    public bool RequiresConfirm { get; set; }

    /// <summary>Si los efectos de la ejecución se pueden deshacer.</summary>
    public bool Reversible { get; set; }

    /// <summary>
    /// Condiciones verificables que el operador tiene que cumplir para que
    /// el módulo pueda ejecutarse (marcadores, custodia, política firmada).
    /// </summary>
    public IReadOnlyList<string> Requires { get; set; } = Array.Empty<string>();

    /// <summary>Lo que el operador tiene que tener listo antes de ejecutar.</summary>
    public IReadOnlyList<string> Preconditions { get; set; } = Array.Empty<string>();

    /// <summary>Lo que la ejecucion deja en disco.</summary>
    public IReadOnlyList<string> Artifacts { get; set; } = Array.Empty<string>();

    /// <summary>Como se comprueba que la ejecucion sirvio.</summary>
    public IReadOnlyList<string> Validation { get; set; } = Array.Empty<string>();

    /// <summary>Donde se ejecuta y con que alcance.</summary>
    public string Execution { get; set; } = string.Empty;

    /// <summary>Capacidades concretas con su grado.</summary>
    public IReadOnlyList<ModuleCapability> Capabilities { get; set; } = Array.Empty<ModuleCapability>();

    /// <summary>Parámetros tipados del módulo.</summary>
    public IReadOnlyList<ModuleParameterSpec> Parameters { get; set; } = Array.Empty<ModuleParameterSpec>();

    /// <summary>Presets aplicables de un clic.</summary>
    public IReadOnlyList<ModulePreset> Presets { get; set; } = Array.Empty<ModulePreset>();
}
