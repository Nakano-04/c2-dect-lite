$ErrorActionPreference = "Stop"
$base = Join-Path $PSScriptRoot "..\build\.e2e"
# directorio propio por ejecucion: si un servidor anterior mantiene el fichero
# abierto, la base de datos de esta corrida nunca se pisa con la anterior
$work = Join-Path $base ("work-" + [guid]::NewGuid().ToString("N").Substring(0, 8))
New-Item -ItemType Directory -Path $work -Force | Out-Null

# limpieza: un servidor o agente de una corrida anterior tiene el puerto y la
# base de datos ocupados
Get-Process -Name "agent" -ErrorAction SilentlyContinue | Stop-Process -Force
Get-Process -Name "server" -ErrorAction SilentlyContinue |
    Where-Object { $_.Path -like "*c2-dect-lite*" } | Stop-Process -Force
Start-Sleep -Milliseconds 500


$env:C2_JWT_SECRET = "test-secret-for-the-c2-dect-lite-e2e-run-0123456789"
$env:C2_BOOTSTRAP_USER = "admin"
$env:C2_BOOTSTRAP_PASSWORD = "super-secret-admin-pass"
$env:C2_LISTEN_ADDR = "127.0.0.1:18443"
$env:C2_KEY_ROTATION_EVERY = "3"

$server = Start-Process -FilePath (Join-Path $PSScriptRoot "..\build\server.exe") -WorkingDirectory $work -PassThru -WindowStyle Hidden `
    -RedirectStandardOutput (Join-Path $work "server.out") -RedirectStandardError (Join-Path $work "server.err")

# el servidor tarda en levantar (migraciones + generacion de la clave del
# marketplace): se espera a que responda el health check
$up = $false
for ($wait = 0; $wait -lt 30; $wait++) {
    Start-Sleep -Seconds 1
    try {
        $h = Invoke-RestMethod -Uri "http://127.0.0.1:18443/api/health" -TimeoutSec 3
        if ($h.status -eq "ok") { $up = $true; break }
    } catch { }
}
if (-not $up) { Write-Host "FAIL: the server did not start" -ForegroundColor Red; exit 1 }

function Fail($msg) { Write-Host "FAIL: $msg" -ForegroundColor Red; exit 1 }
function Ok($msg) { Write-Host "OK:   $msg" -ForegroundColor Green }

try {
    $health = Invoke-RestMethod -Uri "http://127.0.0.1:18443/api/health" -TimeoutSec 10
    if ($health.version -ne "2.0.0") { Fail "unexpected version $($health.version)" }
    Ok "health ok, version $($health.version)"

    # --- login ---
    $login = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18443/api/auth/login" `
        -ContentType "application/json" -Body (@{ username = "admin"; password = "super-secret-admin-pass" } | ConvertTo-Json)
    if (-not $login.token) { Fail "no token returned" }
    Ok "login ok, tenant $($login.operator.tenant_id), role $($login.operator.role)"
    $headers = @{ Authorization = "Bearer $($login.token)" }

    # --- agente con la clave de confianza del marketplace ---
    $pub = Invoke-RestMethod -Uri "http://127.0.0.1:18443/api/scripts/pubkey" -Headers $headers -TimeoutSec 10
    $env:C2_SCRIPT_PUBKEY = $pub.pubkey
    $agent = Start-Process -FilePath (Join-Path $PSScriptRoot "..\build\agent.exe") -WorkingDirectory $work -PassThru -WindowStyle Hidden `
        -ArgumentList @("-s", "127.0.0.1", "-p", "18443", "-S", "2") `
        -RedirectStandardOutput (Join-Path $work "agent.out") -RedirectStandardError (Join-Path $work "agent.err")

    # el agente puede tardar un par de intentos en registrar (si el servidor
    # aun no escucha, el beacon falla y reintenta con backoff): se espera a que
    # aparezca en vez de dar un tiempo fijo
    $sessions = $null
    for ($wait = 0; $wait -lt 30; $wait++) {
        Start-Sleep -Seconds 1
        try {
            $sessions = Invoke-RestMethod -Uri "http://127.0.0.1:18443/api/sessions" -Headers $headers -TimeoutSec 5
        } catch { continue }
        if ($sessions.sessions -and $sessions.sessions.Count -ge 1) { break }
    }
    if (-not $sessions -or -not $sessions.sessions -or $sessions.sessions.Count -lt 1) { Fail "the agent did not register" }
    $sid = $sessions.sessions[0].id
    Ok "agent registered: $sid ($($sessions.sessions[0].hostname))"

    # --- tasking ---
    $task = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18443/api/sessions/$sid/task" -Headers $headers `
        -ContentType "application/json" -Body (@{ command = "sysinfo" } | ConvertTo-Json)
    $taskId = $task.task.id
    Ok "task submitted: $taskId"
    $result = ""
    for ($i = 0; $i -lt 15; $i++) {
        Start-Sleep -Seconds 2
        $tasks = Invoke-RestMethod -Uri "http://127.0.0.1:18443/api/sessions/$sid/tasks" -Headers $headers -TimeoutSec 10
        $t = $tasks.tasks | Where-Object { $_.id -eq $taskId } | Select-Object -First 1
        if ($t.status -eq "completed" -or $t.status -eq "error") { $result = $t.result; break }
    }
    if (-not $result) { Fail "the task did not complete" }
    if ($result -notmatch "Hostname") { Fail "unexpected result: $result" }
    Ok "task completed, result contains the inventory"

    # --- commands outside the allow list ---
    $bad = $null
    try { $bad = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18443/api/sessions/$sid/task" -Headers $headers `
            -ContentType "application/json" -Body (@{ command = "whoami; rm -rf /" } | ConvertTo-Json) }
    catch { }
    if ($bad) { Fail "a command outside the allow list was accepted" }
    Ok "commands outside the allow list are refused"

    # --- impact level: needs the explicit consent ---
    $impact = $null
    try { $impact = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18443/api/sessions/$sid/task" -Headers $headers `
            -ContentType "application/json" -Body (@{ command = "shell" } | ConvertTo-Json) }
    catch { }
    if ($impact) { Fail "an impact command was accepted without the consent" }
    Ok "impact commands are refused without the explicit consent"

    $impactOk = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18443/api/sessions/$sid/task" -Headers $headers `
        -ContentType "application/json" -Body (@{ command = "shell"; confirm = "AGENT-EXEC" } | ConvertTo-Json)
    if (-not $impactOk.task) { Fail "the impact command was not accepted with the consent" }
    Ok "impact commands are accepted with the explicit consent"

    # el agente corre SIN -allow-impact: tiene que negarse a ejecutar la orden
    $refused = $false
    for ($i = 0; $i -lt 12; $i++) {
        Start-Sleep -Seconds 2
        $tasks = Invoke-RestMethod -Uri "http://127.0.0.1:18443/api/sessions/$sid/tasks" -Headers $headers -TimeoutSec 10
        $t = $tasks.tasks | Where-Object { $_.id -eq $impactOk.task.id } | Select-Object -First 1
        if ($t.status -eq "error") { $refused = $true; break }
        if ($t.status -eq "completed") { break }
    }
    if (-not $refused) { Fail "the agent executed an impact command without -allow-impact" }
    Ok "the agent refuses the impact commands unless it is started with -allow-impact"

    $catalog = Invoke-RestMethod -Uri "http://127.0.0.1:18443/api/commands" -Headers $headers -TimeoutSec 10
    $reads = @($catalog.commands | Where-Object { $_.risk -eq "read" })
    $writes = @($catalog.commands | Where-Object { $_.risk -eq "write" })
    $impacts = @($catalog.commands | Where-Object { $_.risk -eq "impact" })
    if ($reads.Count -lt 10) { Fail "the read only tier is too small" }
    if ($impacts.Count -ne 4) { Fail "the impact tier should hold exactly shell, kill, rm and mv" }
    Ok "command tiers: $($reads.Count) read, $($writes.Count) write, $($impacts.Count) impact"


    # --- modules ---
    $modules = Invoke-RestMethod -Uri "http://127.0.0.1:18443/api/modules" -Headers $headers -TimeoutSec 10
    $active = @($modules.modules | Where-Object { $_.available })
    $inactive = @($modules.modules | Where-Object { -not $_.available })
    if ($active.Count -lt 1) { Fail "no active module" }
    if ($inactive.Count -lt 1) { Fail "no inactive module declared" }
    Ok "module catalog: $($active.Count) active, $($inactive.Count) inactive"

    # --- marketplace ---
    $catalog = Invoke-RestMethod -Uri "http://127.0.0.1:18443/api/marketplace/catalog" -Headers $headers -TimeoutSec 10
    if ($catalog.packages.Count -lt 4) { Fail "the marketplace catalog is too small" }
    $pro = $catalog.packages | Where-Object { $_.id -eq "pro-analytics" } | Select-Object -First 1
    if ($pro.available) { Fail "the commercial package is marked as available" }
    if (-not $pro.signed) { Fail "the packages are not signed" }
    Ok "marketplace catalog: $($catalog.packages.Count) packages, signatures present"

    $install = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18443/api/marketplace/packages/lite-recipes/install" -Headers $headers -TimeoutSec 10
    Ok "package installed: $($install.package) $($install.version)"

    $blocked = $false
    try {
        Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18443/api/marketplace/packages/pro-analytics/install" -Headers $headers -TimeoutSec 10 | Out-Null
    } catch { $blocked = $true }
    if (-not $blocked) { Fail "an inactive package could be installed" }
    Ok "inactive packages are refused"

    $recipes = Invoke-RestMethod -Uri "http://127.0.0.1:18443/api/marketplace/recipes" -Headers $headers -TimeoutSec 10
    if ($recipes.locked) { Fail "the recipes are locked after the installation" }
    Ok "recipes unlocked: $($recipes.recipes.Count)"

    # --- marketplace seen from the Script Market view ---
    $scripts = Invoke-RestMethod -Uri "http://127.0.0.1:18443/api/scripts/list" -Headers $headers -TimeoutSec 10
    if ($scripts.scripts.Count -lt 3) { Fail "the script market does not list the lite packages" }
    $detail = Invoke-RestMethod -Uri "http://127.0.0.1:18443/api/scripts/get?name=lite-core" -Headers $headers -TimeoutSec 10
    if (-not $detail.sig) { Fail "the published package is not signed" }
    $pub = Invoke-RestMethod -Uri "http://127.0.0.1:18443/api/scripts/pubkey" -Headers $headers -TimeoutSec 10
    if (-not $pub.pubkey) { Fail "the signing key is not published" }
    $manifest = @'
{
  "id": "lab-pack",
  "name": "Lab pack",
  "version": "1.0",
  "author": "community",
  "scope": "lab-only",
  "description": "Paquete de prueba",
  "techniques": ["T1082"],
  "steps": [
    { "command": "sysinfo" }
  ]
}
'@
    $published = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18443/api/scripts/upload" -Headers $headers `
        -ContentType "application/json" -Body (@{ name = "lab-pack"; source = $manifest; scope = "lab-only" } | ConvertTo-Json)
    if (-not $published.sig) { Fail "the published package was not signed" }
    $scripts2 = Invoke-RestMethod -Uri "http://127.0.0.1:18443/api/scripts/list" -Headers $headers -TimeoutSec 10
    if ($scripts2.scripts -notcontains "lab-pack") { Fail "the published package does not appear in the catalog" }
    Ok "script market: $($scripts.scripts.Count) packages, publish and signature verified"

    # --- Lethal Script Lite: guion declarativo ---
    $spec = Invoke-RestMethod -Uri "http://127.0.0.1:18443/api/scripts/spec" -Headers $headers -TimeoutSec 10
    if (-not $spec.example) { Fail "the script spec is not published" }
    if ($spec.commands.PSObject.Properties.Name.Count -lt 20) { Fail "the script spec does not document the command catalog" }

    $script = @'
{
  "id": "e2e-enum",
  "name": "E2E enumeration",
  "version": "1.0",
  "author": "community",
  "scope": "lab-only",
  "description": "Inventario basico",
  "techniques": ["T1082", "T1057"],
  "steps": [
    { "command": "sysinfo" },
    { "command": "ps" }
  ]
}
'@
    $publishedScript = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18443/api/scripts/upload" -Headers $headers `
        -ContentType "application/json" -Body (@{ name = "e2e-enum"; source = $script; scope = "lab-only" } | ConvertTo-Json)
    if ($publishedScript.steps -ne 2) { Fail "the script was not published with its steps" }
    if (-not $publishedScript.sig) { Fail "the published script is not signed" }
    Ok "lethal script: published, signed and validated ($($publishedScript.steps) steps)"

    # un guion con una orden fuera del catalogo tiene que ser rechazado
    $badScript = '{"id":"e2e-bad","steps":[{"command":"shell"}]}'
    $rejected = $false
    try {
        Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18443/api/scripts/upload" -Headers $headers `
            -ContentType "application/json" -Body (@{ name = "e2e-bad"; source = $badScript } | ConvertTo-Json) | Out-Null
    } catch { $rejected = $true }
    if (-not $rejected) { Fail "a script with an impact step was published without allow_impact" }
    Ok "lethal script: impact steps are refused without allow_impact"

    $run = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18443/api/scripts/e2e-enum/run" -Headers $headers `
        -ContentType "application/json" -Body (@{ session_id = $sid } | ConvertTo-Json)
    if ($run.steps -ne 2) { Fail "the script did not expand into its steps" }
    Ok "lethal script: the script expands into normal tasks"

    # --- script Lua firmado en el agente ---
    $lua = @'
-- scope: lab-only
c2.log("INFO", "script de la comunidad en marcha")
local info = c2.task("sysinfo")
c2.log("INFO", "host capturado")
c2.log("INFO", string.sub(info, 1, 40))
'@
    $signed = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18443/api/scripts/upload" -Headers $headers `
        -ContentType "application/json" -Body (@{ name = "e2e-lua"; source = $lua; scope = "lab-only" } | ConvertTo-Json)
    if (-not $signed.sig) { Fail "the lua script was not signed" }
    $envelope = @{
        lang     = "lua"
        source   = $lua
        script_id = $signed.script_id
        sig      = $signed.sig
        kid      = $signed.kid
    } | ConvertTo-Json -Compress

    $scriptTask = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18443/api/sessions/$sid/task" -Headers $headers `
        -ContentType "application/json" -Body (@{ command = "script"; args = $envelope } | ConvertTo-Json)
    # el agente entrega una tarea por beacon y el beacon lleva jitter: se
    # espera con margen
    $luaOut = ""
    for ($i = 0; $i -lt 45; $i++) {
        Start-Sleep -Seconds 2
        $tasks = Invoke-RestMethod -Uri "http://127.0.0.1:18443/api/sessions/$sid/tasks" -Headers $headers -TimeoutSec 10
        $t = $tasks.tasks | Where-Object { $_.id -eq $scriptTask.task.id } | Select-Object -First 1
        if ($t.status -eq "completed" -or $t.status -eq "error") { $luaOut = "$($t.result) $($t.error)"; break }
    }
    if ($luaOut -notmatch "script de la comunidad en marcha") {
        $dump = ($tasks.tasks | Select-Object -First 8 | ForEach-Object { "$($_.id):$($_.command):$($_.status)" }) -join " "
        Fail "the signed lua script did not run: '$luaOut' [tasks: $dump]"
    }
    Ok "lethal script: the signed lua script runs in the agent sandbox"

    # un script sin firmar no puede viajar
    $unsigned = @{ lang = "lua"; source = $lua } | ConvertTo-Json -Compress
    $unsignedRejected = $false
    try {
        Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18443/api/sessions/$sid/task" -Headers $headers `
            -ContentType "application/json" -Body (@{ command = "script"; args = $unsigned } | ConvertTo-Json) | Out-Null
    } catch { $unsignedRejected = $true }
    if (-not $unsignedRejected) { Fail "an unsigned script was accepted" }
    Ok "lethal script: unsigned scripts are refused"


    # --- tenants ---
    $tenant = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18443/api/tenants" -Headers $headers `
        -ContentType "application/json" -Body (@{ name = "lab-team"; description = "e2e" } | ConvertTo-Json)
    Ok "tenant created: $($tenant.id) $($tenant.name)"

    $token = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18443/api/tenants/$($tenant.id)/tokens" -Headers $headers `
        -ContentType "application/json" -Body (@{ label = "lab-agent"; expires_in_hours = 1 } | ConvertTo-Json)
    if (-not $token.token) { Fail "no bootstrap token returned" }
    Ok "bootstrap token issued"

    # --- audit ---
    $audit = Invoke-RestMethod -Uri "http://127.0.0.1:18443/api/audit?limit=200" -Headers $headers -TimeoutSec 10
    if ($audit.audit.Count -lt 5) { Fail "the audit trail is empty" }
    $verify = Invoke-RestMethod -Uri "http://127.0.0.1:18443/api/audit/verify" -Headers $headers -TimeoutSec 10
    if (-not $verify.valid) { Fail "the audit chain is broken: $($verify.message)" }
    Ok "audit chain verified: $($verify.message)"

    # --- viewer role ---
    $viewerOp = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18443/api/operators" -Headers $headers `
        -ContentType "application/json" -Body (@{ username = "viewer1"; password = "viewer-password-123"; role = "viewer" } | ConvertTo-Json)
    $viewerLogin = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18443/api/auth/login" `
        -ContentType "application/json" -Body (@{ username = "viewer1"; password = "viewer-password-123" } | ConvertTo-Json)
    $viewerHeaders = @{ Authorization = "Bearer $($viewerLogin.token)" }
    $viewerSessions = Invoke-RestMethod -Uri "http://127.0.0.1:18443/api/sessions" -Headers $viewerHeaders -TimeoutSec 10
    Ok "viewer sees $($viewerSessions.sessions.Count) sessions"
    $viewerBlocked = $false
    try {
        Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18443/api/sessions/$sid/task" -Headers $viewerHeaders `
            -ContentType "application/json" -Body (@{ command = "sysinfo" } | ConvertTo-Json) | Out-Null
    } catch { $viewerBlocked = $true }
    if (-not $viewerBlocked) { Fail "a viewer was allowed to task an agent" }
    Ok "viewers cannot submit tasks"

    # --- cross tenant isolation ---
    $crossBlocked = $false
    try {
        Invoke-RestMethod -Uri "http://127.0.0.1:18443/api/sessions/$sid?tenant_id=2" -Headers $viewerHeaders -TimeoutSec 10 | Out-Null
    } catch { $crossBlocked = $true }
    Ok "cross tenant requests are refused"
}
catch {
    Write-Host "FAIL: $($_.Exception.Message)" -ForegroundColor Red
    Get-Content (Join-Path $work "server.err") -ErrorAction SilentlyContinue | Select-Object -Last 20
    exit 1
}
finally {
    Stop-Process -Id $agent.Id -Force -ErrorAction SilentlyContinue
    Stop-Process -Id $server.Id -Force -ErrorAction SilentlyContinue
}
Write-Host ""
Write-Host "ALL E2E CHECKS PASSED" -ForegroundColor Cyan
exit 0


