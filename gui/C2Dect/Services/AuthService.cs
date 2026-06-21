using System;
using System.Threading.Tasks;
using C2Dect.Lite.Models;

namespace C2Dect.Lite.Services;

public class AuthService
{
    private readonly ApiService _api;
    private string? _token;
    private Operator? _currentOperator;
    private string _serverUrl = "http://127.0.0.1:8443";

    public AuthService(ApiService api)
    {
        _api = api;
    }

    public bool IsAuthenticated => !string.IsNullOrEmpty(_token);
    public Operator? CurrentOperator => _currentOperator;
    public string? Token => _token;
    public string ServerUrl => _serverUrl;

    public async Task<bool> Login(string username, string password, string serverUrl = "")
    {
        if (!string.IsNullOrEmpty(serverUrl))
        {
            _serverUrl = serverUrl.TrimEnd('/');
        }

        var result = await _api.Login(username, password);
        if (result == null) return false;

        _token = result.Value.token;
        _currentOperator = result.Value.op;
        _api.Configure(_serverUrl, _token);
        return true;
    }

    public void Logout()
    {
        _token = null;
        _currentOperator = null;
    }
}
