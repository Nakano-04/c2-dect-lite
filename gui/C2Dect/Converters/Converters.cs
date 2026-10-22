using System;
using System.Globalization;
using System.Windows;
using System.Windows.Data;
using System.Windows.Media;

namespace C2Dect.Converters;

public class StatusToColorConverter : IValueConverter
{
    public object Convert(object value, Type targetType, object parameter, CultureInfo culture)
    {
        var status = value?.ToString()?.ToLower();
        return status switch
        {
            "active" => new SolidColorBrush(Color.FromRgb(76, 175, 80)),
            "sleeping" => new SolidColorBrush(Color.FromRgb(255, 193, 7)),
            "dead" => new SolidColorBrush(Color.FromRgb(244, 67, 54)),
            _ => new SolidColorBrush(Color.FromRgb(158, 158, 158))
        };
    }

    public object ConvertBack(object value, Type targetType, object parameter, CultureInfo culture)
        => throw new NotImplementedException();
}

public class BoolToVisibilityConverter : IValueConverter
{
    public object Convert(object value, Type targetType, object parameter, CultureInfo culture)
    {
        return value is true ? Visibility.Visible : Visibility.Collapsed;
    }

    public object ConvertBack(object value, Type targetType, object parameter, CultureInfo culture)
        => throw new NotImplementedException();
}

public class InverseBoolConverter : IValueConverter
{
    public object Convert(object value, Type targetType, object parameter, CultureInfo culture)
        => value is bool b ? !b : value;

    public object ConvertBack(object value, Type targetType, object parameter, CultureInfo culture)
        => value is bool b ? !b : value;
}

public class BoolToOnOffConverter : IValueConverter
{
    public static readonly BoolToOnOffConverter Instance = new();
    public object Convert(object value, Type targetType, object parameter, CultureInfo culture)
        => value is true ? "ON" : "OFF";
    public object ConvertBack(object value, Type targetType, object parameter, CultureInfo culture)
        => throw new NotImplementedException();
}

public class BoolToRunningConverter : IValueConverter
{
    public static readonly BoolToRunningConverter Instance = new();
    public object Convert(object value, Type targetType, object parameter, CultureInfo culture)
        => value is true ? "Active" : "Stopped";
    public object ConvertBack(object value, Type targetType, object parameter, CultureInfo culture)
        => throw new NotImplementedException();
}

public class NullToVisibilityConverter : IValueConverter
{
    public object Convert(object value, Type targetType, object parameter, CultureInfo culture)
        => value == null ? Visibility.Visible : Visibility.Collapsed;
    public object ConvertBack(object value, Type targetType, object parameter, CultureInfo culture)
        => throw new NotImplementedException();
}
