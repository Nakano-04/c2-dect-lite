using Newtonsoft.Json;
using System.Collections.Generic;

namespace C2Dect.Models;

// ============================================================================
// Modelos de respuesta de la API C2 (Ronda 1 - capa de servicios).
//
// Todos los shapes estan copiados del servidor Go (server/api/*.go); no se
// inventan campos: lo que el servidor no devuelve se queda en null/0.
//
// REGLA DE DESERIALIZACION (misma que Models.cs y TenantAccess.cs):
// el servidor Go serializa en snake_case y Newtonsoft solo casa por nombre
// ignorando mayusculas, NO los guiones bajos. Por eso cada campo que llega
// con guion bajo lleva su [JsonProperty] explicito.
//
// Donde la forma exacta depende de lo que el servidor decida (ej. el
// marketplace de scripts devuelve solo nombres en el listado, pero nombre +
// scope + firma al pedir un script), el campo opcional queda null y lo
// rellena quien tenga el dato completo.
// ============================================================================

// ---------------------------------------------------------------------------
// IA Copilot (server/api/ia_copilot.go)
// ---------------------------------------------------------------------------

/// <summary>Una tecnica sugerida por el copiloto (POST /api/ia/suggest).</summary>
public sealed class IaSuggestion
{
    [JsonProperty("technique")] public string? Technique { get; set; }
    [JsonProperty("why")] public string? Why { get; set; }
    [JsonProperty("risk")] public string? Risk { get; set; }
}

/// <summary>POST /api/ia/suggest -> {"suggestions":[{technique,why,risk}, ...]}.</summary>
public sealed class IaSuggestResult
{
    [JsonProperty("suggestions")] public List<IaSuggestion>? Suggestions { get; set; }
}

/// <summary>POST /api/ia/generate-script -> {"source": "...", "validated": true}.</summary>
public sealed class IaScriptResult
{
    [JsonProperty("source")] public string? Source { get; set; }
    [JsonProperty("validated")] public bool Validated { get; set; }
}

/// <summary>POST /api/ia/sigma -> {"technique": "T1003", "sigma": "&lt;regla yaml&gt;"}.</summary>
public sealed class IaSigmaResult
{
    [JsonProperty("technique")] public string? Technique { get; set; }
    [JsonProperty("sigma")] public string? Sigma { get; set; }
}

// ---------------------------------------------------------------------------
// Marketplace de scripts (server/api/scripts_marketplace.go)
// ---------------------------------------------------------------------------

/// <summary>
/// Entrada del marketplace. GET /api/scripts/list solo devuelve nombres
/// ({"scripts":[...], "count":n}); el scope, la firma y el kid solo se
// rellenan con el detalle (GET /api/scripts/get?name=), por eso son
/// opcionales.
/// </summary>
public sealed class ScriptEntry
{
    [JsonProperty("name")] public string? Name { get; set; }
    [JsonProperty("scope")] public string? Scope { get; set; }
    [JsonProperty("script_id")] public string? ScriptId { get; set; }
    /// <summary>Firma Ed25519 en hex (solo si el servidor tiene SCRIPT_SIGN_KEY).</summary>
    [JsonProperty("sig")] public string? Signature { get; set; }
    /// <summary>kid de la clave activa (hex(sha256(pub))[:16]).</summary>
    [JsonProperty("kid")] public string? Kid { get; set; }
}

/// <summary>GET /api/scripts/list -> {"scripts":[...], "count":n}.</summary>
public sealed class ScriptListEnvelope
{
    [JsonProperty("scripts")] public List<string>? Scripts { get; set; }
    [JsonProperty("count")] public int Count { get; set; }
}

/// <summary>GET /api/scripts/get?name= -> {name, source, scope, sig?, kid?}.</summary>
public sealed class ScriptDetail
{
    [JsonProperty("name")] public string? Name { get; set; }
    [JsonProperty("source")] public string? Source { get; set; }
    [JsonProperty("scope")] public string? Scope { get; set; }
    [JsonProperty("script_id")] public string? ScriptId { get; set; }
    [JsonProperty("sig")] public string? Signature { get; set; }
    [JsonProperty("kid")] public string? Kid { get; set; }
}

/// <summary>GET /api/scripts/pubkey -> {pubkey, kid, prev?}.</summary>
public sealed class ScriptPubkey
{
    [JsonProperty("pubkey")] public string? Pubkey { get; set; }
    [JsonProperty("kid")] public string? Kid { get; set; }
    [JsonProperty("tenant_id")] public long TenantId { get; set; }
    [JsonProperty("tenant_keys")] public List<ScriptKey>? TenantKeys { get; set; }
    /// <summary>Claves PREVIAS de la rotacion (SCRIPT_SIGN_KEYS_PREV).</summary>
    [JsonProperty("prev")] public List<ScriptKey>? Prev { get; set; }
}

/// <summary>
/// Veredicto de CONFIANZA de un script del marketplace.
///
/// POR QUE existe: el servidor Go firma los scripts y exige que la firma sea
/// de una clave que el TENANT del llamante acepta
/// (server/api/script_trust.go:verifyScriptTaskForSubmit), pero no expone
/// ningun endpoint que diga "este script concreto esta firmado / es de otro
/// tenant / se queda fuera de scope". La GUI lo deduce de lo que el servidor
/// SI expone (GET /api/scripts/get y GET /api/scripts/pubkey) aplicando la
/// MISMA politica del Go.
///
/// LIMITACION HONESTA: la GUI no verifica la firma criptograficamente (el
/// servidor la firma y la publica). "FIRMADO" significa "el servidor ha
/// adjuntado una firma"; el veredicto de tenant se basa en el `kid` que
/// devuelve el servidor, no en una comprobacion de la firma.
/// </summary>
public sealed class ScriptTrustInfo
{
    [JsonProperty("name")] public string? Name { get; set; }
    /// <summary>El servidor ha adjuntado firma (campo "sig" del detalle).</summary>
    [JsonProperty("signed")] public bool Signed { get; set; }
    /// <summary>kid de la clave con la que se firmo.</summary>
    [JsonProperty("kid")] public string? Kid { get; set; }
    /// <summary>Alcance declarado por el autor (cabecera "-- scope:").</summary>
    [JsonProperty("scope")] public string? Scope { get; set; }
    [JsonProperty("script_id")] public string? ScriptId { get; set; }
    /// <summary>Tenant cuyas claves de confianza rigen para el llamante.</summary>
    [JsonProperty("tenant_id")] public long TenantId { get; set; }
    /// <summary>kids que el tenant del llamante acepta (claves + rotacion).</summary>
    [JsonProperty("trusted_kids")] public List<string>? TrustedKids { get; set; }
    /// <summary>Veredicto: FIRMADO | SIN_FIRMA | FUERA_DE_SCOPE | TENANT_INCORRECTO | DESCONOCIDO.</summary>
    [JsonProperty("verdict")] public string? Verdict { get; set; }
    /// <summary>Motivo del veredicto (texto explicito, sin reinterpretar).</summary>
    [JsonProperty("reason")] public string? Reason { get; set; }
    /// <summary>El servidor tiene firma configurada (SCRIPT_SIGN_KEY o claves del tenant).</summary>
    [JsonProperty("signing_available")] public bool SigningAvailable { get; set; }
}

// ---------------------------------------------------------------------------
// Metricas y reporte ejecutivo (server/api/metrics_reports.go)
// ---------------------------------------------------------------------------

/// <summary>GET /api/metrics -> metricas operativas del servidor.</summary>
public sealed class ServerMetrics
{
    [JsonProperty("sessions_total")] public int SessionsTotal { get; set; }
    [JsonProperty("sessions_active")] public int SessionsActive { get; set; }
    /// <summary>Permenencia media de la sesion (first_checkin -> last_checkin).
    /// NO es MTTD de deteccion: el MTTD real sale del runner de Atomic Red Team.</summary>
    [JsonProperty("session_dwell_seconds")] public double SessionDwellSeconds { get; set; }
    [JsonProperty("session_ttd_seconds")] public double SessionTtdSeconds { get; set; }
    [JsonProperty("success_rate")] public double SuccessRate { get; set; }
    /// <summary>Conteo por comando/tecnica (activity_log).</summary>
    [JsonProperty("by_technique")] public Dictionary<string, int>? ByTechnique { get; set; }
    [JsonProperty("generated_at")] public string? GeneratedAt { get; set; }
}

/// <summary>Una seccion del reporte ejecutivo (titulo + cuerpo).</summary>
public sealed class ExecutiveSection
{
    [JsonProperty("title")] public string? Title { get; set; }
    [JsonProperty("body")] public string? Body { get; set; }
}

/// <summary>GET /api/reports/executive -> resumen + mitre + secciones + html.</summary>
public sealed class ExecutiveReport
{
    [JsonProperty("session_id")] public string? SessionId { get; set; }
    [JsonProperty("sessions")] public int Sessions { get; set; }
    [JsonProperty("events")] public int Events { get; set; }
    [JsonProperty("mitre")] public Dictionary<string, int>? Mitre { get; set; }
    [JsonProperty("sections")] public List<ExecutiveSection>? Sections { get; set; }
    /// <summary>HTML listo para imprimir a PDF (sin dependencias externas).</summary>
    [JsonProperty("html")] public string? Html { get; set; }
}

// ---------------------------------------------------------------------------
// Operations: backup, kill-all, auditoria (server/api/backup_handlers.go,
// server/api/killswitch.go)
// ---------------------------------------------------------------------------

/// <summary>POST /api/backup -> {"backup":"&lt;base64&gt;", "sha256":"...", "size":n}.</summary>
public sealed class BackupResult
{
    /// <summary>Artefacto cifrado AES-256-GCM en base64 (es lo que se guarda en disco).</summary>
    [JsonProperty("backup")] public string? Backup { get; set; }
    [JsonProperty("sha256")] public string? Sha256 { get; set; }
    [JsonProperty("size")] public long Size { get; set; }
}

/// <summary>
/// POST /api/backup/verify -> {"ok":true, "sha256":"...", "size":n} o
/// {"ok":false, "bad_id":&lt;id&gt;, "sha256":"..."}. bad_id es el id de la
/// primera actividad manipulada de la cadena (server/db/backup.go:131).
/// </summary>
public sealed class BackupVerifyResult
{
    [JsonProperty("ok")] public bool Ok { get; set; }
    [JsonProperty("bad_id")] public long? BadId { get; set; }
    [JsonProperty("sha256")] public string? Sha256 { get; set; }
    [JsonProperty("size")] public long Size { get; set; }
}

/// <summary>
/// Respuesta del kill-switch global. El servidor usa dos formas:
///   - POST /api/agents/kill-all/approve -> {approval_token, scope_hash, expires_at}
///   - POST /api/agents/kill-all         -> {sent, failed, reached, unreachable, total}
/// Se modelan en la misma clase para que la vista de Operations pueda usar
/// un solo tipo; los contadores quedan a 0 en la respuesta de aprobacion y
/// el token queda vacio en la de ejecucion.
/// </summary>
public sealed class KillAllResult
{
    [JsonProperty("sent")] public int Sent { get; set; }
    [JsonProperty("failed")] public int Failed { get; set; }
    [JsonProperty("reached")] public int Reached { get; set; }
    [JsonProperty("unreachable")] public int Unreachable { get; set; }
    [JsonProperty("total")] public int Total { get; set; }
    /// <summary>Token single-use devuelto UNA vez por /approve (quema: el token).</summary>
    [JsonProperty("approval_token")] public string? ApprovalToken { get; set; }
    /// <summary>Hash de alcance calculado por el servidor (el cliente no lo decide).</summary>
    [JsonProperty("scope_hash")] public string? ScopeHash { get; set; }
    [JsonProperty("expires_at")] public string? ExpiresAt { get; set; }
}

/// <summary>GET /api/activity/verify -> {"ok":true} o {"ok":false,"bad_id":n}.</summary>
public sealed class AuditVerifyResult
{
    [JsonProperty("ok")] public bool Ok { get; set; }
    /// <summary>Id de la primera entrada manipulada de la cadena de auditoria.</summary>
    [JsonProperty("bad_id")] public long? BadId { get; set; }
}

// ---------------------------------------------------------------------------
// Tenancy: join tokens, script keys (server/api/tenant_tokens.go,
// server/api/script_trust.go)
// ---------------------------------------------------------------------------

/// <summary>
/// db.TenantJoinToken (server/db/tenant_tokens.go:30). El token en claro
/// solo se devuelve en el 201 de creacion; el listado trae el prefijo.
/// </summary>
public sealed class JoinToken
{
    [JsonProperty("id")] public long Id { get; set; }
    [JsonProperty("tenant_id")] public long TenantId { get; set; }
    [JsonProperty("label")] public string? Label { get; set; }
    /// <summary>Prefijo del token (el resto nunca se vuelve a exponer).</summary>
    [JsonProperty("prefix")] public string? Prefix { get; set; }
    [JsonProperty("revoked")] public bool Revoked { get; set; }
    [JsonProperty("created_at")] public string? CreatedAt { get; set; }
    /// <summary>Token en claro: solo en la respuesta 201 de POST /join-tokens.</summary>
    [JsonProperty("token")] public string? Token { get; set; }
}

/// <summary>GET /api/tenants/:id/join-tokens -> {"tokens":[...], "count":n, "tenant_id":n}.</summary>
public sealed class JoinTokenListEnvelope
{
    [JsonProperty("tokens")] public List<JoinToken>? Tokens { get; set; }
    [JsonProperty("count")] public int Count { get; set; }
    [JsonProperty("tenant_id")] public long TenantId { get; set; }
}

/// <summary>
/// db.TenantScriptKey (server/db/script_trust.go:20): clave publica Ed25519
/// que el servidor acepta para el tenant (rotacion de la clave de firma).
/// </summary>
public sealed class ScriptKey
{
    [JsonProperty("kid")] public string? Kid { get; set; }
    [JsonProperty("pubkey")] public string? Pubkey { get; set; }
    [JsonProperty("created_at")] public string? CreatedAt { get; set; }
    [JsonProperty("tenant_id")] public long TenantId { get; set; }
}

/// <summary>GET /api/tenants/:id/script-keys -> {"keys":[...], "count":n, "tenant_id":n}.</summary>
public sealed class ScriptKeyListEnvelope
{
    [JsonProperty("keys")] public List<ScriptKey>? Keys { get; set; }
    [JsonProperty("count")] public int Count { get; set; }
    [JsonProperty("tenant_id")] public long TenantId { get; set; }
}

// ---------------------------------------------------------------------------
// Modulos ofensivos (server/api/offensive_modules.go y wrapper webexploits)
// ---------------------------------------------------------------------------

/// <summary>Resultado de una funcion de un modulo ofensivo.</summary>
public sealed class OffensiveCallResult
{
    [JsonProperty("function")] public string? Function { get; set; }
    /// <summary>Grado de implementacion declarado por el servidor (real/partial/stub).</summary>
    [JsonProperty("implementation")] public string? Implementation { get; set; }
    /// <summary>ok | error | skipped.</summary>
    [JsonProperty("status")] public string? Status { get; set; }
    [JsonProperty("output")] public string? Output { get; set; }
    [JsonProperty("error")] public string? Error { get; set; }
    [JsonProperty("note")] public string? Note { get; set; }
}

/// <summary>
/// Envelope comun de GET /api/modules/:module
/// ({"module","implementation","warning","count","results":[...]}).
/// </summary>
public sealed class OffensiveModuleResult
{
    [JsonProperty("module")] public string? Module { get; set; }
    /// <summary>Grado de implementacion global del modulo.</summary>
    [JsonProperty("implementation")] public string? Implementation { get; set; }
    /// <summary>Aviso de seguridad (donde se ejecuta, que es stub, etc.).</summary>
    [JsonProperty("warning")] public string? Warning { get; set; }
    [JsonProperty("count")] public int Count { get; set; }
    [JsonProperty("results")] public List<OffensiveCallResult>? Results { get; set; }
}

// ---------------------------------------------------------------------------
// Arquetipos / bindings (server/api/archetypes.go, server/db/database.go)
// ---------------------------------------------------------------------------

/// <summary>
/// db.Binding (server/db/database.go:1469): ata un operador a un arquetipo en
/// un segmento/engagement con ventana de validez. Sin binding valido no hay
/// segmento (fail-closed en el servidor).
/// </summary>
public sealed class BindingItem
{
    [JsonProperty("id")] public long Id { get; set; }
    [JsonProperty("operator_id")] public long OperatorId { get; set; }
    [JsonProperty("archetype")] public string? Archetype { get; set; }
    [JsonProperty("segment")] public string? Segment { get; set; }
    [JsonProperty("engagement")] public string? Engagement { get; set; }
    [JsonProperty("scope_json")] public string? ScopeJson { get; set; }
    [JsonProperty("valid_from")] public string? ValidFrom { get; set; }
    [JsonProperty("valid_until")] public string? ValidUntil { get; set; }
    [JsonProperty("created_by")] public string? CreatedBy { get; set; }
    [JsonProperty("created_at")] public string? CreatedAt { get; set; }

    /// <summary>Ventana de validez formateada (valid_from -> valid_until).</summary>
    public string ValidityWindow => $"{ValidFrom} -> {ValidUntil}";

    /// <summary>Resumen corto para listados.</summary>
    public string Display =>
        $"#{Id} op {OperatorId} {Archetype}@{Segment}" +
        (string.IsNullOrWhiteSpace(Engagement) ? "" : $" [{Engagement}]");
}

/// <summary>GET /api/bindings -> {"bindings":[...]} (handleListBindings).</summary>
public sealed class BindingListResponse
{
    [JsonProperty("bindings")] public List<BindingItem>? Bindings { get; set; }
}

// ---------------------------------------------------------------------------
// Merkle cross-segmento (server/api/merkle.go, server/db/merkle.go)
// ---------------------------------------------------------------------------

/// <summary>
/// db.SegmentLeaf (server/db/merkle.go:21): proyeccion de un segmento
/// (ultimo hash + numero de eventos del segmento).
/// </summary>
public sealed class MerkleLeaf
{
    [JsonProperty("segment")] public string? Segment { get; set; }
    [JsonProperty("leaf")] public string? Leaf { get; set; }
    [JsonProperty("count")] public int Count { get; set; }
}

/// <summary>
/// POST /api/audit/merkleroot -> {id, leaves:[...], root, upto_id, created_at}.
/// upto_id fija el prefijo atestiguado: solo las filas id &lt;= upto_id componen.
/// </summary>
public sealed class MerkleSnapshot
{
    [JsonProperty("id")] public long Id { get; set; }
    [JsonProperty("leaves")] public List<MerkleLeaf>? Leaves { get; set; }
    [JsonProperty("root")] public string? Root { get; set; }
    [JsonProperty("upto_id")] public long UptoId { get; set; }
    [JsonProperty("created_at")] public string? CreatedAt { get; set; }
}

/// <summary>
/// GET /api/audit/merkleroot/verify -> {"ok":true,"root":"..."} o
/// {"ok":false,"stored":"...","recomputed":"..."}. Sin snapshot previo el
/// servidor responde 400 con {"ok":false,"error":"sin snapshot: ..."}.
/// </summary>
public sealed class MerkleVerifyResult
{
    [JsonProperty("ok")] public bool Ok { get; set; }
    [JsonProperty("root")] public string? Root { get; set; }
    [JsonProperty("stored")] public string? Stored { get; set; }
    [JsonProperty("recomputed")] public string? Recomputed { get; set; }
    [JsonProperty("error")] public string? Error { get; set; }
}