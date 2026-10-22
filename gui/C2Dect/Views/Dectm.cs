using System;
using System.Collections.Generic;
using System.Globalization;
using System.Text;
using System.Text.RegularExpressions;

namespace C2Dect.Views;

/// <summary>
/// Transpilador de manifiestos .dectm a Lua (subset de la edicion Lite).
///
/// El modulo comercial usa tools/dls/dls.py (Python). La edicion Lite es Go +
/// WPF, asi que el subconjunto que se soporta aqui vive en la consola:
///
///   name, version, scope, mitre, reruns, steps[]
///   ops: log, task, sleep, beacon, hash, crypto.sha256
///
/// El Lua generado solo usa la superficie c2.* del runtime Lite, asi que un
/// manifiesto no puede escalonar a capacidades que el agente no expone.
/// El script_id se calcula sobre el Lua emitido, igual que en el Pro.
/// </summary>
public static class Dectm
{
    public class Result
    {
        public bool Ok { get; set; }
        public string Error { get; set; } = "";
        public string Lua { get; set; } = "";
        public string Name { get; set; } = "";
        public string Version { get; set; } = "";
        public string Scope { get; set; } = "";
        public List<string> Mitre { get; set; } = new();
        public int Steps { get; set; }
    }

    /// <summary>Transpila un manifiesto .dectm a Lua.</summary>
    public static Result Transpile(string manifest)
    {
        var result = new Result();
        if (string.IsNullOrWhiteSpace(manifest))
        {
            result.Error = "manifiesto vacio";
            return result;
        }

        var lines = manifest.Replace("\r\n", "\n").Split('\n');
        string name = "", version = "", scope = "", mitreRaw = "";
        int reruns = 1;
        var steps = new List<(string op, string arg)>();
        bool inSteps = false;

        foreach (var rawLine in lines)
        {
            var line = rawLine.TrimEnd();
            if (string.IsNullOrWhiteSpace(line)) continue;
            if (line.TrimStart().StartsWith("#")) continue;

            if (!inSteps)
            {
                if (line.Trim() == "steps:")
                {
                    inSteps = true;
                    continue;
                }
                var idx = line.IndexOf(':');
                if (idx <= 0) continue;
                var key = line.Substring(0, idx).Trim();
                var value = line.Substring(idx + 1).Trim();
                switch (key)
                {
                    case "name": name = Unquote(value); break;
                    case "version": version = Unquote(value); break;
                    case "scope": scope = Unquote(value); break;
                    case "mitre": mitreRaw = Unquote(value); break;
                    case "reruns":
                        if (!int.TryParse(value, out reruns)) reruns = 1;
                        break;
                    case "transpiler_min":
                    case "defaults":
                    case "sign":
                        break;
                    default:
                        result.Error = $"campo desconocido: {key}";
                        return result;
                }
                continue;
            }

            var step = line.Trim();
            if (!step.StartsWith("-")) continue;
            step = step.Substring(1).Trim();
            if (step.StartsWith("{")) step = step.Substring(1);
            if (step.EndsWith("}")) step = step.Substring(0, step.Length - 1);

            // {name: contexto, beacon: true}
            foreach (Match m in Regex.Matches(step, @"(\w+)\s*:\s*([^,]+)"))
            {
                var op = m.Groups[1].Value.Trim();
                var arg = m.Groups[2].Value.Trim();
                if (op == "name") continue;
                steps.Add((op, Unquote(arg)));
            }
        }

        if (string.IsNullOrWhiteSpace(name))
        {
            result.Error = "falta 'name' en el manifiesto";
            return result;
        }
        if (!Regex.IsMatch(name, @"^[A-Za-z_][A-Za-z0-9_-]*$"))
        {
            result.Error = "name invalido (solo letras, digitos, - y _)";
            return result;
        }
        if (steps.Count == 0)
        {
            result.Error = "el manifiesto no tiene pasos";
            return result;
        }
        if (steps.Count > 64)
        {
            result.Error = "demasiados pasos (max 64)";
            return result;
        }
        if (scope == "") scope = "lab-only";
        if (scope.Length > 64)
        {
            result.Error = "scope demasiado largo (max 64)";
            return result;
        }
        if (reruns < 1 || reruns > 1000) reruns = 1;

        var mitre = new List<string>();
        if (!string.IsNullOrWhiteSpace(mitreRaw))
        {
            foreach (var m in mitreRaw.Trim('[', ']').Split(','))
            {
                var t = m.Trim().Trim('\'', '"');
                if (t.Length > 0) mitre.Add(t);
            }
        }

        // --- emision del Lua ---
        var sb = new StringBuilder();
        sb.AppendLine($"-- generated by dectm-lite v1 from {name}.dectm (scope: {scope})");
        sb.AppendLine("-- DO NOT EDIT: edita el .dectm y recompila.");
        sb.AppendLine($"c2.log(\"INFO\", \"script {name} {version} - {steps.Count} paso(s)\")");
        sb.AppendLine($"c2.log(\"INFO\", \"mitre: {(mitre.Count > 0 ? string.Join(",", mitre) : "-")}\")");

        int n = 0;
        foreach (var (op, arg) in steps)
        {
            n++;
            switch (op)
            {
                case "log":
                    sb.AppendLine($"c2.log(\"INFO\", {LuaString(arg)})");
                    break;
                case "task":
                    sb.AppendLine($"c2.log(\"INFO\", {LuaString($"[step {n}] " + arg)})");
                    sb.AppendLine($"c2.task({LuaString(arg)})");
                    break;
                case "sleep":
                    sb.AppendLine($"c2.sleep({NormalizeMillis(arg)})");
                    break;
                case "beacon":
                    sb.AppendLine("c2.log(\"INFO\", c2.session_id())");
                    break;
                case "hash":
                    sb.AppendLine($"c2.log(\"INFO\", c2.crypto.sha256({LuaString(arg)}))");
                    break;
                case "crypto.sha256":
                    sb.AppendLine($"c2.log(\"INFO\", c2.crypto.sha256({LuaString(arg)}))");
                    break;
                default:
                    result.Error = $"operacion no soportada en la edicion Lite: {op}";
                    return result;
            }
        }
        sb.AppendLine($"return \"{name} ok\"");

        result.Ok = true;
        result.Lua = sb.ToString();
        result.Name = name;
        result.Version = version;
        result.Scope = scope;
        result.Mitre = mitre;
        result.Steps = steps.Count;
        return result;
    }

    private static string Unquote(string raw)
    {
        var v = raw.Trim();
        if (v.Length >= 2 && ((v[0] == '"' && v[v.Length - 1] == '"') || (v[0] == '\'' && v[v.Length - 1] == '\'')))
            return v.Substring(1, v.Length - 2);
        return v;
    }

    private static string LuaString(string raw)
    {
        var escaped = raw.Replace("\\", "\\\\").Replace("\"", "\\\"");
        return "\"" + escaped + "\"";
    }

    /// <summary>Normaliza el valor de sleep a milisegundos.</summary>
    private static string NormalizeMillis(string raw)
    {
        var v = Unquote(raw);
        if (int.TryParse(v, NumberStyles.Integer, CultureInfo.InvariantCulture, out var ms))
            return ms.ToString(CultureInfo.InvariantCulture);
        if (double.TryParse(v, NumberStyles.Float, CultureInfo.InvariantCulture, out var seconds))
            return ((int)(seconds * 1000)).ToString(CultureInfo.InvariantCulture);
        return "0";
    }
}
