using System;
using System.Globalization;
using System.Windows.Data;

namespace C2Dect.Converters;

/// <summary>
/// Convierte el estado de proteccion de un implante en texto legible.
///
/// La distincion importante es de TRES estados, no de dos:
///
///   - "sin sellado": el material sensible permanece en el heap.
///   - "activo (en claro ahora)": el sellado esta configurado y funciona,
///     pero en este instante el material esta en claro (el agente acaba de
///     despertar o esta entre beacons).
///   - "sellado": el material esta cifrado AHORA MISMO, en una region
///     PAGE_NOACCESS.
///
/// Colapsar "activo" y "sellado" en un unico booleano seria mentir: el
/// operador veria "sellado" durante la ventana en la que el material esta
/// en claro, que es justo cuando le importa saberlo.
/// </summary>
public class ProtectionTextConverter : IValueConverter
{
    public object Convert(object value, Type targetType, object parameter, CultureInfo culture)
    {
        if (value is null)
        {
            return "no reporta";
        }

        // El binding pasa el objeto ImplantProtection del modelo.
        var type = value.GetType();
        var enabled = (bool)(type.GetProperty("SleepMaskEnabled")?.GetValue(value) ?? false);
        var sealedNow = (bool)(type.GetProperty("SleepMaskSealed")?.GetValue(value) ?? false);
        var custody = type.GetProperty("SleepMaskCustody")?.GetValue(value) as string ?? "";
        var profile = type.GetProperty("TrafficProfile")?.GetValue(value) as string ?? "default";
        var diversify = (bool)(type.GetProperty("TrafficDiversify")?.GetValue(value) ?? false);
        var ech = (bool)(type.GetProperty("TrafficECH")?.GetValue(value) ?? false);
        var padding = (int)(type.GetProperty("TrafficPadding")?.GetValue(value) ?? 0);
        var version = type.GetProperty("AgentVersion")?.GetValue(value) as string ?? "";

        string maskState;
        if (!enabled)
        {
            maskState = "SIN SELLADO";
        }
        else if (sealedNow)
        {
            maskState = "SELLADO";
        }
        else
        {
            maskState = "ACTIVO (en claro ahora)";
        }

        var traffic = profile;
        if (diversify) traffic += " +div";
        if (ech) traffic += " +ech";
        if (padding > 0) traffic += $" +pad{padding}";

        var custodyPart = string.IsNullOrEmpty(custody) ? "" : $" | custodia:{custody}";
        var versionPart = string.IsNullOrEmpty(version) ? "" : $" | v{version}";

        return $"{maskState} | trafico:{traffic}{custodyPart}{versionPart}";
    }

    public object ConvertBack(object value, Type targetType, object parameter, CultureInfo culture)
        => throw new NotImplementedException();
}
