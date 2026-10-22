using C2Dect.Models;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using System;
using System.Collections.Generic;
using System.Linq;
using System.Net;
using System.Net.Http;
using System.Net.Http.Headers;
using System.Text;
using System.Threading;
using System.Threading.Tasks;

namespace C2Dect.Services;

/// <summary>
/// Cliente REST de la consola Lite.
///
/// Conserva los nombres y las firmas que usa la GUI completa para que el
/// portado sea un recorte fiel y no una reescritura: lo que cambia es el
/// conjunto de endpoints (solo los que sirve el servidor Lite 2.0) y el
/// alcance por tenant, que el Lite aplica de verdad.
/// </summary>
public class ApiService
{
    private HttpClient _http;
    private string? _baseUrl;
    private string? _token;
    private bool _insecure;

    public string? BaseUrl
    {
        get => _baseUrl;
        set => _baseUrl = value?.TrimEnd('/');
    }

    /// <summary>Tenant activo. Lo honra el servidor solo para role=admin.</summary>
    public long? ActiveTenantId { get; set; }

    public string? LastError { get; private set; }

    public void ClearLastError() => LastError = null;

    public ApiService()
    {
        _http = BuildClient();
    }

    private HttpClient BuildClient()
    {
        var handler = new HttpClientHandler();
        if (_insecure)
        {
            // Solo para el laboratorio: acepta certificados autofirmados.
            handler.ServerCertificateCustomValidationCallback = (_, _, _, _) => true;
        }
        var client = new HttpClient(handler) { Timeout = TimeSpan.FromSeconds(30) };
        if (!string.IsNullOrWhiteSpace(_token))
            client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", _token);
        return client;
    }

    /// <summary>Permite certificados autofirmados (laboratorio).</summary>
    public void SetInsecure(bool insecure)
    {
        if (_insecure == insecure) return;
        _insecure = insecure;
        var old = _http;
        _http = BuildClient();
        old.Dispose();
    }

    public void Configure(string baseUrl, string token)
    {
        _baseUrl = baseUrl.TrimEnd('/');
        _token = token;
        _http.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", token);
    }

    public void ClearToken()
    {
        _token = null;
        _http.DefaultRequestHeaders.Authorization = null;
    }

    public bool IsConfigured => !string.IsNullOrWhiteSpace(_baseUrl) && !string.IsNullOrWhiteSpace(_token);

    public async Task<bool> CheckHealthAsync(CancellationToken ct = default)
    {
        if (string.IsNullOrWhiteSpace(_baseUrl)) return false;
        try
        {
            using var resp = await _http.GetAsync(Url("/api/health"), ct);
            return resp.IsSuccessStatusCode;
        }
        catch
        {
            return false;
        }
    }

    // ------------------------------------------------------------- helpers

    private string Url(string path) => $"{_baseUrl}{path}";

    /// <summary>AÃ±ade el tenant activo a la ruta si el servidor lo honra.</summary>
    private string WithTenant(string path)
    {
        if (ActiveTenantId is not long tid || tid <= 0) return path;
        var separator = path.Contains('?') ? "&" : "?";
        return $"{path}{separator}tenant_id={tid}";
    }

    public string ScopedSessionsPath() => WithTenant("/api/sessions");

    private async Task<T?> Get<T>(string path)
    {
        try
        {
            using var resp = await _http.GetAsync(Url(path));
            if (!resp.IsSuccessStatusCode)
            {
                LastError = $"{resp.StatusCode}: {await ErrorBody(resp)}";
                return default;
            }
            var json = await resp.Content.ReadAsStringAsync();
            LastError = null;
            return JsonConvert.DeserializeObject<T>(json);
        }
        catch (Exception ex)
        {
            LastError = ex.Message;
            return default;
        }
    }

    public async Task<ApiResult<T>> GetResultAsync<T>(string path, TimeSpan? timeout = null)
    {
        try
        {
            using var cts = timeout is null ? null : new CancellationTokenSource(timeout.Value);
            using var resp = await _http.GetAsync(Url(path), cts?.Token ?? default);
            var body = await resp.Content.ReadAsStringAsync();
            if (!resp.IsSuccessStatusCode)
            {
                LastError = $"{resp.StatusCode}: {body}";
                return ApiResult<T>.Failure((int)resp.StatusCode, body);
            }
            LastError = null;
            var value = JsonConvert.DeserializeObject<T>(body);
            return ApiResult<T>.Success(value!);
        }
        catch (Exception ex)
        {
            LastError = ex.Message;
            return ApiResult<T>.Failure(0, ex.Message);
        }
    }

    private async Task<T?> Post<T>(string path, object? body)
    {
        try
        {
            using var content = new StringContent(JsonConvert.SerializeObject(body), Encoding.UTF8, "application/json");
            using var resp = await _http.PostAsync(Url(path), content);
            if (!resp.IsSuccessStatusCode)
            {
                LastError = $"{resp.StatusCode}: {await ErrorBody(resp)}";
                return default;
            }
            var json = await resp.Content.ReadAsStringAsync();
            LastError = null;
            return JsonConvert.DeserializeObject<T>(json);
        }
        catch (Exception ex)
        {
            LastError = ex.Message;
            return default;
        }
    }

    private async Task<T?> Put<T>(string path, object body)
    {
        try
        {
            using var content = new StringContent(JsonConvert.SerializeObject(body), Encoding.UTF8, "application/json");
            using var resp = await _http.PutAsync(Url(path), content);
            if (!resp.IsSuccessStatusCode)
            {
                LastError = $"{resp.StatusCode}: {await ErrorBody(resp)}";
                return default;
            }
            var json = await resp.Content.ReadAsStringAsync();
            LastError = null;
            return JsonConvert.DeserializeObject<T>(json);
        }
        catch (Exception ex)
        {
            LastError = ex.Message;
            return default;
        }
    }

    private async Task<bool> Delete(string path)
    {
        try
        {
            using var resp = await _http.DeleteAsync(Url(path));
            if (!resp.IsSuccessStatusCode)
            {
                LastError = $"{resp.StatusCode}: {await ErrorBody(resp)}";
                return false;
            }
            LastError = null;
            return true;
        }
        catch (Exception ex)
        {
            LastError = ex.Message;
            return false;
        }
    }

    private static async Task<string> ErrorBody(HttpResponseMessage resp)
    {
        try
        {
            var body = await resp.Content.ReadAsStringAsync();
            try
            {
                var parsed = JObject.Parse(body);
                var error = parsed["error"]?.ToString();
                if (!string.IsNullOrEmpty(error)) return error;
            }
            catch { }
            return body.Length > 200 ? body[..200] : body;
        }
        catch
        {
            return resp.StatusCode.ToString();
        }
    }

    // ---------------------------------------------------------------- auth

    public async Task<(string token, string refreshToken, Operator? op)?> Login(string username, string password, string? totpCode = null)
    {
        var payload = new Dictionary<string, object?> { ["username"] = username, ["password"] = password };
        if (!string.IsNullOrWhiteSpace(totpCode)) payload["totp_code"] = totpCode;

        try
        {
            using var content = new StringContent(JsonConvert.SerializeObject(payload), Encoding.UTF8, "application/json");
            using var resp = await _http.PostAsync(Url("/api/auth/login"), content);
            var body = await resp.Content.ReadAsStringAsync();
            if (!resp.IsSuccessStatusCode)
            {
                LastError = ExtractLoginError(body);
                return null;
            }
            var json = JObject.Parse(body);
            LastError = null;
            return (
                json["token"]?.ToString() ?? "",
                json["refresh_token"]?.ToString() ?? "",
                json["operator"]?.ToObject<Operator>());
        }
        catch (Exception ex)
        {
            LastError = ex.Message;
            return null;
        }
    }

    private static string ExtractLoginError(string body)
    {
        try
        {
            var parsed = JObject.Parse(body);
            return parsed["error"]?.ToString() ?? body;
        }
        catch
        {
            return body;
        }
    }

    public async Task<string?> Refresh(string refreshToken)
    {
        var result = await Post<JObject>("/api/auth/refresh", new { refresh_token = refreshToken });
        return result?["access_token"]?.ToString();
    }

    public async Task<bool> Logout(string refreshToken)
    {
        var result = await Post<JObject>("/api/auth/logout", new { refresh_token = refreshToken });
        return result != null;
    }

    public async Task<MeEnvelope?> GetMe() => await Get<MeEnvelope>("/api/me");

    // ------------------------------------------------------------ sessions

    public async Task<List<Session>?> GetSessions(string? status = null)
    {
        var path = WithTenant("/api/sessions");
        if (!string.IsNullOrEmpty(status)) path += (path.Contains('?') ? "&" : "?") + "status=" + status;
        var envelope = await Get<SessionsEnvelope>(path);
        return envelope?.sessions;
    }

    public async Task<Session?> GetSessionDetailAsync(string id) => await Get<Session>($"/api/sessions/{id}");

    public async Task<bool> TagSession(string id, string tags)
        => await Put<JObject>($"/api/sessions/{id}/tag", new { tags }) != null;

    public async Task<bool> SetSleep(string id, int sec)
        => await Put<JObject>($"/api/sessions/{id}/sleep", new { sleep_sec = sec }) != null;

    public async Task<bool> KillSession(string id) => await Delete($"/api/sessions/{id}");

    public async Task<bool> DeleteSessionAsync(string id) => await Delete($"/api/sessions/{id}");

    /// <summary>
    /// Encola una orden. Las ordenes de nivel de impacto necesitan el token de
    /// confirmacion; el operador lo marca con la casilla de consentimiento.
    /// </summary>
    public async Task<C2Task?> SubmitTask(string sessionId, string command, string args = "", string confirm = "")
    {
        var envelope = await Post<TaskEnvelope>($"/api/sessions/{sessionId}/task", new { command, args, confirm });
        return envelope?.task;
    }

    /// <summary>Catalogo de ordenes con su nivel de impacto.</summary>
    public async Task<List<CommandSpec>?> GetCommands()
    {
        var resp = await Get<CommandsResponse>("/api/commands");
        return resp?.commands;
    }

    public async Task<List<C2Task>?> GetTasks(string sessionId)
    {
        var envelope = await Get<TasksEnvelope>($"/api/sessions/{sessionId}/tasks");
        return envelope?.tasks;
    }

    public async Task<C2Task?> GetTaskDetailAsync(string sessionId, long taskId)
        => await Get<C2Task>($"/api/sessions/{sessionId}/tasks/{taskId}");

    public async Task<bool> UploadFile(string sessionId, string remotePath, byte[] data)
    {
        try
        {
            using var content = new MultipartFormDataContent();
            content.Add(new ByteArrayContent(data), "file", System.IO.Path.GetFileName(remotePath));
            content.Add(new StringContent(remotePath), "remote_path");
            using var resp = await _http.PostAsync(Url($"/api/sessions/{sessionId}/upload"), content);
            if (!resp.IsSuccessStatusCode)
            {
                LastError = $"{resp.StatusCode}: {await ErrorBody(resp)}";
                return false;
            }
            LastError = null;
            return true;
        }
        catch (Exception ex)
        {
            LastError = ex.Message;
            return false;
        }
    }

    public async Task<List<Loot>?> GetLoot(string? sessionId = null)
    {
        var path = WithTenant("/api/loot");
        if (!string.IsNullOrEmpty(sessionId)) path += (path.Contains('?') ? "&" : "?") + "session_id=" + sessionId;
        var envelope = await Get<LootEnvelope>(path);
        return envelope?.loot;
    }

    public async Task<bool> SaveLootAsync(string sessionId, string type, string name, string path, byte[]? data, string? hash)
        => await Post<JObject>("/api/loot", new { session_id = sessionId, type, name, path, data, hash }) != null;

    public async Task<DashboardStats?> GetStats() => await Get<DashboardStats>("/api/stats");

    public async Task<string> DeleteAllSessions() => await Delete("/api/sessions") ? "ok" : "";

    public async Task<string> CleanupStaleSessions(int maxAgeMinutes)
    {
        var result = await Post<JObject>("/api/sessions/cleanup", new { max_age_minutes = maxAgeMinutes });
        return result?.ToString() ?? "";
    }

    // ------------------------------------------------------------ profiles

    public async Task<List<MalleableProfile>?> GetProfiles()
    {
        var envelope = await Get<ProfilesEnvelope>("/api/profiles");
        return envelope?.profiles;
    }

    public async Task<MalleableProfile?> CreateProfile(MalleableProfile profile)
        => await Post<MalleableProfile>("/api/profiles", profile);

    // --------------------------------------------------------------- audit

    public async Task<List<AuditRow>?> GetAudit(int limit = 200)
    {
        var envelope = await Get<AuditEnvelope>($"/api/audit?limit={limit}");
        return envelope?.audit;
    }

    public async Task<AuditVerifyResult?> VerifyAuditChainAsync()
        => await Get<AuditVerifyResult>("/api/audit/verify");

    // ------------------------------------------------------------- tenants

    public async Task<List<Tenant>?> GetTenants()
    {
        var envelope = await Get<TenantEnvelope>("/api/tenants");
        return envelope?.tenants;
    }

    public async Task<Tenant?> CreateTenant(string name)
        => await Post<Tenant>("/api/tenants", new { name });

    public async Task<bool> DeleteTenant(long id) => await Delete($"/api/tenants/{id}");

    public async Task<List<Operator>?> GetUsersByTenant(long tenantId)
    {
        var envelope = await Get<OperatorsEnvelope>("/api/operators");
        return envelope?.operators;
    }

    public async Task<bool> CreateUserTenant(string username, string password, long tenantId, List<long> roleIds, bool disposable = false)
        => await Post<JObject>("/api/operators", new
        {
            username,
            password,
            tenant_id = tenantId,
            role = roleIds.Count > 0 ? "operator" : "viewer",
            disposable
        }) != null;

    public async Task<List<AgentTokenRow>?> GetAgentTokens(long tenantId)
    {
        var envelope = await Get<TokensEnvelope>($"/api/tenants/{tenantId}/tokens");
        return envelope?.tokens;
    }

    public async Task<AgentTokenCreated?> CreateAgentToken(long tenantId, string? label = null, int hours = 24)
        => await Post<AgentTokenCreated>($"/api/tenants/{tenantId}/tokens", new { label, expires_in_hours = hours });

    public async Task<bool> RevokeAgentToken(long tenantId, long tokenId)
        => await Delete($"/api/tenants/{tenantId}/tokens/{tokenId}");

    // ---------------------------------------------------------- marketplace

    public async Task<List<ScriptEntry>> GetScriptListAsync()
    {
        var envelope = await Get<ScriptListEnvelope>("/api/scripts/list");
        var scripts = new List<ScriptEntry>();
        if (envelope?.Scripts == null) return scripts;
        foreach (var name in envelope.Scripts)
        {
            scripts.Add(new ScriptEntry { Name = name });
        }
        return scripts;
    }

    public async Task<string?> GetScriptAsync(string name)
    {
        var detail = await Get<ScriptDetail>($"/api/scripts/get?name={Uri.EscapeDataString(name)}");
        return detail?.Source;
    }

    public async Task<string?> GetScriptPubkeyAsync()
    {
        var key = await Get<ScriptPubkey>("/api/scripts/pubkey");
        return key?.Pubkey;
    }


    public async Task<bool> ScriptUploadAsync(string name, string source, string? scope = null)
    {
        var result = await Post<JObject>("/api/scripts/upload", new { name, source, scope });
        return result != null;
    }

    /// <summary>Publica un script y devuelve la firma del servidor.</summary>
    public async Task<PublishedScript?> PublishScript(string name, string source, string? scope = null)
    {
        var result = await Post<PublishedScript>("/api/scripts/upload", new { name, source, scope });
        if (result == null) return null;
        return new PublishedScript
        {
            Name = result.Name,
            ScriptId = result.ScriptId,
            Signature = result.Signature,
            Kid = result.Kid,
            Scope = result.Scope,
            Steps = result.Steps,
            Kind = result.Kind,
        };
    }

    /// <summary>Ejecuta un guion publicado contra una sesion.</summary>
    public async Task<ScriptRunResult?> RunScript(string id, string sessionId, string confirm = "")
        => await Post<ScriptRunResult>($"/api/scripts/{id}/run", new { session_id = sessionId, confirm });

    /// <summary>Especificacion del formato de los guiones y sus limites.</summary>
    public async Task<ScriptSpec?> GetScriptSpec() => await Get<ScriptSpec>("/api/scripts/spec");

    /// <summary>Estado de confianza de un guion.</summary>
    public async Task<ScriptTrustInfo?> GetScriptTrustAsync(string name)
    {
        var detail = await Get<ScriptDetail>($"/api/scripts/get?name={Uri.EscapeDataString(name)}");
        if (detail == null) return null;
        var key = await Get<ScriptPubkey>("/api/scripts/pubkey");

        var signed = !string.IsNullOrEmpty(detail.Signature)
            && !string.IsNullOrEmpty(detail.Kid)
            && !string.IsNullOrEmpty(key?.Kid)
            && string.Equals(detail.Kid, key.Kid, StringComparison.OrdinalIgnoreCase);

        var verdict = signed ? "FIRMADO" : "SIN_FIRMA";
        var reason = signed
            ? $"Firmado con la clave activa (kid {detail.Kid})."
            : "El paquete no viene firmado por la clave activa del servidor.";

        return new ScriptTrustInfo
        {
            Name = detail.Name,
            Signed = signed,
            Kid = detail.Kid ?? "",
            Scope = detail.Scope ?? "",
            ScriptId = detail.ScriptId ?? "",
            TenantId = key?.TenantId ?? 0,
            TrustedKids = key?.Prev?.Select(p => p.Kid ?? "").ToList() ?? new List<string>(),
            Verdict = verdict,
            Reason = reason,
        };
    }

    // ------------------------------------------------------------- modules

    public async Task<ModuleCatalogResponse?> GetModuleCatalogAsync()
        => await Get<ModuleCatalogResponse>("/api/modules");

    // -------------------------------------------------------------- extras

    /// <summary>Espacio de trabajo: filtro por etiqueta de sesiÃ³n.</summary>
    public async Task<bool> SetWorkspace(string sessionId, string workspace)
        => await Put<JObject>($"/api/sessions/{sessionId}/tag", new { tags = "ws:" + workspace }) != null;

    public async Task<List<Session>?> GetSessionsByWorkspace(string workspace)
    {
        var all = await GetSessions();
        if (all == null) return null;
        return all.Where(s => (s.Tags ?? "").Contains("ws:" + workspace, StringComparison.OrdinalIgnoreCase)).ToList();
    }

    // ------------------------------------------------------------ envelopes

    private sealed class SessionsEnvelope
    {
        [JsonProperty("sessions")] public List<Session>? sessions { get; set; }
    }

    private sealed class TasksEnvelope
    {
        [JsonProperty("tasks")] public List<C2Task>? tasks { get; set; }
    }

    private sealed class TaskEnvelope
    {
        [JsonProperty("task")] public C2Task? task { get; set; }
    }

    private sealed class CommandsResponse
    {
        [JsonProperty("commands")] public List<CommandSpec>? commands { get; set; }
        [JsonProperty("confirm_token")] public string? confirm_token { get; set; }
    }

    /// <summary>Resultado de ejecutar un guion.</summary>
    public sealed class ScriptRunResult
    {
        [JsonProperty("status")] public string? status { get; set; }
        [JsonProperty("script")] public string? script { get; set; }
        [JsonProperty("steps")] public int steps { get; set; }
        [JsonProperty("session")] public string? session { get; set; }
        [JsonProperty("script_id")] public string? script_id { get; set; }
    }

    /// <summary>Especificacion del formato de los guiones.</summary>
    public sealed class ScriptSpec
    {
        [JsonProperty("spec")] public string? spec { get; set; }
        [JsonProperty("example")] public string? example { get; set; }
        [JsonProperty("limits")] public Dictionary<string, object>? limits { get; set; }
        [JsonProperty("commands")] public List<CommandSpec>? commands { get; set; }
    }

    /// <summary>Script publicado y firmado por el servidor.</summary>
    public sealed class PublishedScript
    {
        [JsonProperty("name")] public string? Name { get; set; }
        [JsonProperty("script_id")] public string? ScriptId { get; set; }
        [JsonProperty("sig")] public string? Signature { get; set; }
        [JsonProperty("kid")] public string? Kid { get; set; }
        [JsonProperty("scope")] public string? Scope { get; set; }
        [JsonProperty("steps")] public int Steps { get; set; }
        [JsonProperty("kind")] public string? Kind { get; set; }
    }

    private sealed class LootEnvelope
    {
        [JsonProperty("loot")] public List<Loot>? loot { get; set; }
    }

    private sealed class ProfilesEnvelope
    {
        [JsonProperty("profiles")] public List<MalleableProfile>? profiles { get; set; }
    }

    private sealed class TenantEnvelope
    {
        [JsonProperty("tenants")] public List<Tenant>? tenants { get; set; }
    }

    private sealed class OperatorsEnvelope
    {
        [JsonProperty("operators")] public List<Operator>? operators { get; set; }
    }

    private sealed class TokensEnvelope
    {
        [JsonProperty("tokens")] public List<AgentTokenRow>? tokens { get; set; }
    }

    private sealed class AuditEnvelope
    {
        [JsonProperty("audit")] public List<AuditRow>? audit { get; set; }
    }

    /// <summary>Identidad del operador con la que se abre la consola.</summary>
    public sealed class MeEnvelope
    {
        [JsonProperty("operator")] public Operator? op { get; set; }
        [JsonProperty("tenant")] public Tenant? tenant { get; set; }
        [JsonProperty("permissions")] public List<string>? permissions { get; set; }
        [JsonProperty("version")] public string? version { get; set; }
    }

    /// <summary>Fila de la traza de auditoria.</summary>
    public sealed class AuditRow
    {
        [JsonProperty("id")] public long id { get; set; }
        [JsonProperty("ts")] public DateTime ts { get; set; }
        [JsonProperty("tenant_id")] public long tenant_id { get; set; }
        [JsonProperty("operator_id")] public long operator_id { get; set; }
        [JsonProperty("username")] public string? username { get; set; }
        [JsonProperty("category")] public string? category { get; set; }
        [JsonProperty("action")] public string? action { get; set; }
        [JsonProperty("severity")] public string? severity { get; set; }
        [JsonProperty("details")] public string? details { get; set; }
        [JsonProperty("prev_hash")] public string? prev_hash { get; set; }
        [JsonProperty("hash")] public string? hash { get; set; }
    }

    /// <summary>Resultado de verificar la cadena de auditoria.</summary>
    public sealed class AuditVerifyResult
    {
        [JsonProperty("valid")] public bool valid { get; set; }
        [JsonProperty("status")] public string? status { get; set; }
        [JsonProperty("message")] public string? message { get; set; }
    }

    /// <summary>Token de arranque de agente.</summary>
    public sealed class AgentTokenRow
    {
        [JsonProperty("id")] public long id { get; set; }
        [JsonProperty("label")] public string? label { get; set; }
        [JsonProperty("tenant_id")] public long tenant_id { get; set; }
        [JsonProperty("expires_at")] public string? expires_at { get; set; }
        [JsonProperty("revoked")] public bool revoked { get; set; }
        [JsonProperty("created_at")] public string? created_at { get; set; }
    }

    /// <summary>Token de arranque recien emitido.</summary>
    public sealed class AgentTokenCreated
    {
        [JsonProperty("token")] public string? token { get; set; }
        [JsonProperty("tenant_id")] public long tenant_id { get; set; }
        [JsonProperty("expires_at")] public string? expires_at { get; set; }
    }
}

