package server

import "fmt"

var stateCodes = map[int]string{
	0: "Kein Wärmebedarf", 1: "Heizen: Gebläseanlauf", 2: "Heizen: Pumpenvorlauf", 3: "Heizen: Zündung",
	4: "Heizen: Brennerbetrieb", 5: "Heizen: Gebläse- und Pumpennachlauf", 6: "Heizen: Gebläsenachlauf",
	7: "Heizen: Pumpennachlauf", 8: "Heizen: Restsperrzeit",
	10: "Warmwasser: Anforderung", 11: "Warmwasser: Gebläseanlauf", 13: "Warmwasser: Zündung",
	14: "Warmwasser: Brennerbetrieb", 15: "Warmwasser: Gebläse- und Pumpennachlauf", 16: "Warmwasser: Gebläsenachlauf",
	17: "Warmwasser: Pumpennachlauf",
	20: "Speicherladung: Anforderung", 21: "Speicherladung: Gebläseanlauf", 22: "Speicherladung: Pumpenvorlauf",
	23: "Speicherladung: Zündung", 24: "Speicherladung: Brennerbetrieb", 25: "Speicherladung: Gebläse- und Pumpennachlauf",
	26: "Speicherladung: Gebläsenachlauf", 27: "Speicherladung: Pumpennachlauf", 28: "Speicherladung: Restsperrzeit",
	30: "Raumthermostat blockiert Heizbetrieb", 31: "Sommerbetrieb / kein Wärmebedarf vom Regler",
	32: "Wartezeit wegen Lüfterdrehzahlabweichung", 34: "Frostschutz aktiv", 36: "Sollwert vom Regler < 20 °C – Heizbetrieb blockiert",
	39: "Anlegethermostat ausgelöst", 41: "Anlagendruck zu hoch", 42: "Abgasklappe blockiert Brennerbetrieb",
	53: "Wartezeit wegen Wassermangel (Spreizung zu groß)", 54: "Wartezeit wegen Wassermangel (Temperaturgradient)",
	88: "Entlüftungsprogramm aktiv", 96: "Selbsttest Rücklauffühler", 97: "Selbsttest Wasserdrucksensor",
	98: "Selbsttest Vor-/Rücklauffühler", 99: "Selbsttest",
}

var errorCodes = map[int]string{
	0: "Unterbrechung Vorlauftemperaturfühler", 1: "Unterbrechung Rücklauftemperaturfühler",
	10: "Kurzschluss Vorlauftemperaturfühler", 11: "Kurzschluss Rücklauftemperaturfühler",
	13: "Kurzschluss Speichertemperaturfühler", 20: "Sicherheitstemperaturbegrenzer ausgelöst",
	22: "Trockenbrand / Wassermangel", 23: "Wassermangel (Spreizung Vor-/Rücklauf zu groß)",
	24: "Wassermangel (Temperaturanstieg zu schnell)", 25: "Abgastemperatur zu hoch",
	26: "Gasarmatur ohne Funktion", 27: "Fremdlicht (Flammensignal bei geschlossenem Gasventil)",
	28: "Zündung beim Anlauf fehlgeschlagen", 29: "Flammenausfall im Betrieb", 32: "Gebläsefehler",
	49: "eBUS-Unterspannung", 61: "Fehler Gasventilansteuerung", 62: "Gasventil-Abschaltverzögerung",
	63: "EEPROM-Fehler", 64: "Elektronik- oder Fühlerfehler", 65: "Elektroniktemperatur zu hoch",
	67: "Flammenwächter-Eingangssignal unplausibel", 70: "Ungültige Gerätekennung (DSN)",
	71: "Vorlauffühler meldet konstanten Wert", 72: "Abweichung Vor-/Rücklauffühler",
	73: "Wasserdrucksensor: Signal zu klein", 74: "Wasserdrucksensor: Signal zu groß",
	75: "Kein Drucksprung beim Pumpenstart erkannt", 77: "Abgasklappe / Kondensatpumpe",
	83: "Trockenbrand (Temperaturänderung zu klein)", 84: "Temperaturdifferenz Vor-/Rücklauf unplausibel",
	85: "Vor-/Rücklauffühler vertauscht oder falsch montiert", 86: "Fußbodenkontakt/Anlegethermostat offen",
}

func stateText(n int) string {
	if t, ok := stateCodes[n]; ok {
		return fmt.Sprintf("S.%02d %s", n, t)
	}
	return fmt.Sprintf("S.%02d", n)
}

func errorText(n int) string {
	if t, ok := errorCodes[n]; ok {
		return fmt.Sprintf("F.%02d %s", n, t)
	}
	return fmt.Sprintf("F.%02d", n)
}
