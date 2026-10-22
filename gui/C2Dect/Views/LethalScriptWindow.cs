using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;
using C2Dect.Models;
using C2Dect.Services;

namespace C2Dect.Views;

/// <summary>
/// Lethal Script: motor de scripting del agente.
///
/// Es la misma ventana del modulo comercial (editor, snippets, validacion
/// estatica, modo .dectm, firma, envio y debug iterativo) con dos cambios
/// propios de la edicion Lite:
///
///   - la FIRMA la hace el servidor (la clave del marketplace), no un seed
///     local: la consola nunca guarda material de firma en disco;
///   - la superficie c2.* es la minima del runtime Lite: sin shell, sin
///     escritura de ficheros y con el gate de impacto vigente.
///
/// El sandbox real sigue siendo el del agente: la consola solo valida.
public class LethalScriptWindow : Window
{
    private readonly ComboBox _sessionCombo = new ComboBox();
    private readonly TextBox _editor = new TextBox();
    private readonly TextBox _output = new TextBox();
    private readonly ComboBox _snippetCombo = new ComboBox();
    private readonly CheckBox _envelopeCheck = new CheckBox();
    private readonly TextBlock _status = new TextBlock();
    private readonly ComboBox _modeCombo = new ComboBox();
    private readonly TextBlock _sigStateLabel = new TextBlock();
    private volatile bool _pollStop;

    // Estado de la firma devuelta por el servidor.
    private string _scriptId = "";
    private string _signature = "";
    private string _kid = "";
    private string _scope = "lab-only";
    private bool _dirty = true;

    // API c2.* que expone el runtime Lite (allowlist del agente).
    private static readonly string[] KnownApis =
    {
        "c2.log", "c2.sleep", "c2.args", "c2.session_id", "c2.script_id", "c2.beacon",
        "c2.task", "c2.fs", "c2.proc", "c2.sys", "c2.net", "c2.crypto", "print"
    };

    private static readonly string[] Blocked =
    {
        "os.", "io.", "debug.", "require", "loadstring", "dofile", "loadfile",
        "load(", "collectgarbage", "newproxy", "package.", "module(", "c2.exec",
        "c2.kv", "c2.bof", "c2.download", "c2.upload"
    };

    /// <summary>Snippets de Lua. Todos declaran su alcance.</summary>
    private static readonly KeyValuePair<string, string>[] LuaSnippets =
    {
        new KeyValuePair<string, string>("hola + sleep",
            "-- scope: lab-only\nc2.log(\"hola desde \" .. c2.session_id())\nc2.sleep(1)\nreturn \"ok\""),
        new KeyValuePair<string, string>("triage sysinfo",
            "-- scope: lab-only\nlocal s = c2.task(\"sysinfo\")\nc2.log(s)\nreturn s"),
        new KeyValuePair<string, string>("procesos",
            "-- scope: lab-only\nlocal p = c2.proc.list()\nc2.log(p)\nreturn p"),
        new KeyValuePair<string, string>("red",
            "-- scope: lab-only\nlocal n = c2.net.info()\nc2.log(n)\nreturn n"),
        new KeyValuePair<string, string>("listar directorio",
            "-- scope: lab-only\nlocal files = c2.fs.list(\".\")\nfor i, f in ipairs(files) do c2.log(f) end\nreturn #files .. \" entradas\""),
        new KeyValuePair<string, string>("sha256 + random",
            "-- scope: lab-only\nlocal h = c2.crypto.sha256(\"abc\")\nlocal r = c2.crypto.rand(8)\nreturn h"),
        new KeyValuePair<string, string>("fichero de configuracion",
            "-- scope: lab-only\nlocal c = c2.fs.read(\"C:\\\\Windows\\\\System32\\\\drivers\\\\etc\\\\hosts\")\nc2.log(c)\nreturn c"),
    };

    /// <summary>Snippets del manifiesto .dectm (subset Lite).</summary>
    private static readonly KeyValuePair<string, string>[] DectmSnippets =
    {
        new KeyValuePair<string, string>("beacon + sleep",
            "# scope: lab-only\nname: demo-beacon\nversion: 1.0.0\nscope: lab-only\nmitre: [T1082]\nreruns: 1\nsteps:\n  - {name: contexto, beacon: true}\n  - {name: espera, sleep: 5000}"),
        new KeyValuePair<string, string>("log + sysinfo",
            "# scope: lab-only\nname: demo-triage\nversion: 1.0.0\nscope: lab-only\nmitre: [T1082]\nreruns: 1\nsteps:\n  - {name: aviso, log: \"inicio triage\"}\n  - {name: sys, task: sysinfo}"),
        new KeyValuePair<string, string>("hash + red",
            "# scope: lab-only\nname: demo-hash\nversion: 1.0.0\nscope: lab-only\nmitre: [T1016]\nreruns: 1\nsteps:\n  - {name: huella, hash: \"abc\"}\n  - {name: red, task: netinfo}"),
    };

    private KeyValuePair<string, string>[] ActiveSnippets()
        => IsDectmMode() ? DectmSnippets : LuaSnippets;

    private void RefillSnippets()
    {
        try
        {
            _snippetCombo.Items.Clear();
            foreach (var s in ActiveSnippets()) _snippetCombo.Items.Add(s.Key);
            if (_snippetCombo.Items.Count > 0) _snippetCombo.SelectedIndex = 0;
        }
        catch { }
    }

    private string SnippetValue(string key)
    {
        try
        {
            foreach (var s in ActiveSnippets())
                if (s.Key == key) return s.Value;
        }
        catch { }
        return "";
    }

    public LethalScriptWindow()
    {
        Title = "Lethal Script - editor Lua/.dectm + debug";
        Width = 1000;
        Height = 740;
        Background = new SolidColorBrush(Color.FromRgb(0x0c, 0x0e, 0x14));
        FontFamily = new FontFamily("Consolas");
        WindowStartupLocation = WindowStartupLocation.CenterOwner;

        var root = new StackPanel { Margin = new Thickness(16) };
        var title = new TextBlock
        {
            Text = "LETHAL SCRIPT LITE - LUA (agente) - timeout 30s, max 1MiB, sandbox sin shell",
            Foreground = new SolidColorBrush(Color.FromRgb(0x00, 0xe6, 0x76)),
            FontWeight = FontWeights.Bold,
            FontSize = 14,
            Margin = new Thickness(0, 0, 0, 8),
        };
        root.Children.Add(title);

        var bar = new WrapPanel { Margin = new Thickness(0, 0, 0, 8) };
        bar.Children.Add(Lbl("Sesion:"));
        _sessionCombo.Width = 260; _sessionCombo.Height = 28; _sessionCombo.Margin = new Thickness(0, 0, 8, 0);
        bar.Children.Add(_sessionCombo);
        bar.Children.Add(Lbl("Modo:"));
        _modeCombo.Width = 110; _modeCombo.Height = 28; _modeCombo.Margin = new Thickness(0, 0, 8, 0);
        _modeCombo.Items.Add("lua");
        _modeCombo.Items.Add("dectm");
        _modeCombo.SelectedIndex = 0;
        _modeCombo.SelectionChanged += (s, e) => { try { OnModeChanged(); } catch { } };
        bar.Children.Add(_modeCombo);
        bar.Children.Add(MkButtonAsync("REFRESCAR", async () => await LoadSessionsAsync()));
        bar.Children.Add(MkButton("DOCS", () =>
        {
            try
            {
                var docs = new LethalScriptDocsWindow { Owner = this };
                docs.Show();
            }
            catch { }
        }));
        bar.Children.Add(MkButtonAsync("VALIDAR", async () => await ValidateAsync()));
        bar.Children.Add(MkButtonAsync("PUBLICAR Y FIRMAR", async () => await PublishAsync()));
        bar.Children.Add(MkButtonAsync("EJECUTAR", async () => await RunScriptAsync()));
        bar.Children.Add(MkButton("DETENER", () => { _pollStop = true; AppendOut("[*] Poll detenido por operador."); }));
        _envelopeCheck.Content = "envolver la fuente";
        _envelopeCheck.Foreground = Brushes.Gray;
        _envelopeCheck.VerticalAlignment = VerticalAlignment.Center;
        _envelopeCheck.Margin = new Thickness(8, 0, 0, 0);
        _envelopeCheck.ToolTip = "Envia {lang,source,script_id,sig,kid} en vez de la fuente suelta";
        bar.Children.Add(_envelopeCheck);
        root.Children.Add(bar);

        var mid = new WrapPanel { Margin = new Thickness(0, 0, 0, 8) };
        mid.Children.Add(Lbl("Snippet:"));
        RefillSnippets();
        _snippetCombo.Width = 220; _snippetCombo.Height = 28; _snippetCombo.Margin = new Thickness(0, 0, 8, 0);
        _snippetCombo.SelectionChanged += (s, e) =>
        {
            try
            {
                if (_snippetCombo.SelectedItem is string k)
                {
                    string v = SnippetValue(k);
                    if (!string.IsNullOrEmpty(v)) _editor.Text = v;
                }
            }
            catch { }
        };
        mid.Children.Add(_snippetCombo);
        var allow = new TextBlock
        {
            Text = "superficie c2.* del runtime Lite: log, sleep, args, session_id, script_id, beacon, task, fs.list/read/stat, proc.list, sys.info/env, net.info/conns, crypto.*",
            Foreground = new SolidColorBrush(Color.FromRgb(0x8b, 0x94, 0x9e)),
            FontSize = 10,
            TextWrapping = TextWrapping.Wrap,
            VerticalAlignment = VerticalAlignment.Center,
        };
        mid.Children.Add(allow);
        root.Children.Add(mid);

        var keyRow = new WrapPanel { Margin = new Thickness(0, 0, 0, 8) };
        keyRow.Children.Add(Lbl("Alcance:"));
        var scopeBox = new TextBox
        {
            Text = "lab-only",
            Width = 160,
            Height = 28,
            Margin = new Thickness(0, 0, 8, 0),
            Background = new SolidColorBrush(Color.FromRgb(0x0a, 0x0c, 0x12)),
            Foreground = new SolidColorBrush(Color.FromRgb(0xe0, 0xe0, 0xe0)),
            ToolTip = "Alcance del script (cabecera -- scope:). El servidor lo deduce de la fuente.",
        };
        keyRow.Children.Add(scopeBox);
        keyRow.Children.Add(Lbl("La firma la pone el servidor (clave del marketplace): la consola no guarda seeds."));
        _sigStateLabel.Foreground = new SolidColorBrush(Color.FromRgb(0x8b, 0x94, 0x9e));
        _sigStateLabel.FontWeight = FontWeights.Bold;
        _sigStateLabel.VerticalAlignment = VerticalAlignment.Center;
        keyRow.Children.Add(_sigStateLabel);
        root.Children.Add(keyRow);

        root.Children.Add(Lbl("Fuente:"));
        _editor.AcceptsReturn = true;
        _editor.VerticalScrollBarVisibility = ScrollBarVisibility.Auto;
        _editor.Height = 240;
        _editor.Background = new SolidColorBrush(Color.FromRgb(0x0a, 0x0c, 0x12));
        _editor.Foreground = new SolidColorBrush(Color.FromRgb(0xe0, 0xe0, 0xe0));
        _editor.FontSize = 12;
        _editor.Text = LuaSnippets[0].Value;
        _editor.TextChanged += (s, e) => { try { _dirty = true; SetSigState("SIN FIRMAR"); } catch { } };
        root.Children.Add(_editor);

        root.Children.Add(Lbl("Salida / debug:"));
        _status.Foreground = new SolidColorBrush(Color.FromRgb(0x8b, 0x94, 0x9e));
        _status.Margin = new Thickness(0, 0, 0, 4);
        root.Children.Add(_status);
        _output.IsReadOnly = true;
        _output.VerticalScrollBarVisibility = ScrollBarVisibility.Auto;
        _output.Height = 210;
        _output.Background = new SolidColorBrush(Color.FromRgb(0x0a, 0x0c, 0x12));
        _output.Foreground = new SolidColorBrush(Color.FromRgb(0x00, 0xe6, 0x76));
        _output.FontSize = 11;
        root.Children.Add(_output);

        Content = new ScrollViewer { Content = root, VerticalScrollBarVisibility = ScrollBarVisibility.Auto };
        SetSigState("SIN FIRMAR");
        Loaded += async (_, _) => await LoadSessionsAsync();
    }

    private static TextBlock Lbl(string t) => new TextBlock
    {
        Text = t, VerticalAlignment = VerticalAlignment.Center, Margin = new Thickness(0, 0, 6, 0),
        Foreground = Brushes.Gray, FontSize = 11,
    };

    private Button MkButtonAsync(string text, Func<Task> onClick)
    {
        var b = new Button
        {
            Content = text, Height = 28, Padding = new Thickness(10, 0, 10, 0),
            Margin = new Thickness(0, 0, 6, 0),
            Background = new SolidColorBrush(Color.FromRgb(0x16, 0x1a, 0x24)),
            Foreground = new SolidColorBrush(Color.FromRgb(0xc0, 0xc8, 0xd8)),
            BorderBrush = new SolidColorBrush(Color.FromRgb(0x25, 0x2a, 0x36)),
            Cursor = System.Windows.Input.Cursors.Hand,
        };
        b.Click += async (s, e) => { try { await onClick(); } catch (Exception ex) { AppendOut("[!] " + ex.Message); } };
        return b;
    }

    private Button MkButton(string text, Action onClick)
    {
        var b = new Button
        {
            Content = text, Height = 28, Padding = new Thickness(10, 0, 10, 0),
            Margin = new Thickness(0, 0, 6, 0),
            Background = new SolidColorBrush(Color.FromRgb(0x16, 0x1a, 0x24)),
            Foreground = new SolidColorBrush(Color.FromRgb(0xc0, 0xc8, 0xd8)),
            BorderBrush = new SolidColorBrush(Color.FromRgb(0x25, 0x2a, 0x36)),
            Cursor = System.Windows.Input.Cursors.Hand,
        };
        b.Click += (s, e) => { try { onClick(); } catch (Exception ex) { AppendOut("[!] " + ex.Message); } };
        return b;
    }

    private void AppendOut(string line)
    {
        try
        {
            _output.AppendText(line + Environment.NewLine);
            _output.ScrollToEnd();
        }
        catch { }
    }

    private bool IsDectmMode()
    {
        try { return (_modeCombo.SelectedItem?.ToString() ?? "lua") == "dectm"; }
        catch { return false; }
    }

    private void SetSigState(string state)
    {
        try { _sigStateLabel.Text = "firma: " + state; } catch { }
    }

    private void OnModeChanged()
    {
        try
        {
            RefillSnippets();
            _editor.Text = ActiveSnippets().Length > 0 ? ActiveSnippets()[0].Value : "";
            _dirty = true;
            SetSigState("SIN FIRMAR");
            AppendOut("[*] modo " + (IsDectmMode() ? "dectm" : "lua"));
        }
        catch { }
    }

    // --- carga de sesiones ---

    private async Task LoadSessionsAsync()
    {
        try
        {
            if (App.ApiService == null) { _status.Text = "sin API"; return; }
            var sessions = await App.ApiService.GetSessions();
            _sessionCombo.Items.Clear();
            if (sessions != null)
                foreach (var s in sessions)
                    _sessionCombo.Items.Add(s.Id + "  " + s.Hostname);
            _status.Text = "sesiones: " + _sessionCombo.Items.Count;
        }
        catch (Exception ex)
        {
            try { _status.Text = "sesiones: " + ex.Message; } catch { }
        }
    }

    private static string SelectedSessionId(ComboBox combo)
    {
        try
        {
            string s = combo.SelectedItem?.ToString() ?? "";
            int i = s.IndexOf(' ');
            return (i > 0 ? s.Substring(0, i) : s).Trim();
        }
        catch { return ""; }
    }

    // --- validacion estatica ---

    private List<string> CheckScript(string src)
    {
        var problems = new List<string>();
        try
        {
            if (src.Length == 0) problems.Add("vacio: nada que validar");
            if (src.Length > 1024 * 1024) problems.Add($"excede 1MiB ({src.Length} bytes)");
            string low = src.ToLower();
            foreach (var b in Blocked)
                if (low.Contains(b)) problems.Add($"bloqueado por sandbox: '{b}' (serÃ¡ nil en el agente)");
            if (!low.Contains("-- scope:")) problems.Add("sin cabecera de alcance: anade '-- scope: lab-only'");
            var used = new HashSet<string>();
            int idx = 0;
            while ((idx = low.IndexOf("c2.", idx, StringComparison.Ordinal)) >= 0)
            {
                int j = idx + 3;
                while (j < low.Length && (char.IsLetterOrDigit(low[j]) || low[j] == '_' || low[j] == '.')) j++;
                used.Add(low.Substring(idx, Math.Min(j - idx, 24)));
                idx = j;
            }
            foreach (var u in used)
            {
                bool known = KnownApis.Any(k => u == k || u.StartsWith(k + ".") || (k.EndsWith(".*") && u.StartsWith(k.Substring(0, k.Length - 1))));
                if (!known) problems.Add($"c2.* desconocida: '{u}' (puede fallar en el agente)");
            }
            if (problems.Count == 0) problems.Add("OK: sin globales peligrosas; APIs dentro de la superficie conocida.");
        }
        catch (Exception ex) { problems.Add("validacion: " + ex.Message); }
        return problems;
    }

    private async Task ValidateAsync()
    {
        try
        {
            AppendOut("-- VALIDACION ESTATICA --");
            string src = CurrentSource();
            foreach (var p in CheckScript(src))
                AppendOut((p.StartsWith("OK") ? "[OK] " : "[!] ") + p);
        }
        catch { }
        await Task.CompletedTask;
    }

    /// <summary>Devuelve la fuente que se va a ejecutar (transpila si es .dectm).</summary>
    private string CurrentSource()
    {
        if (!IsDectmMode()) return _editor.Text ?? "";
        var result = Dectm.Transpile(_editor.Text ?? "");
        if (!result.Ok)
        {
            AppendOut("[!] .dectm: " + result.Error);
            return "";
        }
        return result.Lua;
    }

    // --- publicar y firmar (lo hace el servidor) ---

    private async Task PublishAsync()
    {
        try
        {
            if (App.ApiService == null) { AppendOut("[!] Sin API."); return; }
            string src = CurrentSource();
            if (string.IsNullOrWhiteSpace(src))
            {
                AppendOut("[!] Fuente vacia.");
                return;
            }
            var name = "script-" + DateTime.Now.ToString("HHmmss");
            var scope = "lab-only";
            AppendOut($"[*] Publicando '{name}' (lo firma el servidor)...");
            var published = await App.ApiService.PublishScript(name, src, scope);
            if (published == null)
            {
                AppendOut("[!] El servidor rechazo el script: " + (App.ApiService.LastError ?? "sin detalle"));
                return;
            }
            _scriptId = published.ScriptId ?? "";
            _signature = published.Signature ?? "";
            _kid = published.Kid ?? "";
            _dirty = false;
            SetSigState($"FIRMADO (kid {_kid})");
            AppendOut($"[OK] script_id={_scriptId} firmando con kid={_kid}");
            _status.Text = "script firmado por el servidor";
        }
        catch (Exception ex)
        {
            AppendOut("[!] Publicar: " + ex.Message);
        }
    }

    // --- envio y poll ---

    private async Task RunScriptAsync()
    {
        _pollStop = false;
        try
        {
            string sid = SelectedSessionId(_sessionCombo);
            if (string.IsNullOrEmpty(sid)) { AppendOut("[!] Elige sesion (REFRESCAR)."); return; }

            string src = CurrentSource();
            if (string.IsNullOrWhiteSpace(src)) { AppendOut("[!] Fuente vacia."); return; }
            if (src.Length > 1024 * 1024) { AppendOut("[!] Excede 1MiB."); return; }

            string args = src;
            if (_envelopeCheck.IsChecked == true || !string.IsNullOrEmpty(_signature))
            {
                if (string.IsNullOrEmpty(_signature))
                {
                    AppendOut("[!] El script no esta firmado: pulsa PUBLICAR Y FIRMAR antes de ejecutar.");
                    return;
                }
                args = Newtonsoft.Json.JsonConvert.SerializeObject(new
                {
                    lang = "lua",
                    source = src,
                    script_id = _scriptId,
                    sig = _signature,
                    kid = _kid,
                });
            }
            await SendEnvelopeAsync(sid, args, $"-- EJECUTAR en {sid} ({src.Length} bytes) --");
        }
        catch (Exception ex)
        {
            AppendOut("[!] Ejecutar: " + ex.Message);
        }
    }

    private async Task SendEnvelopeAsync(string sid, string args, string? header = null)
    {
        try
        {
            if (App.ApiService == null) { AppendOut("[!] Sin API."); return; }
            AppendOut(header ?? $"-- EJECUTAR en {sid} ({args.Length} bytes) --");
            var task = await App.ApiService.SubmitTask(sid, "script", args);
            if (task == null) { AppendOut("[!] El servidor no acepto la tarea: " + (App.ApiService.LastError ?? "")); return; }
            long tid = task.Id;
            AppendOut($"[*] task {tid} encolada; esperando resultado (max 40s)...");
            _status.Text = $"task {tid} en curso...";
            for (int i = 0; i < 40 && !_pollStop; i++)
            {
                await Task.Delay(1000);
                List<C2Task>? list = null;
                try { list = await App.ApiService.GetTasks(sid); } catch { continue; }
                if (list == null) continue;
                C2Task? mine = null;
                foreach (var t in list)
                {
                    try { if (t.Id == tid) { mine = t; break; } } catch { }
                }
                if (mine == null) continue;
                string st = "";
                try { st = (mine.Status ?? "").ToLower(); } catch { }
                bool done = st == "completed" || st == "success" || st == "done" || st == "failed" || st == "error" || st == "timeout";
                try { if (mine.CompletedAt != null) done = true; } catch { }
                string res = "", err = "";
                try { res = mine.Result ?? ""; err = mine.Error ?? ""; } catch { }
                if (!string.IsNullOrEmpty(res) || !string.IsNullOrEmpty(err)) done = true;
                if (done)
                {
                    AppendOut($"[OK] task {tid} estado={st}");
                    if (!string.IsNullOrEmpty(res))
                        AppendOut("-- SALIDA --\n" + (res.Length > 4000 ? res.Substring(0, 4000) + "...[truncado]" : res));
                    if (!string.IsNullOrEmpty(err)) AppendOut("[!] error agente: " + err);
                    if (string.IsNullOrEmpty(res) && string.IsNullOrEmpty(err))
                        AppendOut("[*] Sin salida (revisa el timeout de 30s del agente o el sandbox).");
                    _status.Text = $"task {tid} {st} - edita y re-ejecuta para debug.";
                    return;
                }
            }
            if (_pollStop) AppendOut("[*] Poll detenido; la tarea puede seguir en el agente.");
            else AppendOut("[!] Timeout 40s sin resultado (el agente corta a 30s; prueba algo mas corto).");
            _status.Text = "sin resultado - acorta el script y reintenta.";
        }
        catch (Exception ex)
        {
            AppendOut("[!] Envio: " + ex.Message);
        }
    }
}
