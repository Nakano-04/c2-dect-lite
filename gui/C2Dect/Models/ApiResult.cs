using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using System;

namespace C2Dect.Models;

/// <summary>
/// Error de API tipado. El servidor Go responde con {"error": "..."} en casi
/// todos los handlers, pero los 404 cross-tenant son deliberadamente
/// indistinguibles de un 404 real: no se puede inferir existencia desde la GUI.
/// </summary>
public sealed class ApiError
{
    public ApiError(int statusCode, string message)
    {
        StatusCode = statusCode;
        Message = string.IsNullOrWhiteSpace(message) ? "sin detalle" : message.Trim();
    }

    public int StatusCode { get; }
    public string Message { get; }

    public bool IsTransport => StatusCode == 0;
    public bool IsUnauthorized => StatusCode == 401;
    public bool IsForbidden => StatusCode == 403;
    public bool IsNotFound => StatusCode == 404;

    /// <summary>Extrae {"error": "..."}; si no está, devuelve el cuerpo recortado.</summary>
    public static ApiError FromBody(int statusCode, string? body)
    {
        string message = body ?? "";
        if (!string.IsNullOrWhiteSpace(body))
        {
            try
            {
                var token = JToken.Parse(body);
                if (token is JObject obj)
                {
                    if (obj.TryGetValue("error", out var err) && err.Type != JTokenType.Null)
                        message = err.ToString();
                }
                else
                {
                    message = body.Length > 200 ? body[..200] : body;
                }
            }
            catch (JsonException)
            {
                message = body.Length > 200 ? body[..200] : body;
            }
        }
        return new ApiError(statusCode, message);
    }
}

/// <summary>
/// Envoltura de resultado: distingue "el servidor respondió y esto es lo que hay"
/// de "falló la llamada", que es justo lo que Get&lt;T&gt; no puede expresar.
/// </summary>
public sealed class ApiResult<T>
{
    private ApiResult(bool ok, T? value, ApiError? error)
    {
        Ok = ok;
        Value = value;
        Error = error;
    }

    public bool Ok { get; }
    public T? Value { get; }
    public ApiError? Error { get; }

    public static ApiResult<T> Success(T value) => new(true, value, null);
    public static ApiResult<T> Failure(ApiError error) => new(false, default, error);
    public static ApiResult<T> Failure(int statusCode, string message) => new(false, default, new ApiError(statusCode, message));

    public string ErrorMessage => Error?.Message ?? "";

    public override string ToString() => Ok ? "ok" : $"{Error?.StatusCode}: {ErrorMessage}";
}
