package server

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/L480/vaillant-ui/internal/catalog"
)

var sensorFault = map[string]string{"circuit": "Kurzschluss", "cutoff": "Unterbrechung", "short": "Kurzschluss", "undef": "undefiniert"}

var Days = []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}

type Slot [2]string

func isEmpty(s string) bool {
	switch strings.TrimSpace(s) {
	case "", "-", "empty", "-:-", "-.-.-", "--:--", "-:-:-":
		return true
	}
	return false
}

func firstField(raw string) string {
	f, _, _ := strings.Cut(raw, ";")
	return strings.TrimSpace(f)
}

func decode(it *catalog.Item, raw string) (any, error) {
	switch it.Kind {
	case "number":
		f := firstField(raw)
		if isEmpty(f) {
			return nil, nil
		}
		for _, st := range strings.Split(raw, ";")[1:] {
			if t, bad := sensorFault[strings.TrimSpace(st)]; bad {
				return nil, fmt.Errorf("Fühlerfehler: %s", t)
			}
		}
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			return nil, fmt.Errorf("kein Zahlenwert: %q", f)
		}
		return math.Round(v*10000) / 10000, nil
	case "enum":
		return firstField(raw), nil
	case "bool":
		f := firstField(raw)
		switch f {
		case it.BoolRaw[0]:
			return true, nil
		case it.BoolRaw[1]:
			return false, nil
		}
		if isEmpty(f) {
			return nil, nil
		}
		return nil, fmt.Errorf("unbekannter Wert %q", f)
	case "date":
		f := firstField(raw)
		if isEmpty(f) {
			return nil, nil
		}
		t, err := time.Parse("02.01.2006", f)
		if err != nil {
			return nil, fmt.Errorf("ungültiges Datum %q", f)
		}
		return t.Format("2006-01-02"), nil
	case "time":
		f := firstField(raw)
		if isEmpty(f) {
			return nil, nil
		}
		if len(f) >= 5 {
			return f[:5], nil
		}
		return f, nil
	case "text":
		return strings.TrimRight(raw, " "), nil
	}
	return raw, nil
}

func decodeTimerDay(raw string) ([]Slot, error) {
	parts := strings.Split(raw, ";")
	if len(parts)%2 != 0 {
		return nil, fmt.Errorf("ungültiges Zeitprogramm %q", raw)
	}
	slots := []Slot{}
	for i := 0; i+1 < len(parts); i += 2 {
		from, to := strings.TrimSpace(parts[i]), strings.TrimSpace(parts[i+1])
		if isEmpty(from) || isEmpty(to) {
			continue
		}
		slots = append(slots, Slot{from, to})
	}
	return slots, nil
}

var timeRe = regexp.MustCompile(`^([01]\d|2[0-4]):([0-5]\d)$`)

func minutes(s string) (int, bool) {
	m := timeRe.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	h, _ := strconv.Atoi(m[1])
	mi, _ := strconv.Atoi(m[2])
	if h == 24 && mi != 0 {
		return 0, false
	}
	return h*60 + mi, true
}

func encodeTimerDay(slots []Slot) (string, error) {
	if len(slots) > 3 {
		return "", fmt.Errorf("maximal 3 Zeitfenster pro Tag")
	}
	sorted := append([]Slot{}, slots...)
	sort.Slice(sorted, func(i, j int) bool { a, _ := minutes(sorted[i][0]); b, _ := minutes(sorted[j][0]); return a < b })
	prevEnd := -1
	out := make([]string, 0, 6)
	for _, s := range sorted {
		a, ok1 := minutes(s[0])
		b, ok2 := minutes(s[1])
		if !ok1 || !ok2 {
			return "", fmt.Errorf("ungültige Uhrzeit in %s–%s", s[0], s[1])
		}
		if a%10 != 0 || b%10 != 0 {
			return "", fmt.Errorf("Zeiten nur in 10-Minuten-Schritten (%s–%s)", s[0], s[1])
		}
		if a >= b {
			return "", fmt.Errorf("Beginn muss vor Ende liegen (%s–%s)", s[0], s[1])
		}
		if a < prevEnd {
			return "", fmt.Errorf("Zeitfenster überschneiden sich")
		}
		prevEnd = b
		out = append(out, s[0], s[1])
	}
	for len(out) < 6 {
		out = append(out, "-:-")
	}
	return strings.Join(out, ";"), nil
}

var textRe = regexp.MustCompile(`^[A-Za-z0-9 .,+\-/&()]*$`)

// encode validates a JSON value against the item and returns the ebusd write argument.
func encode(it *catalog.Item, msg json.RawMessage) (string, error) {
	if strings.TrimSpace(string(msg)) == "null" {
		return "", fmt.Errorf("Wert fehlt")
	}
	switch it.Kind {
	case "number":
		var v float64
		if err := json.Unmarshal(msg, &v); err != nil {
			return "", fmt.Errorf("Zahl erwartet")
		}
		if it.Min != nil && v < *it.Min-1e-9 || it.Max != nil && v > *it.Max+1e-9 {
			return "", fmt.Errorf("Wert muss zwischen %s und %s liegen", fnum(*it.Min, it.Decimals), fnum(*it.Max, it.Decimals))
		}
		if it.Step != nil && *it.Step > 0 {
			base := 0.0
			if it.Min != nil {
				base = *it.Min
			}
			n := (v - base) / *it.Step
			if math.Abs(n-math.Round(n)) > 1e-6 {
				return "", fmt.Errorf("nur Schritte von %s erlaubt", fnum(*it.Step, it.Decimals))
			}
		}
		return fnum(v, it.Decimals), nil
	case "enum":
		var v string
		if err := json.Unmarshal(msg, &v); err != nil {
			return "", fmt.Errorf("Text erwartet")
		}
		for _, o := range it.Options {
			if o.Value == v {
				return v, nil
			}
		}
		return "", fmt.Errorf("ungültige Auswahl %q", v)
	case "bool":
		var v bool
		if err := json.Unmarshal(msg, &v); err != nil {
			return "", fmt.Errorf("true/false erwartet")
		}
		if v {
			return it.BoolRaw[0], nil
		}
		return it.BoolRaw[1], nil
	case "date":
		var v string
		if err := json.Unmarshal(msg, &v); err != nil {
			return "", fmt.Errorf("Datum erwartet")
		}
		t, err := time.Parse("2006-01-02", v)
		if err != nil || t.Year() < 2000 || t.Year() > 2099 {
			return "", fmt.Errorf("ungültiges Datum %q", v)
		}
		return t.Format("02.01.2006"), nil
	case "time":
		var v string
		if err := json.Unmarshal(msg, &v); err != nil {
			return "", fmt.Errorf("Uhrzeit erwartet")
		}
		m, ok := minutes(v)
		if !ok || m >= 24*60 {
			return "", fmt.Errorf("ungültige Uhrzeit %q", v)
		}
		return v + ":00", nil
	case "text":
		var v string
		if err := json.Unmarshal(msg, &v); err != nil {
			return "", fmt.Errorf("Text erwartet")
		}
		if it.MaxLen > 0 && len(v) > it.MaxLen {
			return "", fmt.Errorf("maximal %d Zeichen", it.MaxLen)
		}
		if !textRe.MatchString(v) {
			return "", fmt.Errorf("nur Buchstaben A–Z, Ziffern und . , + - / & ( ) erlaubt")
		}
		return v, nil
	}
	return "", fmt.Errorf("nicht schreibbar")
}

func fnum(v float64, dec int) string {
	return strconv.FormatFloat(v, 'f', dec, 64)
}
