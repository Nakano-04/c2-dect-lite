using System;
using System.IO;

namespace C2Dect;

public static class Logger
{
    private static readonly string LogDir = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.Desktop),
        "test", "cs", "c2-dect");
    private static readonly string LogFile = Path.Combine(LogDir, "gui_crash.log");

    public static void Init()
    {
        try
        {
            if (!Directory.Exists(LogDir))
                Directory.CreateDirectory(LogDir);

            File.AppendAllText(LogFile, $"\n=== START {DateTime.Now:yyyy-MM-dd HH:mm:ss} ===\n");
        }
        catch { }
    }

    public static void Log(string message)
    {
        try
        {
            File.AppendAllText(LogFile, $"[{DateTime.Now:HH:mm:ss}] {message}\n");
        }
        catch { }
    }

    public static void LogError(string title, Exception ex)
    {
        try
        {
            File.AppendAllText(LogFile, $"\n!!! ERROR: {title} !!!\n{ex}\n!!! END ERROR !!!\n");
        }
        catch { }
    }

    public static void LogFatal(string title, Exception ex)
    {
        try
        {
            File.AppendAllText(LogFile, $"\n!!! FATAL: {title} !!!\n{ex}\n!!! END FATAL !!!\n");
        }
        catch { }
    }
}
