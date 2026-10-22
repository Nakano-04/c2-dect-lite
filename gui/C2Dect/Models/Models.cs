using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Newtonsoft.Json;
using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading.Tasks;
using System.Windows;

namespace C2Dect.Models;

public partial class Session : ObservableObject
{
    [ObservableProperty] private string _id = "";
    [ObservableProperty] private string _uuid = "";
    [ObservableProperty] private string _hostname = "";
    [ObservableProperty] private string _username = "";
    [ObservableProperty] private string _internalIp = "";
    [ObservableProperty] private string _externalIp = "";
    [ObservableProperty] private string _oS = "";
    [ObservableProperty] private string _arch = "";
    [ObservableProperty] private int _pID;
    [ObservableProperty] private string _process = "";
    [ObservableProperty] private string _status = "active";
    [ObservableProperty] private string _tags = "";
    [ObservableProperty] private string _operator = "";
    // El servidor serializa snake_case (db.Session, database.go:58-75). Newtonsoft
    // solo hace match case-insensitive, no ignora guiones bajos: sin este
    // JsonProperty tenant_id NUNCA se deserializa y TenantId se queda en 1.
    [ObservableProperty] [JsonProperty("tenant_id")] private long _tenantId = 1;
    [ObservableProperty] [JsonProperty("last_checkin")] private DateTime _lastCheckIn;
    [ObservableProperty] [JsonProperty("first_checkin")] private DateTime _firstCheckIn;
    [ObservableProperty] [JsonProperty("sleep_sec")] private int _sleepSec;
    [ObservableProperty] [JsonProperty("jitter")] private int _jitter;
    [ObservableProperty] [JsonProperty("agent_version")] private string _agentVersion = "";
    [ObservableProperty] [JsonProperty("key_epoch")] private int _keyEpoch;

    public string DisplayName => $"{Username}@{Hostname}";
    public string StatusIcon => Status switch
    {
        "active" => "ðŸŸ¢",
        "sleeping" => "ðŸŸ¡",
        "dead" => "ðŸ”´",
        _ => "âšª"
    };

    // Vivo / dormido / caÃ­do segÃºn Ãºltimo check-in + estado reportado
    public string StateText
    {
        get
        {
            if (Status == "dead") return "DEAD";
            var ago = DateTime.UtcNow - LastCheckIn.ToUniversalTime();
            if (Status == "active" && ago.TotalSeconds < 60) return "LIVE";
            if (ago.TotalMinutes < 5) return "SLEEP";
            return "STALE";
        }
    }

    public string StatusColor => StateText switch
    {
        "LIVE" => "#00e676",
        "SLEEP" => "#d4a017",
        "STALE" => "#5a6070",
        "DEAD" => "#ff5252",
        _ => "#5a6070"
    };
}

public partial class C2Task : ObservableObject
{
    [ObservableProperty] private long _id;
    [ObservableProperty] private string _sessionId = "";
    [ObservableProperty] private string _command = "";
    [ObservableProperty] private string _args = "";
    [ObservableProperty] private string _status = "pending";
    [ObservableProperty] private string _result = "";
    [ObservableProperty] private string _error = "";
    [ObservableProperty] private long _operatorId;
    [ObservableProperty] private DateTime _createdAt;
    [ObservableProperty] private DateTime? _sentAt;
    [ObservableProperty] private DateTime? _completedAt;

    public string StatusIcon => Status switch
    {
        "pending" => "â³",
        "sent" => "ðŸ“¤",
        "completed" => "âœ…",
        "error" => "âŒ",
        _ => "â“"
    };
}

public partial class Listener : ObservableObject
{
    [ObservableProperty] private string _id = "";
    [ObservableProperty] private string _name = "";
    [ObservableProperty] private string _protocol = "http";
    [ObservableProperty] private string _bindAddress = "0.0.0.0";
    [ObservableProperty] private int _port;

    // `useSsl` se elimino del modelo. La API lo devuelve SIEMPRE a false porque
    // ningun listener de esta ruta aplica TLS (no hay tls.NewListener en el
    // servidor). Mantener la propiedad era mantener en el modelo una promesa que
    // el backend no cumple. La columna que la pintaba tambien se elimino de
    // ListenersView.xaml.
    //
    // `boundProtocol` es su sustituto: la red REAL de bind que devuelve la API
    // (tcp, tcp4, unix...), util porque `protocol` es un nombre de canal
    // logico (http, dns, smb) que `net.Listen` no entiende.
    [ObservableProperty] private string _boundProtocol = "";
    [ObservableProperty] private string _profileName = "browser";
    [ObservableProperty] private bool _isRunning;
    [ObservableProperty] private int _connectedSessions;
}

public partial class Campaign : ObservableObject
{
    [ObservableProperty] private string _id = "";
    [ObservableProperty] private string _name = "";
    [ObservableProperty] private string _description = "";
    [ObservableProperty] private DateTime _createdAt;
    [ObservableProperty] private bool _isActive = true;
    [ObservableProperty] private int _sessionCount;
    [ObservableProperty] private int _taskCount;
}

public partial class Operator : ObservableObject
{
    [ObservableProperty] private long _id;
    [ObservableProperty] private string _username = "";
    [ObservableProperty] private string _role = "operator";
    [ObservableProperty] private DateTime _createdAt;
    [ObservableProperty] private bool _isOnline;
    [JsonProperty("tenant_id")] public long TenantId { get; set; } = 1;
}

public partial class Loot : ObservableObject
{
    [ObservableProperty] private long _id;
    [ObservableProperty] private string _sessionId = "";
    [ObservableProperty] private string _type = "";
    [ObservableProperty] private string _name = "";
    [ObservableProperty] private string _path = "";
    [ObservableProperty] private string _hash = "";
    [ObservableProperty] private DateTime _createdAt;
    public byte[]? Data { get; set; }
}

/// <summary>
/// Perfil malleable (server/profiles/malleable.go).
///
/// El servidor Go serializa en snake_case. Newtonsoft solo casa por nombre
/// ignorando mayÃºsculas, NO los guiones bajos: sin [JsonProperty] el GET
/// /api/profiles devolverÃ­a "content_type" y la GUI no lo associarÃ­a a
/// ContentType (quedarÃ­a "application/json" por defecto). El atributo va en el
/// campo porque la propiedad la genera el source generator de MVVM y no
/// admite atributos propios.
/// </summary>
public partial class MalleableProfile : ObservableObject
{
    [ObservableProperty] [JsonProperty("name")] private string _name = "";
    [ObservableProperty] [JsonProperty("description")] private string _description = "";
    [ObservableProperty] [JsonProperty("method")] private string _method = "POST";
    [ObservableProperty] [JsonProperty("content_type")] private string _contentType = "application/json";
    [ObservableProperty] [JsonProperty("encode_mode")] private string _encodeMode = "json";
    [ObservableProperty] [JsonProperty("jitter")] private int _jitter = 30;
    [ObservableProperty] [JsonProperty("default_sleep")] private int _defaultSleep = 10;
    [JsonProperty("uris")] public List<string> URIs { get; set; } = new();
    [JsonProperty("user_agents")] public List<string> UserAgents { get; set; } = new();
    [JsonProperty("headers")] public Dictionary<string, string> Headers { get; set; } = new();
    [ObservableProperty] [JsonProperty("post_template")] private string _postTemplate = "";
}

public class ConsoleMessage
{
    public DateTime Timestamp { get; set; } = DateTime.Now;
    public string Direction { get; set; } = "";
    public string Content { get; set; } = "";
    public string? ExplicitColor { get; set; }
    // Paleta legible: info en gris claro (no amarillo puro), Ã©xito verde, error rojo
    public string Color => ExplicitColor ?? Direction switch
    {
        "sent" => "#00e676",
        "received" => "#c0c8d8",
        "success" => "#00e676",
        "info" => "#9aa4b8",
        "warn" => "#d4a017",
        "error" => "#ff5252",
        _ => "#c0c8d8"
    };
    public string ColorHex { get => Color; set => ExplicitColor = value; }
}

public class DashboardStats
{
    [JsonProperty("total_sessions")] public int TotalSessions { get; set; }
    [JsonProperty("active_sessions")] public int ActiveSessions { get; set; }
    [JsonProperty("timestamp")] public DateTime Timestamp { get; set; }
}

/// <summary>Una orden del agente con su nivel de impacto.</summary>
public class CommandSpec
{
    [JsonProperty("name")] public string Name { get; set; } = "";
    [JsonProperty("risk")] public string Risk { get; set; } = "read";
    [JsonProperty("aliases")] public List<string> Aliases { get; set; } = new();
    [JsonProperty("summary")] public string Summary { get; set; } = "";

    public string RiskLabel => Risk switch
    {
        "impact" => "IMPACTO",
        "write" => "ESCRITURA",
        _ => "LECTURA"
    };

    public string RiskColor => Risk switch
    {
        "impact" => "#ff5252",
        "write" => "#d4a017",
        _ => "#00c853"
    };
}

