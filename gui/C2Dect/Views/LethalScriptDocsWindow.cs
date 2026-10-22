using System;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;

namespace C2Dect.Views;

/// <summary>
/// Lethal Script Docs: la referencia que necesita quien escribe un script.
///
/// Es la ventana de documentacion del modulo comercial, con el contenido
/// adaptado a la edicion Lite: la superficie c2.* es la minima, el sandbox
/// no expone shell ni escritura, y la tabla de tecnicas cubre las ordenes
/// que el catalogo del agente permite de verdad.
/// </summary>
public class LethalScriptDocsWindow : Window
{
    public LethalScriptDocsWindow()
    {
        Title = "Lethal Script Docs - referencia de scripting (Lite)";
        Width = 1000;
        Height = 760;
        Background = new SolidColorBrush(Color.FromRgb(0x0c, 0x0e, 0x14));
        FontFamily = new FontFamily("Segoe UI");
        WindowStartupLocation = WindowStartupLocation.CenterOwner;

        var root = new StackPanel { Margin = new Thickness(16) };

        root.Children.Add(Heading("LETHAL SCRIPT LITE - REFERENCIA"));
        root.Children.Add(Body("El scripting es un modulo ABIERTO de la edicion Lite: cualquiera puede escribir un script, " +
            "publicarlo (queda firmado con la clave del marketplace) y compartirlo. Lo que un script NO puede es " +
            "ejecutar codigo arbitrario: la superficie c2.* es la minima y cada orden pasa el gate de impacto."));

        root.Children.Add(Section("1. Formato del script"));
        root.Children.Add(Code(
            "-- scope: lab-only\n" +
            "c2.log(\"INFO\", \"hola desde \" .. c2.session_id())\n" +
            "local s = c2.task(\"sysinfo\")      -- ordenes del catalogo del agente\n" +
            "c2.log(s)\n" +
            "return s"));

        root.Children.Add(Section("2. Superficie c2.* disponible"));
        root.Children.Add(Code(
            "c2.log(level, msg)      escribe en la salida del script\n" +
            "c2.sleep(ms)            espera (max 120s por llamada)\n" +
            "c2.args()               argumentos del envelope\n" +
            "c2.session_id()         id de la sesion\n" +
            "c2.script_id(src)       sha256 del source\n" +
            "c2.beacon()             tabla con el estado del agente\n" +
            "c2.task(\"orden args\")  ejecuta una orden del catalogo\n" +
            "c2.fs.list([path])      lista un directorio (solo lectura)\n" +
            "c2.fs.read(path)        lee un fichero (max 256 KiB)\n" +
            "c2.fs.stat(path)        metadatos de un fichero\n" +
            "c2.proc.list()          procesos en ejecucion\n" +
            "c2.sys.info()           inventario del host\n" +
            "c2.sys.env()            entorno del agente\n" +
            "c2.net.info()           interfaces de red\n" +
            "c2.net.conns()          conexiones de red\n" +
            "c2.crypto.sha256(s)     hash\n" +
            "c2.crypto.b64encode(s)  base64\n" +
            "c2.crypto.b64decode(s)  base64\n" +
            "c2.crypto.rand(n)       aleatorio (1..64 bytes)"));

        root.Children.Add(Section("3. Lo que NO existe"));
        root.Children.Add(Body(
            "No hay shell (c2.exec), no hay escritura, borrado ni movimiento de ficheros (c2.fs.write / rm / move / copy), " +
            "no hay descarga ni subida de ficheros, no hay WASM, no hay os/io/debug/require/loadstring (el agente los anula) " +
            "y no hay API de impacto: las ordenes de impacto (shell, kill, rm, mv) necesitan -allow-impact en el agente " +
            "y el consentimiento del operador en la consola."));

        root.Children.Add(Section("4. Como llega un script al agente"));
        root.Children.Add(Body(
            "1) el operador publica el script: el servidor valida la fuente, deduce el alcance de la cabecera, " +
            "lo firma con Ed25519 y devuelve script_id, sig y kid;\n" +
            "2) la consola construye el envelope {lang, source, script_id, sig, kid} y lo envia como la tarea 'script';\n" +
            "3) el servidor vuelve a verificar la firma, la ventana temporal (exp/nbf) y el alcance antes de encolar;\n" +
            "4) el agente verifica la firma con C2_SCRIPT_PUBKEY, comprueba la denylist y ejecuta en el sandbox " +
            "con un timeout de 30s y un tope de salida de 1 MiB."));

        root.Children.Add(Section("5. Manifiestos .dectm"));
        root.Children.Add(Body(
            "La edicion Lite transpila un subconjunto de manifiestos .dectm a Lua desde la propia consola " +
            "(sin Python). Campos: name, version, scope, mitre, reruns y steps[] con las operaciones log, task, " +
            "sleep, beacon, hash y crypto.sha256."));
        root.Children.Add(Code(
            "# scope: lab-only\n" +
            "name: demo-triage\n" +
            "version: 1.0.0\n" +
            "scope: lab-only\n" +
            "mitre: [T1082, T1057]\n" +
            "reruns: 1\n" +
            "steps:\n" +
            "  - {name: aviso, log: \"inicio triage\"}\n" +
            "  - {name: sys, task: sysinfo}\n" +
            "  - {name: espera, sleep: 2000}"));

        root.Children.Add(Section("6. Tecnicas que cubren las ordenes del catalogo"));
        root.Children.Add(Body(
            "Cada orden del catalogo del agente lleva las tecnicas de la matriz ATT&CK que cubre. Es la referencia " +
            "para saber que puede revelar un script escrito con la superficie de arriba:"));
        root.Children.Add(Code(
            "T1082  System Information Discovery      sysinfo, hostname, env, uptime\n" +
            "T1033  System Owner/User Discovery     whoami\n" +
            "T1057  Process Discovery               ps, proc.list\n" +
            "T1016  System Network Configuration     netinfo, net.info\n" +
            "T1049  System Network Connections       connections, net.conns\n" +
            "T1007  System Service Discovery         services\n" +
            "T1083  File and Directory Discovery     ls, find, stat, hashfile\n" +
            "T1005  Data from Local System          cat, download, fs.read\n" +
            "T1105  Ingress Tool Transfer           upload, cp, mkdir\n" +
            "T1059  Command and Scripting Interpreter   shell, script (opt-in)"));

        root.Children.Add(Section("7. Firmas y confianza"));
        root.Children.Add(Body(
            "La firma es Ed25519 sobre el source EXACTO (sin canonicalizar). El kid es hex(sha256(pubkey))[:16]. " +
            "El agente exige C2_SCRIPT_PUBKEY: sin clave el scripting queda desactivado, y un script sin firma " +
            "nunca se ejecuta. La denylist (C2_SCRIPT_DENY / C2_SCRIPT_DENY_FILE) revoca por script_id sin " +
            "reiniciar el agente."));

        Content = new ScrollViewer { Content = root, VerticalScrollBarVisibility = ScrollBarVisibility.Auto };
    }

    private static TextBlock Heading(string t) => new TextBlock
    {
        Text = t,
        Foreground = new SolidColorBrush(Color.FromRgb(0x00, 0xe6, 0x76)),
        FontWeight = FontWeights.Bold,
        FontSize = 16,
        Margin = new Thickness(0, 0, 0, 10),
    };

    private static TextBlock Section(string t) => new TextBlock
    {
        Text = t,
        Foreground = new SolidColorBrush(Color.FromRgb(0xd4, 0xa0, 0x17)),
        FontWeight = FontWeights.Bold,
        FontSize = 13,
        Margin = new Thickness(0, 16, 0, 6),
    };

    private static TextBlock Body(string t) => new TextBlock
    {
        Text = t,
        Foreground = new SolidColorBrush(Color.FromRgb(0xc0, 0xc8, 0xd8)),
        FontSize = 12,
        TextWrapping = TextWrapping.Wrap,
        Margin = new Thickness(0, 0, 0, 6),
    };

    private static TextBlock Code(string t) => new TextBlock
    {
        Text = t,
        Foreground = new SolidColorBrush(Color.FromRgb(0x00, 0xb8, 0xff)),
        FontFamily = new FontFamily("Consolas"),
        FontSize = 11,
        TextWrapping = TextWrapping.Wrap,
        Background = new SolidColorBrush(Color.FromRgb(0x0a, 0x0c, 0x12)),
        Padding = new Thickness(10),
        Margin = new Thickness(0, 0, 0, 8),
    };
}
