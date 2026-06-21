using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Newtonsoft.Json;
using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading.Tasks;
using System.Windows;

namespace C2Dect.Lite.Models;

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
    [ObservableProperty] private DateTime _lastCheckIn;
    [ObservableProperty] private DateTime _firstCheckIn;
    [ObservableProperty] private int _sleepSec;
    [ObservableProperty] private string _tags = "";
    [ObservableProperty] private string _operator = "";

    public string DisplayName => $"{Username}@{Hostname}";
    public string StatusIcon => Status switch
    {
        "active" => "🟢",
        "sleeping" => "🟡",
        "dead" => "🔴",
        _ => "⚪"
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
        "pending" => "⏳",
        "sent" => "📤",
        "completed" => "✅",
        "error" => "❌",
        _ => "❓"
    };
}

public partial class Operator : ObservableObject
{
    [ObservableProperty] private long _id;
    [ObservableProperty] private string _username = "";
    [ObservableProperty] private string _role = "operator";
    [ObservableProperty] private DateTime _createdAt;
    [ObservableProperty] private bool _isOnline;
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

public partial class MalleableProfile : ObservableObject
{
    [ObservableProperty] private string _name = "";
    [ObservableProperty] private string _description = "";
    [ObservableProperty] private string _method = "POST";
    [ObservableProperty] private string _contentType = "application/json";
    [ObservableProperty] private string _encodeMode = "json";
    [ObservableProperty] private int _jitter = 30;
    [ObservableProperty] private int _defaultSleep = 10;
    public List<string> URIs { get; set; } = new();
    public List<string> UserAgents { get; set; } = new();
    public Dictionary<string, string> Headers { get; set; } = new();
    [ObservableProperty] private string _postTemplate = "";
}

public class ConsoleMessage
{
    public DateTime Timestamp { get; set; } = DateTime.Now;
    public string Direction { get; set; } = ""; // "sent", "received", "info", "error"
    public string Content { get; set; } = "";
    public string Color => Direction switch
    {
        "sent" => "#00FF41",
        "received" => "#00BFFF",
        "info" => "#FFD700",
        "error" => "#FF4444",
        _ => "#FFFFFF"
    };
}

public class DashboardStats
{
    [JsonProperty("total_sessions")] public int TotalSessions { get; set; }
    [JsonProperty("active_sessions")] public int ActiveSessions { get; set; }
    [JsonProperty("timestamp")] public DateTime Timestamp { get; set; }
}
