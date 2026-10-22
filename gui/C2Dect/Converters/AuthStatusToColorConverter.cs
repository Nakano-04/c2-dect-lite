using System;
using System.Globalization;
using System.Windows.Data;
using System.Windows.Media;

namespace C2Dect.Converters;

public class AuthStatusToColorConverter : IValueConverter
{
    public object Convert(object value, Type targetType, object parameter, CultureInfo culture)
    {
        var status = value?.ToString()?.ToLower();
        return status switch
        {
            "autorizado" or "autorizado (hard stop)" => new SolidColorBrush(Color.FromRgb(76, 175, 80)),   // Verde
            "no autorizado" or "no autorizado (hard stop)" => new SolidColorBrush(Color.FromRgb(244, 67, 54)), // Rojo
            _ => new SolidColorBrush(Color.FromRgb(158, 158, 158))
        };
    }

    public object ConvertBack(object value, Type targetType, object parameter, CultureInfo culture)
        => throw new NotImplementedException();
}
