package catalog

import (
	"fmt"
	"strings"
)

type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type Item struct {
	ID       string   `json:"id"`
	Circuit  string   `json:"circuit"`
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Help     string   `json:"help,omitempty"`
	Kind     string   `json:"kind"`
	Unit     string   `json:"unit,omitempty"`
	Min      *float64 `json:"min,omitempty"`
	Max      *float64 `json:"max,omitempty"`
	Step     *float64 `json:"step,omitempty"`
	Decimals int      `json:"decimals"`
	MaxLen   int      `json:"maxlen,omitempty"`
	Options  []Option `json:"options"`
	Writable bool     `json:"writable"`
	Level    string   `json:"level"`
	Format   string   `json:"format,omitempty"`

	AllowWrite bool      `json:"-"`
	BoolRaw    [2]string `json:"-"`
	MaxAge     int       `json:"-"`
}

type Group struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Icon     string   `json:"icon"`
	Requires string   `json:"requires,omitempty"`
	Items    []string `json:"items"`
}

type Section struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Level  string   `json:"level"`
	Groups []*Group `json:"groups"`
}

type Catalog struct {
	Sections  []*Section       `json:"sections"`
	Dashboard []string         `json:"dashboard"`
	Items     map[string]*Item `json:"items"`
}

func fp(v float64) *float64 { return &v }

func it(circuit, name, label string) *Item {
	return &Item{ID: circuit + "." + name, Circuit: circuit, Name: name, Label: label, Kind: "number", Level: "user", MaxAge: 30}
}

func (i *Item) Num(unit string, min, max, step float64, dec int) *Item {
	i.Kind, i.Unit, i.Min, i.Max, i.Step, i.Decimals = "number", unit, fp(min), fp(max), fp(step), dec
	return i
}

func (i *Item) Val(unit string, dec int) *Item {
	i.Kind, i.Unit, i.Decimals = "number", unit, dec
	return i
}

// Enum takes "raw=Label" pairs.
func (i *Item) Enum(pairs ...string) *Item {
	i.Kind = "enum"
	for _, p := range pairs {
		v, l, _ := strings.Cut(p, "=")
		i.Options = append(i.Options, Option{v, l})
	}
	return i
}

func (i *Item) Bool(trueRaw, falseRaw string) *Item {
	i.Kind, i.BoolRaw = "bool", [2]string{trueRaw, falseRaw}
	return i
}

func (i *Item) Date() *Item  { i.Kind = "date"; return i }
func (i *Item) Time() *Item  { i.Kind = "time"; return i }
func (i *Item) Timer() *Item { i.Kind = "timer"; i.MaxAge = 300; return i }
func (i *Item) Text(max int) *Item {
	i.Kind, i.MaxLen = "text", max
	return i
}
func (i *Item) W() *Item           { i.AllowWrite = true; return i }
func (i *Item) Inst() *Item        { i.Level = "installer"; return i }
func (i *Item) H(h string) *Item   { i.Help = h; return i }
func (i *Item) Fmt(f string) *Item { i.Format = f; return i }
func (i *Item) Age(s int) *Item    { i.MaxAge = s; return i }

var (
	opMode  = []string{"off=Aus", "auto=Automatik", "day=Tag", "night=Nacht"}
	sfMode  = []string{"auto=Keine", "party=Party", "onedayaway=1 Tag außer Haus", "onedayathome=1 Tag zu Hause", "ventilation=Stoßlüften", "veto=Manuell (Quick Veto)", "load=Warmwasser einmalig laden"}
	relay   = []string{"circulationpump=Zirkulationspumpe", "extheatingpump=Externe Heizungspumpe", "storagechargingpump=Speicherladepumpe", "fluegasflapextractorhood=Abgasklappe/Dunstabzug", "externalgasvalve=Externes Gasventil", "externalerrormessage=Externe Störmeldung", "solarpump=Solarpumpe", "remotecontrol=Fernbedienung"}
	mapping = []string{"none=Keine", "VRC700=VRC 700", "VR91.1=VR 91 (1)", "VR91.2=VR 91 (2)", "VR91.3=VR 91 (3)"}
)

func zoneGroups(z int) (user, timers, installer *Group, items []*Item) {
	zn, hc := fmt.Sprintf("z%d", z), fmt.Sprintf("Hc%d", z)
	req := ""
	if z > 1 {
		req = fmt.Sprintf("zone%d", z)
	}
	add := func(g *Group, is ...*Item) {
		for _, i := range is {
			g.Items = append(g.Items, i.ID)
			items = append(items, i)
		}
	}
	user = &Group{ID: zn, Title: fmt.Sprintf("Zone %d – Heizen", z), Icon: "flame", Requires: req}
	add(user,
		it("700", zn+"OpMode", "Betriebsart").Enum(opMode...).W().H("Automatik folgt dem Zeitprogramm (Tag/Absenkung)."),
		it("700", zn+"DayTemp", "Wunschtemperatur (Tag)").Num("°C", 5, 30, 0.5, 1).W().H("Raumsolltemperatur innerhalb der Zeitfenster des Zeitprogramms."),
		it("700", zn+"NightTemp", "Absenktemperatur (Nacht)").Num("°C", 5, 30, 0.5, 1).W().H("Raumsolltemperatur außerhalb der Zeitfenster. Gilt nur bei Absenkbetrieb „Absenken“; bei „Eco“ ist die Heizung dann aus (nur Frostschutz)."),
		it("700", zn+"ActualRoomTempDesired", "Aktuelle Raumsolltemperatur").Val("°C", 1).H("Berücksichtigt Zeitprogramm, Sonderfunktionen und Urlaub."),
		it("700", zn+"RoomTemp", "Raumtemperatur").Val("°C", 1).H("Gemessen am Fühler laut „Zuordnung Raumtemperatur“."),
		it("700", zn+"SFMode", "Sonderfunktion").Enum(sfMode...).W().Age(10).H("Zeitlich begrenzte Abweichung vom Zeitprogramm, endet automatisch. Party: Wunschtemperatur bis zum nächsten Zeitfenster. 1 Tag außer Haus: Absenkbetrieb bis 24 Uhr. 1 Tag zu Hause: heute Sonntags-Zeitprogramm. Stoßlüften: Heizung 30 min aus."),
		it("700", zn+"QuickVetoTemp", "Quick-Veto-Temperatur").Num("°C", 5, 30, 0.5, 1).W().H("Solltemperatur für die Sonderfunktion „Manuell (Quick Veto)“."),
		it("700", zn+"ValveStatus", "Zonenventil").Bool("1", "0").H("Ein = Zonenventil offen, die Zone wird mit Wärme versorgt."),
		it("700", zn+"Shortname", "Zonenname (kurz)").Text(6).W().H("Kurzname der Zone im Reglerdisplay (max. 6 Zeichen)."),
		it("700", zn+"Name1", "Bezeichnung Teil 1").Text(5).W().H("Erste Hälfte des Zonennamens (max. 5 Zeichen)."),
		it("700", zn+"Name2", "Bezeichnung Teil 2").Text(5).W().H("Zweite Hälfte des Zonennamens (max. 5 Zeichen)."),
	)
	timers = &Group{ID: zn + "timer", Title: fmt.Sprintf("Zeitprogramm Heizen Zone %d", z), Icon: "clock", Requires: req}
	add(timers, it("700", zn+"Timer", fmt.Sprintf("Heizen Zone %d", z)).Timer().W().H("Zeitfenster mit Wunschtemperatur, außerhalb Absenkbetrieb."))

	installer = &Group{ID: strings.ToLower(hc), Title: fmt.Sprintf("Heizkreis %d", z), Icon: "wrench", Requires: req}
	add(installer,
		it("700", hc+"CircuitType", "Kreistyp").Enum("inactive=Inaktiv", "mixer=Mischerkreis", "fixed=Festwertkreis", "hwc=Warmwasser", "returnincr=Rücklaufanhebung", "pool=Schwimmbad").Inst().Age(300).H("Art des Kreises, festgelegt bei der Inbetriebnahme. „Mischerkreis“ = witterungsgeführter Heizkreis (am Regler „Heizen“, direkt oder gemischt); Festwert = konstante Vorlauftemperatur. Nur am Regler änderbar."),
		it("700", hc+"HeatCurve", "Heizkurve").Num("", 0.1, 4.0, 0.05, 2).W().Inst().H("Steilheit: Vorlaufsolltemperatur abhängig von der Außentemperatur. Höher = wärmerer Vorlauf bei Kälte. Richtwerte: Heizkörper 1,0–1,5, Fußbodenheizung 0,3–0,7."),
		it("700", hc+"HeatCurveAdaption", "Heizkurvenanpassung (adaptiv)").Val("", 2).Inst().H("Vom Regler ermittelte Korrektur der Heizkurve; nur bei aktiver „Adaptiver Heizkurve“ ungleich 0."),
		it("700", hc+"MinFlowTempDesired", "Minimale Vorlauftemperatur").Num("°C", 15, 90, 1, 0).W().Inst().H("Untergrenze der Vorlaufsolltemperatur, gilt auch bei mildem Wetter."),
		it("700", hc+"MaxFlowTempDesired", "Maximale Vorlauftemperatur").Num("°C", 15, 90, 1, 0).W().Inst().H("Obergrenze der Vorlaufsolltemperatur; bei Fußbodenheizung Schutz für Estrich und Rohre. Die Therme begrenzt zusätzlich über d.71."),
		it("700", hc+"SummerTempLimit", "AT-Abschaltgrenze").Num("°C", 10, 99, 1, 0).W().Inst().H("Liegt die Außentemperatur darüber, schaltet die Heizung ab (Sommerbetrieb)."),
		it("700", hc+"AutoOffMode", "Absenkbetrieb").Enum("eco=Eco (Heizung aus, Frostschutz)", "night=Absenken (Absenktemperatur)").W().Inst().H("Verhalten außerhalb der Zeitfenster. Eco: Heizung aus, Frostschutz aktiv. Absenken: Regelung auf die Absenktemperatur."),
		it("700", hc+"RoomTempSwitchOn", "Raumaufschaltung").Enum("off=Keine", "modulating=Aufschalten", "thermostat=Thermostat").W().Inst().H("Einfluss der Raumtemperatur auf die Vorlaufregelung. Keine: rein witterungsgeführt. Aufschalten: Abweichung vom Raumsollwert korrigiert den Vorlaufsollwert. Thermostat: zusätzlich wird der Kreis abgeschaltet, sobald der Raumsollwert überschritten ist. Benötigt einen zugeordneten Raumfühler."),
		it("700", hc+"ExcessTemp", "Überhöhung Mischer").Num("K", 0, 30, 1, 0).W().Inst().H("Vorlaufsollwert wird um diesen Wert erhöht, damit der Mischer im Regelbereich bleibt."),
		it("700", hc+"ActualFlowTempDesired", "Vorlaufsolltemperatur").Val("°C", 1).Inst().H("Aktuell berechneter Vorlaufsollwert aus Heizkurve, Außentemperatur und Raumsollwert, begrenzt durch Min./Max."),
		it("700", hc+"FlowTemp", "Vorlauftemperatur").Val("°C", 1).Inst().H("Gemessene Vorlauftemperatur dieses Heizkreises."),
		it("700", hc+"MixerMovement", "Mischerbewegung").Val("", 1).Inst().H("< 0 schließt, > 0 öffnet."),
		it("700", hc+"Status", "Status Heizkreis").Val("", 0).Inst().H("Betriebszustand des Heizkreises als Rohwert des Reglers."),
		it("700", hc+"PumpStatus", "Pumpenstatus").Val("", 0).Inst().H("Zustand der Heizkreispumpe als Rohwert (0 = aus)."),
		it("700", zn+"RoomZoneMapping", "Zuordnung Raumtemperatur").Enum(mapping...).W().Inst().H("Welcher Fühler die Raumtemperatur dieser Zone liefert."),
	)
	return
}

func Build() *Catalog {
	c := &Catalog{Items: map[string]*Item{}}
	reg := func(g *Group, is ...*Item) *Group {
		for _, i := range is {
			g.Items = append(g.Items, i.ID)
			c.Items[i.ID] = i
		}
		return g
	}

	grund := reg(&Group{ID: "grundanzeige", Title: "Grundanzeige", Icon: "home"},
		it("700", "OpMode", "Betriebsart (System)").Enum(opMode...).W().H("Wirkt auf die unter „Wirkung Grundanzeige“ gewählten Zonen. Automatik folgt dem Zeitprogramm, Tag/Nacht gelten dauerhaft."),
		it("700", "SFMode", "Schnellwahl / Sonderfunktion").Enum(sfMode...).W().Age(10).H("Zeitlich begrenzte Abweichung vom Zeitprogramm, endet automatisch. Party: Wunschtemperatur bis zum nächsten Zeitfenster. 1 Tag außer Haus: Absenkbetrieb bis 24 Uhr. 1 Tag zu Hause: heute Sonntags-Zeitprogramm. Stoßlüften: Heizung 30 min aus."),
		it("700", "GlobalSystemOff", "System aus").Bool("yes", "no").W().Inst().H("Schaltet das gesamte System inkl. Frostschutz-Überwachung ab – Frostgefahr für die Anlage."),
	)

	var zoneUser, zoneTimers, zoneInst []*Group
	for z := 1; z <= 3; z++ {
		u, t, i, items := zoneGroups(z)
		for _, x := range items {
			c.Items[x.ID] = x
		}
		zoneUser, zoneTimers, zoneInst = append(zoneUser, u), append(zoneTimers, t), append(zoneInst, i)
	}

	ww := reg(&Group{ID: "warmwasser", Title: "Warmwasser", Icon: "drop", Requires: "hwc"},
		it("700", "HwcOpMode", "Betriebsart Warmwasser").Enum(opMode...).W().H("Automatik: Speicher wird nur in den Zeitfenstern des Warmwasser-Zeitprogramms geladen. Tag: dauerhaft auf Solltemperatur. Aus: nur Frostschutz."),
		it("700", "HwcTempDesired", "Warmwasser-Solltemperatur").Num("°C", 35, 70, 1, 0).W().H("Speichersolltemperatur. Unter 50 °C steigt das Legionellenrisiko, über 60 °C die Verkalkung."),
		it("700", "HwcSFMode", "Sonderfunktion Warmwasser").Enum(sfMode...).W().Age(10).H("„Warmwasser einmalig laden“ heizt den Speicher sofort einmal auf Solltemperatur, unabhängig vom Zeitprogramm."),
		it("700", "HwcStorageTemp", "Speichertemperatur").Val("°C", 1),
		it("700", "HwcFlowTemp", "Warmwasser-Vorlauftemperatur").Val("°C", 1).H("Vorlauftemperatur der Speicherladung."),
		it("700", "HwcHolidayStartPeriod", "Abwesenheit Warmwasser von").Date().W().Age(60).H("Während der Abwesenheit keine Warmwasserbereitung, nur Frostschutz."),
		it("700", "HwcHolidayEndPeriod", "Abwesenheit Warmwasser bis").Date().W().Age(60),
		it("700", "HwcBankHolidayStartPeriod", "Zu Hause (Feiertag) Warmwasser von").Date().W().Age(60).H("In diesem Zeitraum gilt das Sonntags-Zeitprogramm für Warmwasser."),
		it("700", "HwcBankHolidayEndPeriod", "Zu Hause (Feiertag) Warmwasser bis").Date().W().Age(60),
	)
	kuehlen := reg(&Group{ID: "kuehlen", Title: "Kühlen", Icon: "drop", Requires: "cooling"},
		it("700", "OpModeCooling", "Betriebsart Kühlen (System)").Enum(opMode...).W(),
		it("700", "z1OpModeCooling", "Betriebsart Kühlen Zone 1").Enum(opMode...).W(),
		it("700", "z1CoolingTemp", "Kühl-Solltemperatur Zone 1").Num("°C", 15, 30, 0.5, 1).W(),
	)
	lueftung := reg(&Group{ID: "lueftung", Title: "Lüftung", Icon: "home", Requires: "ventilation"},
		it("700", "OpModeVentilation", "Betriebsart Lüftung").Enum(opMode...).W(),
		it("700", "VentilationDay", "Lüftungsstufe Tag").Num("", 0, 6, 1, 0).W().H("Lüftungsstufe des Lüftungsgeräts innerhalb der Zeitfenster."),
		it("700", "VentilationNight", "Lüftungsstufe Nacht").Num("", 0, 6, 1, 0).W().H("Lüftungsstufe des Lüftungsgeräts außerhalb der Zeitfenster."),
	)

	timerGroups := append([]*Group{}, zoneTimers...)
	timerGroups = append(timerGroups,
		reg(&Group{ID: "wwtimer", Title: "Zeitprogramm Warmwasser", Icon: "clock", Requires: "hwc"},
			it("700", "hwcTimer", "Warmwasser").Timer().W().H("Zeitfenster, in denen der Speicher auf Solltemperatur gehalten wird."),
			it("700", "ccTimer", "Zirkulation").Timer().W().H("Zeitfenster, in denen die Zirkulationspumpe läuft.")),
		reg(&Group{ID: "cooltimer", Title: "Zeitprogramm Kühlen", Icon: "clock", Requires: "cooling"},
			it("700", "z1CoolingTimer", "Kühlen Zone 1").Timer().W()),
		reg(&Group{ID: "venttimer", Title: "Zeitprogramm Lüftung", Icon: "clock", Requires: "ventilation"},
			it("700", "ventilationTimer", "Lüftung").Timer().W()),
		reg(&Group{ID: "wptimer", Title: "Zeitprogramme Wärmepumpe", Icon: "clock", Requires: "heatpump"},
			it("700", "noiseReductionTimer", "Flüsterbetrieb").Timer().W().H("Zeitfenster mit reduzierter Leistung der Wärmepumpe (leiser)."),
			it("700", "tariffTimer", "Hochtarif").Timer().W().H("Zeitfenster mit Hochtarif; der Regler bevorzugt dann den günstigeren Wärmeerzeuger.")),
	)

	urlaub := reg(&Group{ID: "urlaub", Title: "Urlaub & Feiertage", Icon: "suitcase"},
		it("700", "HolidayStartPeriod", "Abwesenheit von").Date().W().Age(60).H("Während der Abwesenheit wird auf „Temperatur bei Abwesenheit“ geregelt, Warmwasser und Zirkulation sind aus, Frostschutz bleibt aktiv."),
		it("700", "HolidayEndPeriod", "Abwesenheit bis").Date().W().Age(60),
		it("700", "HolidayTemp", "Temperatur bei Abwesenheit").Num("°C", 5, 30, 0.5, 1).W().H("Raumsolltemperatur während der Abwesenheit."),
		it("700", "BankHolidayStartPeriod", "Zu Hause (Feiertag) von").Date().W().Age(60).H("In diesem Zeitraum gilt das Sonntags-Zeitprogramm."),
		it("700", "BankHolidayEndPeriod", "Zu Hause (Feiertag) bis").Date().W().Age(60),
	)
	urlaubGroups := []*Group{urlaub}
	for z := 1; z <= 3; z++ {
		zn, req := fmt.Sprintf("z%d", z), ""
		if z > 1 {
			req = fmt.Sprintf("zone%d", z)
		}
		urlaubGroups = append(urlaubGroups, reg(&Group{ID: "urlaub" + zn, Title: fmt.Sprintf("Urlaub Zone %d", z), Icon: "suitcase", Requires: req},
			it("700", zn+"HolidayStartPeriod", "Abwesenheit von").Date().W().Age(60).H("Während der Abwesenheit wird diese Zone auf „Temperatur bei Abwesenheit“ geregelt."),
			it("700", zn+"HolidayEndPeriod", "Abwesenheit bis").Date().W().Age(60),
			it("700", zn+"HolidayTemp", "Temperatur bei Abwesenheit").Num("°C", 5, 30, 0.5, 1).W().H("Raumsolltemperatur dieser Zone während der Abwesenheit."),
			it("700", zn+"BankHolidayStartPeriod", "Zu Hause (Feiertag) von").Date().W().Age(60).H("In diesem Zeitraum gilt für diese Zone das Sonntags-Zeitprogramm."),
			it("700", zn+"BankHolidayEndPeriod", "Zu Hause (Feiertag) bis").Date().W().Age(60),
		))
	}

	status := reg(&Group{ID: "status", Title: "Systemstatus", Icon: "info"},
		it("700", "DisplayedOutsideTemp", "Außentemperatur").Val("°C", 1),
		it("700", "OutsideTempAvg", "Mittlere Außentemperatur").Val("°C", 1).H("Gleitender Mittelwert, alle 3 h aktualisiert."),
		it("700", "WaterPressure", "Wasserdruck").Val("bar", 1).Fmt("pressure").H("Anlagendruck. Optimal 1,0–2,0 bar bei kalter Anlage; unter 0,8 bar Wasser nachfüllen."),
		it("700", "SystemFlowTemp", "Systemvorlauftemperatur").Val("°C", 1).H("Gemessen am Systemvorlauffühler (z. B. hinter einer hydraulischen Weiche)."),
		it("700", "MaintenanceDue", "Wartung fällig").Bool("yes", "no").Age(300),
		it("700", "MaintenanceDate", "Nächste Wartung").Date().W().Inst().Age(300).H("Vom Fachhandwerker gesetzt; ab diesem Datum zeigt der Regler „Wartung fällig“."),
	)
	verbrauch := reg(&Group{ID: "verbrauch", Title: "Verbrauch & Ertrag", Icon: "chart"},
		it("700", "PrFuelSumHcThisMonth", "Gas Heizen – aktueller Monat").Val("kWh", 0).Age(300).H("Vom Regler aus Brennerleistung und Laufzeit geschätzt, nicht gemessen."),
		it("700", "PrFuelSumHcLastMonth", "Gas Heizen – Vormonat").Val("kWh", 0).Age(300).H("Vom Regler aus Brennerleistung und Laufzeit geschätzt, nicht gemessen."),
		it("700", "PrFuelSumHc", "Gas Heizen – gesamt").Val("kWh", 0).Age(300).H("Vom Regler aus Brennerleistung und Laufzeit geschätzt, nicht gemessen."),
		it("700", "PrFuelSumHwcThisMonth", "Gas Warmwasser – aktueller Monat").Val("kWh", 0).Age(300).H("Vom Regler aus Brennerleistung und Laufzeit geschätzt, nicht gemessen."),
		it("700", "PrFuelSumHwcLastMonth", "Gas Warmwasser – Vormonat").Val("kWh", 0).Age(300).H("Vom Regler aus Brennerleistung und Laufzeit geschätzt, nicht gemessen."),
		it("700", "PrFuelSumHwc", "Gas Warmwasser – gesamt").Val("kWh", 0).Age(300).H("Vom Regler aus Brennerleistung und Laufzeit geschätzt, nicht gemessen."),
		it("700", "PrFuelSum", "Gas gesamt").Val("kWh", 0).Age(300).H("Vom Regler aus Brennerleistung und Laufzeit geschätzt, nicht gemessen."),
		it("700", "PrEnergySumHcThisMonth", "Strom Heizen – aktueller Monat").Val("kWh", 0).Age(300).H("Elektrische Energie der Wärmeerzeuger (Pumpe, Gebläse bzw. Wärmepumpe), vom Regler geschätzt."),
		it("700", "PrEnergySumHcLastMonth", "Strom Heizen – Vormonat").Val("kWh", 0).Age(300).H("Elektrische Energie der Wärmeerzeuger (Pumpe, Gebläse bzw. Wärmepumpe), vom Regler geschätzt."),
		it("700", "PrEnergySumHc", "Strom Heizen – gesamt").Val("kWh", 0).Age(300).H("Elektrische Energie der Wärmeerzeuger (Pumpe, Gebläse bzw. Wärmepumpe), vom Regler geschätzt."),
		it("700", "PrEnergySumHwcThisMonth", "Strom Warmwasser – aktueller Monat").Val("kWh", 0).Age(300).H("Elektrische Energie der Wärmeerzeuger (Pumpe, Gebläse bzw. Wärmepumpe), vom Regler geschätzt."),
		it("700", "PrEnergySumHwcLastMonth", "Strom Warmwasser – Vormonat").Val("kWh", 0).Age(300).H("Elektrische Energie der Wärmeerzeuger (Pumpe, Gebläse bzw. Wärmepumpe), vom Regler geschätzt."),
		it("700", "PrEnergySumHwc", "Strom Warmwasser – gesamt").Val("kWh", 0).Age(300).H("Elektrische Energie der Wärmeerzeuger (Pumpe, Gebläse bzw. Wärmepumpe), vom Regler geschätzt."),
		it("700", "PrEnergySum", "Strom gesamt").Val("kWh", 0).Age(300).H("Elektrische Energie der Wärmeerzeuger (Pumpe, Gebläse bzw. Wärmepumpe), vom Regler geschätzt."),
		it("700", "YieldTotal", "Umweltertrag gesamt").Val("kWh", 0).Age(300).H("Von einer Wärmepumpe genutzte Umweltwärme."),
		it("700", "SolarYieldTotal", "Solarertrag gesamt").Val("kWh", 0).Age(300).H("Ertrag einer Solarthermieanlage."),
	)
	kontakt := reg(&Group{ID: "kontakt", Title: "Kontakt Fachhandwerker", Icon: "phone"},
		it("700", "Installer1", "Firma (Teil 1)").Text(6).W().Inst().Age(300).H("Erste Hälfte des Firmennamens unter „Kontakt Fachhandwerker“ am Regler (max. 6 Zeichen)."),
		it("700", "Installer2", "Firma (Teil 2)").Text(6).W().Inst().Age(300).H("Zweite Hälfte des Firmennamens (max. 6 Zeichen)."),
		it("700", "PhoneNumber1", "Telefon (Teil 1)").Text(6).W().Inst().Age(300).H("Erste Hälfte der Telefonnummer (max. 6 Zeichen)."),
		it("700", "PhoneNumber2", "Telefon (Teil 2)").Text(6).W().Inst().Age(300).H("Zweite Hälfte der Telefonnummer (max. 6 Zeichen)."),
	)
	grundeinst := reg(&Group{ID: "grundeinstellungen", Title: "Grundeinstellungen", Icon: "settings"},
		it("700", "Date", "Datum").Date().W().Age(60).H("Wird bei DCF77-Empfang über den Außenfühler automatisch gestellt."),
		it("700", "Time", "Uhrzeit").Time().W().Age(10).H("Wird bei DCF77-Empfang über den Außenfühler automatisch gestellt."),
		it("700", "OpModeEffect", "Wirkung Grundanzeige").Enum("alle=Alle Zonen", "1=Zone 1", "2=Zone 2", "3=Zone 3").W().H("Welche Zonen Betriebsart und Schnellwahl der Grundanzeige betreffen."),
	)

	system := reg(&Group{ID: "system", Title: "Systemkonfiguration", Icon: "wrench"},
		it("700", "HydraulicScheme", "Systemschema").Val("", 0).Inst().Age(300).H("Nur am Regler änderbar."),
		it("700", "AdaptHeatCurve", "Adaptive Heizkurve").Bool("yes", "no").W().Inst().H("Automatische Korrektur der eingestellten Heizkurve anhand der Raumtemperatur; nur sinnvoll mit Raumaufschaltung."),
		it("700", "ContinuosHeating", "Durchheizen unter").Num("°C", -26, 10, 1, 0).W().Inst().H("Unterhalb dieser Außentemperatur wird ohne Absenkung durchgeheizt. −26 °C = aus."),
		it("700", "FrostOverRideTime", "Frostschutzverzögerung").Num("h", 0, 12, 1, 0).W().Inst().H("Verzögert den Frostschutz-Heizbetrieb, nachdem die Außentemperatur unter ca. 4 °C gefallen ist."),
		it("700", "MultiRelaySetting", "Multifunktionsausgang").Enum("circulation=Zirkulation", "dryer=Entfeuchtung", "zone=Zone", "legiopump=Legionellenschutzpumpe").W().Inst().H("Funktion des Multifunktionsausgangs im System: Zirkulationspumpe, Entfeuchter, Zonenventil oder Legionellenschutzpumpe."),
		it("700", "ManualCooling", "Manuelles Kühlen (Tage)").Num("d", 0, 99, 1, 0).W().Inst().H("Kühlen für die eingestellte Zahl an Tagen manuell erzwingen. 0 = aus."),
		it("700", "MaxRoomHumidity", "Max. Raumluftfeuchte").Val("%", 0).Inst().H("Grenzwert der Raumluftfeuchte; darüber schaltet ein Entfeuchter (Multifunktionsausgang „Entfeuchtung“) ein."),
	)
	wwInst := reg(&Group{ID: "warmwasserfhw", Title: "Warmwasser (Fachhandwerker)", Icon: "drop", Requires: "hwc"},
		it("700", "HwcMaxFlowTempDesired", "Max. Vorlauf Speicherladung").Num("°C", 55, 90, 1, 0).W().Inst().H("Höchste Vorlauftemperatur, die der Regler beim Laden des Speichers anfordert."),
		it("700", "HwcLockTime", "Sperrzeit Speicherladung").Num("min", 0, 120, 10, 0).W().Inst().H("Nach Erreichen der max. Speicherladezeit wird das Laden für diese Zeit gesperrt, damit die Heizkreise wieder Wärme bekommen."),
		it("700", "HwcParallelLoading", "Parallele Speicherladung").Bool("on", "off").W().Inst().H("Ein: Mischerkreise werden während der Speicherladung weiter versorgt. Aus: Warmwasser hat Vorrang, Heizkreise pausieren."),
		it("700", "MaxCylinderChargeTime", "Max. Speicherladezeit").Num("min", 20, 120, 10, 0).W().Inst().H("Maximale Dauer einer Speicherladung am Stück; danach folgt die Sperrzeit."),
		it("700", "CylinderChargeHyst", "Hysterese Speicherladung").Num("K", 3, 20, 0.5, 1).W().Inst().H("Nachladen beginnt, wenn die Speichertemperatur um diesen Wert unter die Solltemperatur fällt."),
		it("700", "CylinderChargeOffset", "Offset Speicherladung").Num("K", 15, 40, 1, 0).W().Inst().H("Vorlaufsollwert beim Speicherladen = Warmwasser-Solltemperatur + Offset."),
		it("700", "PumpAdditionalTime", "Nachlaufzeit Ladepumpe").Num("min", 0, 10, 1, 0).W().Inst().H("Nachlauf der Ladepumpe nach der Speicherladung, um Restwärme in den Speicher zu bringen."),
	)

	thermeLive := reg(&Group{ID: "thermelive", Title: "Therme – Live-Werte", Icon: "flame"},
		it("bai", "Statenumber", "Statuscode").Val("", 0).Fmt("statecode").Inst().Age(10).H("S-Code der Therme: aktueller Betriebszustand, z. B. Brennerbetrieb, Nachlauf, Sperrzeit."),
		it("bai", "Flame", "Flamme").Val("", 0).Fmt("flame").Inst().Age(10).H("Brenner brennt (Flammenüberwachung per Ionisation)."),
		it("bai", "FlowTemp", "d.40 Vorlauftemperatur").Val("°C", 1).Inst().Age(10).H("Gemessen am Vorlauffühler der Therme."),
		it("bai", "ReturnTemp", "d.41 Rücklauftemperatur").Val("°C", 1).Inst().Age(10).H("Gemessen am Rücklauffühler der Therme. Unter ca. 55 °C kondensiert das Abgas (Brennwertnutzung)."),
		it("bai", "FlowTempDesired", "d.05 Vorlaufsolltemperatur").Val("°C", 1).Inst().Age(10).H("Vorlaufsollwert, auf den die Therme gerade regelt (Anforderung des Reglers, begrenzt durch d.71)."),
		it("bai", "ModulationTempDesired", "Modulationssollwert").Val("%", 0).Inst().Age(10).H("Aktuelle Brennerleistung relativ zur Nennleistung."),
		it("bai", "FanSpeed", "d.34 Lüfterdrehzahl").Val("1/min", 0).Inst().Age(10).H("Gebläsedrehzahl; bestimmt die Gas-/Luftmenge und damit die Brennerleistung."),
		it("bai", "TargetFanSpeed", "d.33 Lüfter-Solldrehzahl").Val("1/min", 0).Inst().Age(10).H("Von der Elektronik vorgegebene Gebläsedrehzahl."),
		it("bai", "IonisationVoltageLevel", "d.44 Ionisationsspannung").Val("", 1).Inst().Age(10).H("Flammensignal der Ionisationselektrode; dient der Flammenüberwachung."),
		it("bai", "WaterPressure", "Wasserdruck").Val("bar", 2).Fmt("pressure").Inst().Age(10).H("Anlagendruck. Optimal 1,0–2,0 bar bei kalter Anlage; unter 0,8 bar Wasser nachfüllen."),
		it("bai", "PrimaryCircuitFlowrate", "d.29 Umlaufwassermenge").Val("l/min", 1).Inst().Age(10).H("Volumenstrom durch die Therme."),
		it("bai", "PumpPower", "d.15 Leistung Heizungspumpe").Val("%", 0).Inst().Age(10).H("Aktuelle Leistung der internen Heizungspumpe."),
		it("bai", "WP", "d.10 Heizungspumpe").Bool("on", "off").Inst().Age(10).H("Interne Heizungspumpe läuft."),
		it("bai", "extWP", "d.11 Externe Heizungspumpe").Bool("on", "off").Inst().Age(10).H("Über ein Zubehörrelais angesteuerte externe Heizungspumpe läuft."),
		it("bai", "CirPump", "d.13 Zirkulationspumpe").Bool("on", "off").Inst().Age(10).H("Über ein Zubehörrelais angesteuerte Zirkulationspumpe läuft."),
		it("bai", "DCRoomthermostat", "d.16 Raumthermostat (24 V)").Bool("on", "off").Inst().Age(10).H("Ein = ein 24-V-Raumthermostat an der Therme fordert Wärme an."),
		it("bai", "PositionValveSet", "d.35 Position Vorrangumschaltventil").Val("", 0).Inst().Age(10).H("Stellung des Vorrangumschaltventils zwischen Heizen und Warmwasser."),
		it("bai", "OutdoorstempSensor", "d.47 Außentemperaturfühler").Val("°C", 1).Inst().Age(10).H("Wert des direkt an der Therme angeschlossenen Außenfühlers."),
		it("bai", "RemainingBoilerblocktime", "d.67 Verbleibende Brennersperrzeit").Val("min", 0).Inst().Age(10).H("Restzeit der Brennersperre nach dem Abschalten (Taktschutz, siehe d.02)."),
		it("bai", "ExtFlowTempDesiredMin", "d.09 Min. ext. Vorlaufsollwert").Val("°C", 1).Inst().H("Vorlaufsollwert, den der externe Regler (VRC 700 über eBUS) anfordert."),
	)
	thermeSet := reg(&Group{ID: "thermeset", Title: "Therme – Diagnosecodes", Icon: "wrench"},
		it("bai", "PartloadHcKW", "d.00 Heizungsteillast").Num("kW", 1, 40, 1, 0).W().Inst().Age(300).H("Maximale Heizleistung im Heizbetrieb. Kleiner = längere Laufzeiten und weniger Takten; zu klein = Räume werden bei Kälte nicht warm. Nur innerhalb des Gerätebereichs einstellen."),
		it("bai", "WPPostrunTime", "d.01 Pumpennachlaufzeit Heizen").Num("min", 1, 60, 1, 0).W().Inst().Age(300).H("Nachlauf der Heizungspumpe nach Ende einer Wärmeanforderung."),
		it("bai", "BlockTimeHcMax", "d.02 Max. Brennersperrzeit").Num("min", 2, 60, 1, 0).W().Inst().Age(300).H("Wartezeit nach dem Abschalten des Brenners bis zum nächsten Start (Taktschutz). Gilt bei 20 °C Vorlaufsollwert, bei höherem Sollwert kürzer."),
		it("bai", "PumpPowerDesired", "d.14 Pumpenleistung Soll").Enum("auto=Automatisch", "53=53 %", "60=60 %", "70=70 %", "85=85 %", "100=100 %").Inst().Age(300).H("Leistungsvorgabe der internen Pumpe; Automatisch = Pumpe moduliert selbst. Nur am Gerät änderbar: die Werteliste der ebusd-Definition passt nicht zu allen Gerätewerten."),
		it("bai", "ReturnRegulation", "d.17 Rücklaufregelung").Bool("on", "off").W().Inst().Age(300).H("Ein: Therme regelt auf die Rücklauf- statt die Vorlauftemperatur (z. B. Fußbodenheizung)."),
		it("bai", "HcPumpMode", "d.18 Pumpenbetriebsart").Enum("post run=Nachlauf", "permanent=Weiterlaufend", "winter=Winter").W().Inst().Age(300).H("Nachlauf: Pumpe läuft nur bei Wärmeanforderung plus Nachlaufzeit d.01 (sparsam). Weiterlaufend: Pumpe läuft, solange der Heizbetrieb eingeschaltet ist. Winter: Pumpe läuft im Winterbetrieb durchgehend."),
		it("bai", "HwcTempMax", "d.20 Max. Warmwassertemperatur").Num("°C", 50, 70, 1, 0).W().Inst().Age(300).H("Obergrenze für die am Regler einstellbare Warmwassertemperatur."),
		it("bai", "HeatingSwitch", "d.23 Heizung ein (Winterbetrieb)").Bool("on", "off").W().Inst().Age(60).H("Aus = Sommerbetrieb: keine Heizung, nur Warmwasser."),
		it("bai", "HwcSwitch", "Warmwasserbereitung ein").Bool("on", "off").W().Inst().Age(60).H("Aus = keine Warmwasserbereitung."),
		it("bai", "OptionalRelais", "d.26 Optionales Relais").Enum(relay...).W().Inst().Age(300).H("Funktion des optionalen Relais auf der Geräteelektronik."),
		it("bai", "AccessoriesOne", "d.27 Zubehörrelais 1").Enum(relay...).W().Inst().Age(300).H("Funktion von Relais 1 eines Zubehörmoduls (z. B. VR 40)."),
		it("bai", "AccessoriesTwo", "d.28 Zubehörrelais 2").Enum(relay...).W().Inst().Age(300).H("Funktion von Relais 2 eines Zubehörmoduls (z. B. VR 40)."),
		it("bai", "FanSpeedOffsetMin", "d.50 Offset min. Lüfterdrehzahl").Val("1/min", 0).Inst().Age(300).H("Korrektur der minimalen Gebläsedrehzahl, nur durch den Kundendienst."),
		it("bai", "FanSpeedOffsetMax", "d.51 Offset max. Lüfterdrehzahl").Val("1/min", 0).Inst().Age(300).H("Korrektur der maximalen Gebläsedrehzahl, nur durch den Kundendienst."),
		it("bai", "FlowsetHcMax", "d.71 Max. Vorlauf Heizbetrieb").Num("°C", 45, 80, 1, 0).W().Inst().Age(300).H("Höchste Vorlauftemperatur der Therme im Heizbetrieb; begrenzt jeden Sollwert des Reglers."),
		it("bai", "StorageLoadTimeMax", "d.75 Max. Speicherladezeit").Num("min", 20, 90, 1, 0).W().Inst().Age(300).H("Maximale Ladezeit eines Speichers ohne eigene Regelung."),
		it("bai", "FlowsetHwcMax", "d.78 Max. Vorlauf Speicherladung").Num("°C", 55, 80, 1, 0).W().Inst().Age(300).H("Höchste Vorlauftemperatur der Therme bei Speicherladung."),
		it("bai", "HoursTillService", "d.84 Stunden bis Wartung").Num("h", 0, 3000, 10, 0).W().Inst().Age(300).H("Brennerstunden bis zur nächsten Wartungsmeldung."),
		it("bai", "AntiCondensValue", "d.85 Mindestleistung").Val("kW", 0).Inst().Age(300).H("Mindestleistung, damit sich im Abgasweg kein Kondensat bildet (Versottungsschutz)."),
		it("bai", "DSNOffset", "d.93 Gerätekennung").Val("", 0).Inst().Age(300).H("Gerätevariante, legt Typ und Leistung für die Elektronik fest. Nur nach Tausch der Leiterplatte einstellen."),
	)
	thermeDiag := reg(&Group{ID: "thermediag", Title: "Therme – Diagnose", Icon: "info"},
		it("bai", "Templimiter", "Sicherheitstemperaturbegrenzer ausgelöst").Bool("on", "off").Inst().Age(30).H("Der Sicherheitstemperaturbegrenzer hat wegen Übertemperatur abgeschaltet."),
		it("bai", "VolatileLockout", "Verriegelnde Störabschaltung").Bool("yes", "no").Inst().Age(30).H("Die Therme ist nach einer Störung verriegelt und startet erst nach Entstörung wieder."),
		it("bai", "ExternalFaultmessage", "Externe Störmeldung").Bool("on", "off").Inst().Age(30).H("Eine Störmeldung über den externen Eingang liegt an."),
		it("bai", "FloorHeatingContact", "Anlegethermostat (Fußbodenkontakt)").Bool("on", "off").Inst().Age(30).H("Zustand des Kontakts für einen Fußboden-Anlegethermostaten (Übertemperaturschutz)."),
		it("bai", "TimerInputHc", "Eingang Schaltuhr").Bool("on", "off").Inst().Age(30).H("Zustand des Eingangs für eine externe Schaltuhr."),
		it("bai", "Ignitor", "Zünder").Bool("on", "off").Inst().Age(10).H("Zündung aktiv."),
		it("bai", "GasvalveUC", "Gasventil").Bool("on", "off").Inst().Age(10).H("Gasventil offen."),
		it("bai", "Fluegasvalve", "Abgasklappe").Bool("on", "off").Inst().Age(30).H("Ansteuerung einer Abgasklappe (falls vorhanden)."),
		it("bai", "EbusVoltage", "eBUS-Spannung").Bool("on", "off").Inst().Age(60).H("eBUS-Versorgungsspannung liegt an."),
		it("bai", "dcfState", "DCF77-Funkuhr").Enum("valid=Empfang ok", "nosignal=Kein Signal", "receiving=Empfang läuft", "synchronized=Synchronisiert").Inst().Age(60).H("Empfangsstatus der DCF77-Funkuhr im Außenfühler."),
		it("bai", "DCFTimeDate", "DCF77-Zeit").Text(0).Inst().Age(60).H("Per DCF77 empfangene Uhrzeit und Datum."),
		it("bai", "HwcTemp", "d.03 Warmwasser-Auslauftemperatur").Val("°C", 1).Inst().Age(30).H("Warmwasser-Auslauftemperatur; nur bei Kombithermen belegt."),
		it("bai", "StorageTemp", "d.04 Speichertemperatur").Val("°C", 1).Inst().Age(30).H("Speicherfühler an der Therme; nur bei direkt angeschlossenem Speicher belegt."),
		it("bai", "HwcDemand", "d.22 Warmwasseranforderung").Bool("yes", "no").Inst().Age(30).H("Eine Warmwasseranforderung liegt an."),
		it("bai", "HcUnderHundredStarts", "Brennerstarts < 100 s (Heizen)").Val("", 0).Inst().Age(300).H("Brennerstarts mit weniger als 100 s Laufzeit. Ein hoher Anteil an allen Starts zeigt Takten."),
		it("bai", "FanStarts", "Lüfterstarts").Val("", 0).Fmt("counter").Inst().Age(300).H("Gesamtzahl der Gebläsestarts."),
		it("bai", "BoilerType", "Gerätetyp").Val("", 0).Inst().Age(3600).H("Interne Gerätetypnummer."),
	)
	thermeStat := reg(&Group{ID: "thermestat", Title: "Therme – Statistik", Icon: "chart"},
		it("bai", "HcHours", "d.80 Betriebsstunden Heizen").Val("h", 0).Inst().Age(300).H("Brennerlaufzeit im Heizbetrieb."),
		it("bai", "HwcHours", "d.81 Betriebsstunden Warmwasser").Val("h", 0).Inst().Age(300).H("Brennerlaufzeit im Warmwasserbetrieb."),
		it("bai", "HcStarts", "d.82 Brennerstarts Heizen").Val("", 0).Fmt("counter").Inst().Age(300).H("Brennerstarts im Heizbetrieb; Starts pro Betriebsstunde zeigen, wie stark die Therme taktet."),
		it("bai", "HwcStarts", "d.83 Brennerstarts Warmwasser").Val("", 0).Fmt("counter").Inst().Age(300).H("Brennerstarts im Warmwasserbetrieb."),
		it("bai", "PumpHours", "Betriebsstunden Pumpe").Val("h", 0).Inst().Age(300).H("Laufzeit der internen Heizungspumpe."),
		it("bai", "HcPumpStarts", "Pumpenstarts").Val("", 0).Fmt("counter").Inst().Age(300).H("Starts der internen Heizungspumpe."),
		it("bai", "FanHours", "Betriebsstunden Lüfter").Val("h", 0).Inst().Age(300).H("Laufzeit des Gebläses."),
		it("bai", "ValveStarts", "Schaltspiele Umschaltventil").Val("", 0).Fmt("counter").Inst().Age(300).H("Umschaltungen des Vorrangumschaltventils zwischen Heizen und Warmwasser."),
		it("bai", "DeactivationsTemplimiter", "d.60 STB-Abschaltungen").Val("", 0).Inst().Age(300).H("Anzahl der Abschaltungen durch den Sicherheitstemperaturbegrenzer."),
		it("bai", "DeactivationsIFC", "d.61 Zündfehler").Val("", 0).Inst().Age(300).H("Anzahl der Störabschaltungen, weil die Zündung auch im letzten Versuch fehlschlug."),
		it("bai", "averageIgnitiontime", "d.64 Mittlere Zündzeit").Val("s", 1).Inst().Age(300).H("Durchschnittliche Zeit vom Zündbeginn bis zur Flammenerkennung."),
		it("bai", "maxIgnitiontime", "d.65 Max. Zündzeit").Val("s", 1).Inst().Age(300).H("Längste Zeit vom Zündbeginn bis zur Flammenerkennung."),
		it("bai", "minIgnitiontime", "Min. Zündzeit").Val("s", 1).Inst().Age(300).H("Kürzeste Zeit vom Zündbeginn bis zur Flammenerkennung."),
		it("bai", "CounterStartattempts1", "d.68 Zündung erst im 1. Versuch fehlgeschlagen").Val("", 0).Inst().Age(300).H("Brennerstarts, bei denen der 1. Zündversuch fehlschlug."),
		it("bai", "CounterStartattempts2", "d.69 Zündung erst im 2. Versuch fehlgeschlagen").Val("", 0).Inst().Age(300).H("Brennerstarts, bei denen auch der 2. Zündversuch fehlschlug."),
		it("bai", "CounterStartAttempts3", "Zündung erst im 3. Versuch fehlgeschlagen").Val("", 0).Inst().Age(300).H("Brennerstarts, bei denen auch der 3. Zündversuch fehlschlug."),
		it("bai", "CounterStartAttempts4", "Zündung erst im 4. Versuch fehlgeschlagen").Val("", 0).Inst().Age(300).H("Brennerstarts, bei denen auch der 4. Zündversuch fehlschlug."),
		it("bai", "FlowTempMax", "Max. gemessene Vorlauftemperatur").Val("°C", 1).Inst().Age(300).H("Höchste je gemessene Vorlauftemperatur."),
		it("bai", "ReturnTempMax", "Max. gemessene Rücklauftemperatur").Val("°C", 1).Inst().Age(300).H("Höchste je gemessene Rücklauftemperatur."),
		it("bai", "FanMinSpeedOperation", "Min. Lüfterdrehzahl").Val("1/min", 0).Inst().Age(300).H("Untere Gebläsedrehzahlgrenze im Betrieb."),
		it("bai", "FanMaxSpeedOperation", "Max. Lüfterdrehzahl").Val("1/min", 0).Inst().Age(300).H("Obere Gebläsedrehzahlgrenze im Betrieb."),
		it("bai", "DSN", "Gerätekennung (DSN)").Val("", 0).Inst().Age(3600).H("Gerätekennung; legt Gerätetyp und Leistung fest."),
	)

	c.Sections = []*Section{
		{ID: "bedienung", Title: "Bedienung", Level: "user", Groups: append(append([]*Group{grund}, zoneUser...), ww, kuehlen, lueftung)},
		{ID: "zeitprogramme", Title: "Zeitprogramme", Level: "user", Groups: timerGroups},
		{ID: "urlaub", Title: "Urlaub", Level: "user", Groups: urlaubGroups},
		{ID: "information", Title: "Information", Level: "user", Groups: []*Group{status, verbrauch, kontakt}},
		{ID: "einstellungen", Title: "Einstellungen", Level: "user", Groups: []*Group{grundeinst}},
		{ID: "fachhandwerker", Title: "Fachhandwerkerebene", Level: "installer", Groups: append(append(append([]*Group{}, zoneInst...), system, wwInst), thermeLive, thermeSet, thermeDiag, thermeStat)},
	}
	c.Dashboard = []string{
		"700.DisplayedOutsideTemp", "700.z1ActualRoomTempDesired", "700.z1RoomTemp",
		"700.OpMode", "700.SFMode", "700.z1DayTemp", "700.z1NightTemp", "700.z1QuickVetoTemp",
		"700.Hc1ActualFlowTempDesired",
		"bai.FlowTemp", "bai.ReturnTemp", "bai.WaterPressure", "bai.Flame", "bai.WP",
		"bai.Statenumber", "bai.ModulationTempDesired", "700.PrFuelSumHcThisMonth",
		"700.HolidayStartPeriod", "700.HolidayEndPeriod",
	}
	for _, id := range c.Dashboard {
		if c.Items[id] == nil {
			panic("dashboard item missing: " + id)
		}
	}
	return c
}
