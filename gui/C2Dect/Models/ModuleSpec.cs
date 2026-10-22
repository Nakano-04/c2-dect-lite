using System;
using System.Collections.Generic;

namespace C2Dect.Models;

// ---------------------------------------------------------------------------
// Contrato del "Module Studio": fichas profesionales de cada mÃ³dulo
// cableado al C2 (GET /api/modules/:module).
//
// REGLA DE HONESTIDAD (heredada de server/api/offensive_modules.go):
// una ficha declara que GRADO tiene cada capacidad, que RIESGO tiene para el
// operador y que NO hace el mÃ³dulo. Nada se presenta como capacidad real si
// no lo es.
// ---------------------------------------------------------------------------

/// <summary>Riesgo que supone ejecutar el mÃ³dulo en el host del servidor C2.</summary>
public enum ModuleRisk
{
    /// <summary>Solo lectura o emulacion sin salida a la red.</summary>
    Low = 0,
    /// <summary>Ejecucion local acotada (ficheros, procesos) sin salir a la red.</summary>
    Medium = 1,
    /// <summary>Salida a la red, escritura en disco o acciones sobre terceros.</summary>
    High = 2,
    /// <summary>Capacidad destructiva o de impacto sobre el host.</summary>
    Critical = 3,
}

/// <summary>Tipo de editor que usa un parÃ¡metro.</summary>
public enum ModuleParamKind
{
    /// <summary>Texto libre.</summary>
    Text = 0,
    /// <summary>NÃºmero con rango opcional.</summary>
    Number = 1,
    /// <summary>SÃ­/No.</summary>
    Boolean = 2,
    /// <summary>Lista cerrada de valores.</summary>
    Choice = 3,
}

/// <summary>
/// Grado de una capacidad concreta del mÃ³dulo. DOS VALORES, y solo dos.
///
///   real -> la capacidad ejecuta la acciÃ³n de verdad.
///   simulado -> la capacidad NO ejecuta la acciÃ³n: describe lo que harÃ­a.
///
/// No hay grados intermedios: un mÃ³dulo que hace una parte y describe otra
/// declara una capacidad "real" y una "simulado", y la ficha lo dice capability por
/// capability. La palabra "simulado" es la que ve el operador: un mÃ³dulo que se
/// presenta como real y no lo es es el fallo que hay que hacer imposible.
/// </summary>
public enum ModuleCapabilityGrade
{
    /// <summary>Ejecuta la acciÃ³n de verdad.</summary>
    Real = 0,
    /// <summary>No ejecuta la acciÃ³n: solo devuelve texto o descripciÃ³n.</summary>
    Simulado = 1,
}

/// <summary>Una capacidad concreta del mÃ³dulo, con su grado.</summary>
public sealed class ModuleCapability
{
    /// <summary>Nombre de la funciÃ³n o capacidad.</summary>
    public string Name { get; set; } = string.Empty;

    /// <summary>QuÃ© hace exactamente, en una frase.</summary>
    public string Detail { get; set; } = string.Empty;

    /// <summary>Grado declarado: real, partial, stub o info.</summary>
    public ModuleCapabilityGrade Grade { get; set; } = ModuleCapabilityGrade.Simulado;

    /// <summary>Comando o query param que la activa (opcional).</summary>
    public string Trigger { get; set; } = string.Empty;
}

/// <summary>DefiniciÃ³n de un parÃ¡metro con su editor y su validaciÃ³n.</summary>
public sealed class ModuleParameterSpec
{
    /// <summary>Nombre de la clave en el query string.</summary>
    public string Key { get; set; } = string.Empty;

    /// <summary>Etiqueta visible en la GUI.</summary>
    public string Label { get; set; } = string.Empty;

    /// <summary>ExplicaciÃ³n de quÃ© hace y quÃ© se espera.</summary>
    public string Hint { get; set; } = string.Empty;

    /// <summary>Texto de ejemplo que se ve en el campo vacÃ­o.</summary>
    public string Placeholder { get; set; } = string.Empty;

    /// <summary>Valor por defecto que se precarga.</summary>
    public string DefaultValue { get; set; } = string.Empty;

    /// <summary>Tipo de editor.</summary>
    public ModuleParamKind Kind { get; set; } = ModuleParamKind.Text;

    /// <summary>Si el mÃ³dulo no puede ejecutarse sin este parÃ¡metro.</summary>
    public bool Required { get; set; }

    /// <summary>Valores admitidos cuando Kind es Choice.</summary>
    public IReadOnlyList<string> Options { get; set; } = Array.Empty<string>();

    /// <summary>MÃ­nimo numÃ©rico (Kind = Number).</summary>
    public double? Min { get; set; }

    /// <summary>MÃ¡ximo numÃ©rico (Kind = Number).</summary>
    public double? Max { get; set; }
}

/// <summary>Conjunto de valores predefinidos que se pueden aplicar de un clic.</summary>
public sealed class ModulePreset
{
    /// <summary>Nombre del preset.</summary>
    public string Name { get; set; } = string.Empty;

    /// <summary>QuÃ© hace este preset y cuÃ¡ndo usarlo.</summary>
    public string Description { get; set; } = string.Empty;

    /// <summary>Valores por clave de parÃ¡metro.</summary>
    public IReadOnlyDictionary<string, string> Values { get; set; } =
        new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase);
}

/// <summary>
/// Ficha completa de un mÃ³dulo: grado, riesgo, quÃ© hace, quÃ© NO hace,
/// parÃ¡metros tipados, presets y quÃ© esperar en la salida.
/// </summary>
public sealed class ModuleSpec
{
    /// <summary>Clave del mÃ³dulo (segmento de la URL).</summary>
    public string Key { get; set; } = string.Empty;

    /// <summary>Posicion de lectura en la GUI: la decide el catalogo.</summary>
    public int Order { get; set; }

    /// <summary>Nombre para mostrar.</summary>
    public string Title { get; set; } = string.Empty;

    /// <summary>CategorÃ­a funcional (recon, credenciales, validaciÃ³n, impacto...).</summary>
    public string Category { get; set; } = string.Empty;

    /// <summary>Grado global declarado por el servidor (texto literal).</summary>
    public string Grade { get; set; } = string.Empty;

    /// <summary>Una lÃ­nea: quÃ© es este mÃ³dulo.</summary>
    public string Summary { get; set; } = string.Empty;

    /// <summary>QuÃ© hace, en un pÃ¡rrafo honesto.</summary>
    public string WhatItDoes { get; set; } = string.Empty;

    /// <summary>QuÃ© NO hace. Nunca vacÃ­o si el mÃ³dulo tiene lÃ­mites.</summary>
    public string WhatItDoesNot { get; set; } = string.Empty;

    /// <summary>Warning declarado por el servidor (texto literal).</summary>
    public string Warning { get; set; } = string.Empty;

    /// <summary>ExplicaciÃ³n operator-facing del mÃ³dulo.</summary>
    public string Notice { get; set; } = string.Empty;

    /// <summary>Aviso extra: el mÃ³dulo ejecuta binarios en el host del C2.</summary>
    public string HostNotice { get; set; } = string.Empty;

    /// <summary>Aviso extra: el mÃ³dulo sale a la red desde el host del C2.</summary>
    public string NetworkNotice { get; set; } = string.Empty;

    /// <summary>QuÃ© esperar en la salida (interpretaciÃ³n de resultados).</summary>
    public string OutputHint { get; set; } = string.Empty;

    /// <summary>TÃ©cnica MITRE ATT&amp;CK asociada, si la hay.</summary>
    public string Technique { get; set; } = string.Empty;

    /// <summary>Endpoint completo que se va a llamar.</summary>
    public string Endpoint { get; set; } = string.Empty;

    /// <summary>Riesgo para el operador.</summary>
    public ModuleRisk Risk { get; set; } = ModuleRisk.Low;

    /// <summary>El mÃ³dulo ejecuta binarios en el host del servidor C2.</summary>
    public bool HostExecution { get; set; }

    /// <summary>El mÃ³dulo sale a la red desde el host del servidor C2.</summary>
    public bool Network { get; set; }

    /// <summary>El mÃ³dulo exige que el operador confirme la autorizaciÃ³n del objetivo.</summary>
    public bool RequiresConsent { get; set; }

    /// <summary>
    /// Como se ejecuta el mÃ³dulo: "http-module" (funciÃ³n Go dentro del
    /// servidor), "cli" (proceso externo de modules/) o "agent-command" /
    /// "native-binary" (capacidades declaradas, que se ejecutan en la
    /// vÃ­ctima o como binario, no desde este panel).
    /// </summary>
    public string Kind { get; set; } = string.Empty;

    /// <summary>
    /// Capacidad DECLARADA: existe y es real, pero no se ejecuta desde aquÃ­.
    ///
    /// Los mÃ³dulos del plano del agente (comandos del implante) y los binarios
    /// nativos se declaran para que el inventario sea completo y auditable,
    /// pero su ejecuciÃ³n la decide una persona: no hay endpoint de mÃ³dulo, no
    /// hay allowlist de comandos, ni dry-run, ni confirmaciÃ³n, ni evidencia.
    /// </summary>
    public bool IsDeclaredOnly =>
        !string.IsNullOrWhiteSpace(Kind)
        && !string.Equals(Kind, "http-module", StringComparison.OrdinalIgnoreCase)
        && !string.Equals(Kind, "cli", StringComparison.OrdinalIgnoreCase);

    /// <summary>
    /// Por quÃ© la IA no puede proponer este mÃ³dulo. Vacio en los orquestables.
    /// </summary>
    public string NotOrchestrableReason { get; set; } = string.Empty;

    /// <summary>
    /// Comandos concretos del agente que declara esta capacidad (solo en las
    /// fichas del plano del agente).
    ///
    /// Es la allowlist de la puerta del plano del agente: lo que no estÃ¡ aquÃ­
    /// no se encola. Vive en el catÃ¡logo, no en el cÃ³digo del gate, para que
    /// la lista de capacidades y la de comandos ejecutables no diverjan.
    /// </summary>
    public IReadOnlyList<string> AgentCommands { get; set; } = Array.Empty<string>();

    /// <summary>
    /// Si la IA puede proponer este mÃ³dulo en un plan. Un mÃ³dulo visible pero
    /// no orquestable estÃ¡ DECLARADO, no prohibido: el copiloto lo ve y puede
    /// responder por Ã©l, pero el orquestador no lo ejecuta.
    /// </summary>
    public bool Orquestable { get; set; }

    /// <summary>Si el mÃ³dulo tiene modo en seco (listar sin ejecutar).</summary>
    public bool DryRun { get; set; }

    /// <summary>
    /// Si las acciones destructivas exigen una confirmaciÃ³n explÃ­cita del
    /// operador en el momento de ejecutarlas.
    /// </summary>
    public bool RequiresConfirm { get; set; }

    /// <summary>Si los efectos de la ejecuciÃ³n se pueden deshacer.</summary>
    public bool Reversible { get; set; }

    /// <summary>
    /// Condiciones verificables que el operador tiene que cumplir para que
    /// el mÃ³dulo pueda ejecutarse (marcadores, custodia, polÃ­tica firmada).
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

    /// <summary>ParÃ¡metros tipados del mÃ³dulo.</summary>
    public IReadOnlyList<ModuleParameterSpec> Parameters { get; set; } = Array.Empty<ModuleParameterSpec>();

    /// <summary>Presets aplicables de un clic.</summary>
    public IReadOnlyList<ModulePreset> Presets { get; set; } = Array.Empty<ModulePreset>();
}
