# Smoke test de la consola: abre la GUI, entra y comprueba que los paneles
# funcionales cargan (incluido Lethal Script y su documentacion).
#
#   powershell -ExecutionPolicy Bypass -File scripts\gui_smoke.ps1
#
# Necesita el servidor construido (build\server.exe) y la consola en
# gui\C2Dect\bin\Release\net8.0-windows\C2Dect-Lite.exe.

Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class Win32 {
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
    [DllImport("user32.dll")] public static extern bool SetCursorPos(int X, int Y);
    [DllImport("user32.dll")] public static extern void mouse_event(uint flags, uint dx, uint dy, uint data, IntPtr extra);
    [DllImport("user32.dll")] public static extern bool PostMessage(IntPtr hWnd, uint msg, IntPtr wParam, IntPtr lParam);
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll")] public static extern bool BringWindowToTop(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern bool SetWindowPos(IntPtr hWnd, IntPtr after, int x, int y, int cx, int cy, uint flags);
    [DllImport("user32.dll")] public static extern void keybd_event(byte vk, byte scan, uint flags, UIntPtr extra);

    public const uint LEFTDOWN = 0x0002;
    public const uint LEFTUP = 0x0004;

    public static void ForceForeground(IntPtr hWnd) {
        keybd_event(0x12, 0, 0x0000, UIntPtr.Zero);
        keybd_event(0x12, 0, 0x0002, UIntPtr.Zero);
        BringWindowToTop(hWnd);
        SetForegroundWindow(hWnd);
    }
}
"@

$ErrorActionPreference = "Stop"
$base = Join-Path $PSScriptRoot "..\build\.e2e"
$work = Join-Path $base "work"
$gui = Join-Path $PSScriptRoot "..\gui\C2Dect\bin\Release\net8.0-windows\C2Dect-Lite.exe"
$serverExe = Join-Path $PSScriptRoot "..\build\server.exe"

Get-Process -Name "agent" -ErrorAction SilentlyContinue | Stop-Process -Force
Get-Process -Name "server" -ErrorAction SilentlyContinue |
    Where-Object { $_.Path -like "*c2-dect-lite*" } | Stop-Process -Force
Start-Sleep -Milliseconds 500
Remove-Item $work -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path $work -Force | Out-Null

$env:C2_JWT_SECRET = "test-secret-for-the-c2-dect-lite-e2e-run-0123456789"
$env:C2_BOOTSTRAP_USER = "admin"
$env:C2_BOOTSTRAP_PASSWORD = "super-secret-admin-pass"
$env:C2_LISTEN_ADDR = "127.0.0.1:18443"

$server = Start-Process -FilePath $serverExe -WorkingDirectory $work -PassThru -WindowStyle Hidden `
    -RedirectStandardOutput (Join-Path $work "server.out") -RedirectStandardError (Join-Path $work "server.err")
Start-Sleep -Seconds 3

function Fail($msg) { Write-Host "FAIL: $msg" -ForegroundColor Red; exit 1 }

$root = [System.Windows.Automation.AutomationElement]::RootElement
$windowCondition = New-Object System.Windows.Automation.PropertyCondition(
    [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
    [System.Windows.Automation.ControlType]::Window)
$buttonCondition = New-Object System.Windows.Automation.PropertyCondition(
    [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
    [System.Windows.Automation.ControlType]::Button)
$editCondition = New-Object System.Windows.Automation.PropertyCondition(
    [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
    [System.Windows.Automation.ControlType]::Edit)
$textCondition = New-Object System.Windows.Automation.PropertyCondition(
    [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
    [System.Windows.Automation.ControlType]::Text)

function Click-Element($element) {
    # InvokePattern no depende del raton ni del foco
    try {
        $inv = $element.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern)
        $inv.Invoke()
        return
    } catch { }
    $r = $element.Current.BoundingRectangle
    [Win32]::SetCursorPos([int]($r.X + $r.Width / 2), [int]($r.Y + $r.Height / 2)) | Out-Null
    [Win32]::mouse_event([Win32]::LEFTDOWN, 0, 0, 0, [IntPtr]::Zero)
    [Win32]::mouse_event([Win32]::LEFTUP, 0, 0, 0, [IntPtr]::Zero)
}

function Click-Named($parent, $name) {
    foreach ($b in $parent.FindAll([System.Windows.Automation.TreeScope]::Descendants, $buttonCondition)) {
        if ($b.Current.Name -eq $name) { Click-Element $b; return $true }
    }
    return $false
}

function Scroll-Sidebar($console, $notches) {
    $side = $console.Current.BoundingRectangle
    [Win32]::SetCursorPos([int]($side.X + 100), [int]($side.Y + 300)) | Out-Null
    for ($i = 0; $i -lt $notches; $i++) {
        [Win32]::mouse_event(0x0800, 0, 0, [uint32]4294967176, [IntPtr]::Zero)
        Start-Sleep -Milliseconds 60
    }
    Start-Sleep -Milliseconds 400
}

function Click-Named-Visible($console, $name) {
    # baja el menu lateral hasta que la entrada este dentro de la pantalla
    for ($attempt = 0; $attempt -lt 10; $attempt++) {
        foreach ($b in $console.FindAll([System.Windows.Automation.TreeScope]::Descendants, $buttonCondition)) {
            if ($b.Current.Name -ne $name) { continue }
            $r = $b.Current.BoundingRectangle
            $screen = [System.Windows.Forms.SystemInformation]::VirtualScreen
            if ($r.Y -lt $screen.Top -or ($r.Y + $r.Height) -gt ($screen.Bottom + 40)) {
                Scroll-Sidebar $console 2
                continue
            }
            Click-Element $b
            return $true
        }
        Scroll-Sidebar $console 2
    }
    return $false
}

function Save-Shot($window, $file) {
    try {
        $r = $window.Current.BoundingRectangle
        if ($r.Width -le 0 -or $r.Height -le 0) { return }
        $bmp = New-Object System.Drawing.Bitmap([int]$r.Width, [int]$r.Height)
        $g = [System.Drawing.Graphics]::FromImage($bmp)
        $g.CopyFromScreen([int]$r.X, [int]$r.Y, 0, 0, $bmp.Size)
        $bmp.Save((Join-Path $base $file), [System.Drawing.Imaging.ImageFormat]::Png)
        $g.Dispose(); $bmp.Dispose()
    } catch { Write-Host ("INFO: screenshot skipped -> " + $_.Exception.Message) }
}

function Show-Texts($parent) {
    $out = @()
    foreach ($t in $parent.FindAll([System.Windows.Automation.TreeScope]::Descendants, $textCondition)) {
        if ($t.Current.Name) { $out += $t.Current.Name }
    }
    Write-Host ("INFO: texts -> " + (($out | Select-Object -First 8) -join " || "))
}

try {
    $app = Start-Process -FilePath $gui -PassThru

    # la maquina puede tardar en levantar la ventana: se espera a que aparezca
    $login = $null
    $console = $null
    for ($wait = 0; $wait -lt 30; $wait++) {
        Start-Sleep -Seconds 1
        foreach ($w in $root.FindAll([System.Windows.Automation.TreeScope]::Children, $windowCondition)) {
            if ($w.Current.ProcessId -ne $app.Id) { continue }
            if ($w.Current.Name -like "*Login*") { $login = $w }
            if ($w.Current.Name -like "*Operator Console*") { $console = $w }
        }
        if ($login -or $console) { break }
    }

    if (-not $console) {
        if (-not $login) { Fail "the login window did not appear" }
        Write-Host "OK:   login window appeared"

        # rellena y verifica: un foco robado por otra ventana puede colar caracteres
        for ($round = 0; $round -lt 5; $round++) {
            foreach ($e in $login.FindAll([System.Windows.Automation.TreeScope]::Descendants, $editCondition)) {
                $id = $e.Current.AutomationId
                $pattern = $e.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern)
                if ($id -eq "UsernameBox") { $pattern.SetValue("admin") }
                if ($id -eq "ServerBox") { $pattern.SetValue("http://127.0.0.1:18443") }
                if ($id -eq "PasswordBox") { $pattern.SetValue("super-secret-admin-pass") }
            }
            Start-Sleep -Milliseconds 300
            $bad = $false
            foreach ($e in $login.FindAll([System.Windows.Automation.TreeScope]::Descendants, $editCondition)) {
                $id = $e.Current.AutomationId
                $value = $e.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern).Current.Value
                if ($id -eq "ServerBox" -and $value -ne "http://127.0.0.1:18443") { $bad = $true }
                if ($id -eq "UsernameBox" -and $value -ne "admin") { $bad = $true }
            }
            if (-not $bad) { break }
        }
        Write-Host "OK:   credentials filled"

        # la ventana se redimensiona para que el boton quede dentro de la pantalla
        [Win32]::SetWindowPos([IntPtr]$login.Current.NativeWindowHandle, [IntPtr](-1), 60, 0, 700, 900, 0x0040) | Out-Null
        [Win32]::ForceForeground([IntPtr]$login.Current.NativeWindowHandle)
        Start-Sleep -Milliseconds 800

        $log = Join-Path $env:LOCALAPPDATA "C2Dect\logs\gui.log"
        for ($try = 0; $try -lt 8; $try++) {
            Start-Sleep -Seconds 2
            $attempted = (Test-Path $log) -and ((Get-Content $log -Tail 12) -match "Login attempt")
            if ($attempted) { break }
            [Win32]::ForceForeground([IntPtr]$login.Current.NativeWindowHandle)
            Start-Sleep -Milliseconds 400
            foreach ($b in $login.FindAll([System.Windows.Automation.TreeScope]::Descendants, $buttonCondition)) {
                if ($b.Current.Name -like "*LOGIN*") {
                    try {
                        $inv = $b.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern)
                        $inv.Invoke()
                    } catch {
                        Click-Element $b
                    }
                }
            }
            Start-Sleep -Milliseconds 300
            [Win32]::PostMessage([IntPtr]$login.Current.NativeWindowHandle, 0x0100, 0x0D, 0) | Out-Null
            [Win32]::PostMessage([IntPtr]$login.Current.NativeWindowHandle, 0x0101, 0x0D, 0) | Out-Null
        }
        Start-Sleep -Seconds 8

        $console = $null
        for ($wait = 0; $wait -lt 15; $wait++) {
            foreach ($w in $root.FindAll([System.Windows.Automation.TreeScope]::Children, $windowCondition)) {
                if ($w.Current.ProcessId -eq $app.Id -and $w.Current.Name -like "*Operator Console*") { $console = $w }
            }
            if ($console) { break }
            Start-Sleep -Seconds 1
        }
    }
    else {
        Write-Host "OK:   the stored session was restored and the console opened directly"
    }

    if (-not $console) { Fail "the operator console did not open" }
    Write-Host "OK:   the operator console opened"
    Save-Shot $console "console.png"

    [Win32]::ForceForeground([IntPtr]$console.Current.NativeWindowHandle)
    Start-Sleep -Milliseconds 600

    # marketplace
    if (Click-Named-Visible $console "  Script Market") {
        Start-Sleep -Seconds 4
        Save-Shot $console "marketplace.png"
        $panelText = ""
        foreach ($t in $console.FindAll([System.Windows.Automation.TreeScope]::Descendants, $textCondition)) {
            if ($t.Current.Name -like "SCRIPT*") { $panelText = $t.Current.Name }
        }
        if ($panelText -eq "") { Fail "the marketplace panel did not render" }
        Write-Host "OK:   marketplace panel opened and rendered"
    } else { Fail "the script market entry was not found in the sidebar" }

    # modulo inactivo
    if (Click-Named-Visible $console "  Recon (Go)") {
        Start-Sleep -Seconds 3
        Save-Shot $console "inactive.png"
        Write-Host "OK:   inactive module panel opened"
    } else { Fail "the recon entry was not found in the sidebar" }

    # lethal script: ventana propia, como en el modulo comercial
    if (Click-Named-Visible $console "  Lethal Script") {
        Start-Sleep -Seconds 5
        $scriptWindow = $null
        $seen = @()
        for ($wait = 0; $wait -lt 10; $wait++) {
            # una ventana con Owner cuelga del owner en el arbol de UIA
            foreach ($w in $console.FindAll([System.Windows.Automation.TreeScope]::Descendants, $windowCondition)) {
                $seen += "'" + $w.Current.Name + "'"
                if ($w.Current.Name -like "Lethal Script*" -and $w.Current.ProcessId -eq $app.Id) { $scriptWindow = $w }
            }
            foreach ($w in $root.FindAll([System.Windows.Automation.TreeScope]::Children, $windowCondition)) {
                if ($w.Current.Name -like "Lethal Script*" -and $w.Current.ProcessId -eq $app.Id) { $scriptWindow = $w }
            }
            if ($scriptWindow) { break }
            Start-Sleep -Seconds 1
        }
        if (-not $scriptWindow) {
            Write-Host ("INFO: windows -> " + ($seen -join ", "))
            $guiLog = Join-Path $env:LOCALAPPDATA "C2Dect\logs\gui.log"
            if (Test-Path $guiLog) { Write-Host ("INFO: gui log -> " + ((Get-Content $guiLog -Tail 10) -join " / ")) }
            Fail "the lethal script window did not open"
        }
        $scriptButtons = @()
        foreach ($b in $scriptWindow.FindAll([System.Windows.Automation.TreeScope]::Descendants, $buttonCondition)) {
            $scriptButtons += $b.Current.Name
        }
        foreach ($needed in @("VALIDAR", "PUBLICAR Y FIRMAR", "EJECUTAR", "DOCS")) {
            if ($scriptButtons -notcontains $needed) { Fail "the lethal script window has no $needed button" }
        }
        Write-Host "OK:   lethal script window opened with the full editor toolbar"
        Save-Shot $scriptWindow "lethal-script.png"

        foreach ($b in $scriptWindow.FindAll([System.Windows.Automation.TreeScope]::Descendants, $buttonCondition)) {
            if ($b.Current.Name -eq "DOCS") { Click-Element $b }
        }
        Start-Sleep -Seconds 3
        $docsWindow = $null
        foreach ($w in $scriptWindow.FindAll([System.Windows.Automation.TreeScope]::Descendants, $windowCondition)) {
            if ($w.Current.Name -like "Lethal Script Docs*") { $docsWindow = $w }
        }
        foreach ($w in $root.FindAll([System.Windows.Automation.TreeScope]::Children, $windowCondition)) {
            if ($w.Current.Name -like "Lethal Script Docs*" -and $w.Current.ProcessId -eq $app.Id) { $docsWindow = $w }
        }
        if (-not $docsWindow) { Fail "the lethal script docs window did not open" }
        Write-Host "OK:   lethal script docs window opened"
        Save-Shot $docsWindow "lethal-docs.png"
    } else { Fail "the lethal script entry was not found in the sidebar" }

    # auditoria
    [Win32]::ForceForeground([IntPtr]$console.Current.NativeWindowHandle)
    Start-Sleep -Milliseconds 500
    if (Click-Named-Visible $console "  Audit") {
        Start-Sleep -Seconds 4
        Save-Shot $console "audit.png"
        Write-Host "OK:   audit panel opened"
    } else { Fail "the audit entry was not found in the sidebar" }
}
catch {
    Write-Host "FAIL: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}
finally {
    Get-Process -Name "C2Dect-Lite" -ErrorAction SilentlyContinue | Stop-Process -Force
    Get-Process -Name "agent" -ErrorAction SilentlyContinue | Stop-Process -Force
    Stop-Process -Id $server.Id -Force -ErrorAction SilentlyContinue
}
Write-Host ""
Write-Host "GUI SMOKE TEST PASSED" -ForegroundColor Cyan
exit 0
