using System;
using System.Globalization;
using System.Windows.Data;
using System.Windows.Media;

namespace C2Dect.Converters;

public class RiskToColorConverter : IValueConverter
{
    public object Convert(object value, Type targetType, object parameter, CultureInfo culture)
    {
        if (value is not int score) return new SolidColorBrush(Color.FromRgb(158, 158, 158));

        return score switch
        {
            >= 70 => new SolidColorBrush(Color.FromRgb(244, 67, 54)),   // Rojo - crítico
            >= 40 => new SolidColorBrush(Color.FromRgb(255, 193, 7)),   // Amarillo - medio
            _     => new SolidColorBrush(Color.FromRgb(76, 175, 80)),   // Verde - bajo
        };
    }

    public object ConvertBack(object value, Type targetType, object parameter, CultureInfo culture)
        => throw new NotImplementedException();
}
