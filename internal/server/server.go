package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/L480/vaillant-ui/internal/catalog"
	"github.com/L480/vaillant-ui/internal/ebusd"
)

type Server struct {
	Bus      *ebusd.Client
	Catalog  *catalog.Catalog
	ReadOnly bool
	Version  string
	Static   fs.FS
	// AllowedHosts are hostnames accepted for state-changing requests besides IP literals and localhost.
	AllowedHosts []string

	mu        sync.RWMutex
	writeDefs map[string]bool
	defsAt    time.Time
	refreshMu sync.Mutex
	meta      *Meta
	metaAt    time.Time

	pollEvery time.Duration
	settle    time.Duration
	enfMu     sync.Mutex
	enfCancel context.CancelFunc
	enfDone   chan struct{}
	enfWG     sync.WaitGroup
}

type Meta struct {
	ReadOnly bool           `json:"readonly"`
	Version  string         `json:"version"`
	Features map[string]any `json:"features"`
	Devices  []ebusd.Device `json:"devices"`
	Ebusd    ebusd.Info     `json:"ebusd"`
}

type Value struct {
	Value any    `json:"value"`
	Text  string `json:"text,omitempty"`
	Error string `json:"error,omitempty"`
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/meta", s.handleMeta)
	mux.HandleFunc("GET /api/catalog", s.handleCatalog)
	mux.HandleFunc("GET /api/values", s.handleValues)
	mux.HandleFunc("PUT /api/values/{id}", s.handlePut)
	mux.HandleFunc("GET /api/errors", s.handleErrors)
	mux.HandleFunc("GET /api/dump", s.handleDump)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })

	mux.HandleFunc("GET /status", s.legacyStatus)
	mux.HandleFunc("GET /pump-status", s.legacyPump)
	mux.HandleFunc("POST /on", s.legacyOpMode("auto", "off", "Heizung ist nun eingeschaltet."))
	mux.HandleFunc("POST /off", s.legacyOpMode("off", "auto", "Heizung ist nun ausgeschaltet."))
	for path, a := range map[string][3]string{
		"/auto":        {"SFMode", "auto", "Heizung ist nun im Automodus."},
		"/party":       {"SFMode", "party", "Heizung ist nun im Partymodus."},
		"/ventilation": {"SFMode", "ventilation", "Heizung ist nun im Lüftungsmodus."},
	} {
		mux.HandleFunc("POST "+path, s.legacyWrite(a[0], a[1], a[2]))
	}
	if s.Static != nil {
		mux.Handle("GET /", http.FileServerFS(s.Static))
	}
	return s.guard(mux)
}

// guard rejects cross-site state changes: browsers always send Origin on non-GET requests.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'")
		if !s.hostAllowed(r.Host) {
			errJSON(w, http.StatusForbidden, "Unbekannter Host "+r.Host+" (ALLOWED_HOSTS)")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if o := r.Header.Get("Origin"); o != "" {
				u, err := url.Parse(o)
				if err != nil || u.Host != r.Host {
					errJSON(w, http.StatusForbidden, "Cross-Origin-Anfrage abgelehnt")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// hostAllowed blocks DNS rebinding: a rebound name arrives with its own Host header.
func (s *Server) hostAllowed(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host == "localhost" || net.ParseIP(host) != nil {
		return true
	}
	for _, a := range s.AllowedHosts {
		if strings.EqualFold(a, host) {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func errJSON(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (s *Server) loadWriteDefs(_ context.Context) map[string]bool {
	fresh := func() (map[string]bool, bool) {
		s.mu.RLock()
		defer s.mu.RUnlock()
		ttl := 5 * time.Minute
		if len(s.writeDefs) == 0 {
			ttl = 15 * time.Second
		}
		return s.writeDefs, s.writeDefs != nil && time.Since(s.defsAt) < ttl
	}
	if d, ok := fresh(); ok {
		return d
	}
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	d, ok := fresh()
	if ok {
		return d
	}
	// Detached: a cancelled client request must not poison the shared cache.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	nd, err := s.Bus.WriteDefs(ctx)
	if err != nil {
		log.Printf("find -w: %v", err)
		if d != nil {
			nd = d
		} else {
			nd = map[string]bool{}
		}
	}
	s.mu.Lock()
	s.writeDefs, s.defsAt = nd, time.Now()
	s.mu.Unlock()
	return nd
}

func (s *Server) writable(ctx context.Context, it *catalog.Item) bool {
	if s.ReadOnly || !it.AllowWrite {
		return false
	}
	defs := s.loadWriteDefs(ctx)
	if it.Kind == "timer" {
		return defs[it.Circuit+"."+it.Name+".Monday"]
	}
	return defs[it.ID]
}

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	items := make(map[string]*catalog.Item, len(s.Catalog.Items))
	for id, it := range s.Catalog.Items {
		cp := *it
		cp.Writable = s.writable(r.Context(), it)
		items[id] = &cp
	}
	writeJSON(w, 200, catalog.Catalog{Sections: s.Catalog.Sections, Dashboard: s.Catalog.Dashboard, Items: items})
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	m, err := s.getMeta(r.Context())
	if err != nil {
		errJSON(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, 200, m)
}

func (s *Server) getMeta(_ context.Context) (*Meta, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s.mu.RLock()
	m, at := s.meta, s.metaAt
	s.mu.RUnlock()
	if m != nil && time.Since(at) < 10*time.Minute {
		return m, nil
	}
	inf, err := s.Bus.Info(ctx)
	if err != nil {
		if m != nil {
			return m, nil
		}
		return nil, err
	}
	notEmpty := func(c, n string) bool {
		raw, err := s.Bus.Read(ctx, c, n, 3600, "")
		return err == nil && !isEmpty(firstField(raw)) && raw != "inactive" && raw != "none"
	}
	f := map[string]any{
		"zones":   []int{1},
		"zone2":   notEmpty("700", "Hc2CircuitType"),
		"zone3":   notEmpty("700", "Hc3CircuitType"),
		"hwc":     notEmpty("700", "HwcStorageTemp"),
		"cooling": false, "ventilation": false, "heatpump": false,
	}
	for _, d := range inf.Devices {
		id := strings.ToUpper(d.ID)
		switch {
		case strings.HasPrefix(id, "HMU"), strings.HasPrefix(id, "VWZ"):
			f["heatpump"], f["cooling"] = true, true
		case strings.HasPrefix(id, "VWL"), strings.Contains(id, "RECOV"):
			f["ventilation"] = true
		}
	}
	zones := []int{1}
	if f["zone2"] == true {
		zones = append(zones, 2)
	}
	if f["zone3"] == true {
		zones = append(zones, 3)
	}
	f["zones"] = zones
	m = &Meta{ReadOnly: s.ReadOnly, Version: s.Version, Features: f, Devices: inf.Devices, Ebusd: inf}
	s.mu.Lock()
	s.meta, s.metaAt = m, time.Now()
	s.mu.Unlock()
	return m, nil
}

func (s *Server) readItem(ctx context.Context, it *catalog.Item, force bool) Value {
	age := it.MaxAge
	if force {
		age = -1
	}
	if it.Kind == "timer" {
		week := map[string][]Slot{}
		for _, d := range Days {
			raw, err := s.Bus.Read(ctx, it.Circuit, it.Name+"."+d, age, "")
			if err != nil {
				return Value{Error: readErr(err)}
			}
			slots, err := decodeTimerDay(raw)
			if err != nil {
				return Value{Error: err.Error()}
			}
			week[d] = slots
		}
		return Value{Value: week}
	}
	raw, err := s.Bus.Read(ctx, it.Circuit, it.Name, age, "")
	if err != nil {
		return Value{Error: readErr(err)}
	}
	v, err := decode(it, raw)
	if err != nil {
		return Value{Error: err.Error()}
	}
	out := Value{Value: v}
	if n, ok := v.(float64); ok {
		switch it.Format {
		case "statecode":
			out.Text = stateText(int(n))
		case "flame":
			if n == 240 || n == 0 {
				out.Text = "Aus"
			} else {
				out.Text = "An"
			}
		}
	}
	return out
}

func readErr(err error) string {
	var e *ebusd.Error
	if errors.As(err, &e) {
		switch {
		case strings.Contains(e.Msg, "not found"):
			return "Nicht verfügbar"
		case strings.Contains(e.Msg, "no answer"), strings.Contains(e.Msg, "timed out"):
			return "Keine Antwort vom Gerät"
		}
		return "Lesefehler: " + e.Msg
	}
	return err.Error()
}

func (s *Server) handleValues(w http.ResponseWriter, r *http.Request) {
	ids := strings.Split(r.URL.Query().Get("ids"), ",")
	if len(ids) > 120 {
		errJSON(w, 400, "zu viele Werte angefragt")
		return
	}
	force := r.URL.Query().Get("fresh") == "1"
	vals := map[string]Value{}
	for _, id := range ids {
		if r.Context().Err() != nil {
			return
		}
		if id == "" {
			continue
		}
		it := s.Catalog.Items[id]
		if it == nil {
			vals[id] = Value{Error: "Unbekannter Parameter"}
			continue
		}
		vals[id] = s.readItem(r.Context(), it, force)
	}
	writeJSON(w, 200, map[string]any{"time": time.Now().Format(time.RFC3339), "values": vals})
}

func (s *Server) handlePut(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	it := s.Catalog.Items[id]
	if it == nil {
		errJSON(w, 404, "Unbekannter Parameter")
		return
	}
	if s.ReadOnly {
		errJSON(w, 403, "Nur-Lese-Modus")
		return
	}
	if !s.writable(r.Context(), it) {
		errJSON(w, 403, "Dieser Parameter ist nicht schreibbar")
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		errJSON(w, 415, "JSON erwartet")
		return
	}
	var body struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil || len(body.Value) == 0 {
		errJSON(w, 400, "Ungültige Anfrage")
		return
	}

	type op struct{ name, arg string }
	var ops []op
	if it.Kind == "timer" {
		var week map[string][]Slot
		if err := json.Unmarshal(body.Value, &week); err != nil || len(week) == 0 {
			errJSON(w, 400, "Zeitprogramm erwartet")
			return
		}
		for _, d := range Days {
			slots, ok := week[d]
			if !ok {
				continue
			}
			if slots == nil {
				errJSON(w, 400, dayDE[d]+": Zeitfenster fehlen")
				return
			}
			arg, err := encodeTimerDay(slots)
			if err != nil {
				errJSON(w, 400, dayDE[d]+": "+err.Error())
				return
			}
			ops = append(ops, op{it.Name + "." + d, arg})
		}
		if len(ops) != len(week) {
			errJSON(w, 400, "Unbekannter Wochentag")
			return
		}
	} else {
		arg, err := encode(it, body.Value)
		if err != nil {
			errJSON(w, 400, err.Error())
			return
		}
		ops = append(ops, op{it.Name, arg})
	}

	// Detached so an aborted client can't leave a half-written week program.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Minute)
	defer cancel()
	for _, o := range ops {
		log.Printf("write %s %s = %q (from %s)", it.Circuit, o.name, o.arg, r.RemoteAddr)
		if err := s.Bus.Write(ctx, it.Circuit, o.name, o.arg); err != nil {
			log.Printf("write %s %s failed: %v", it.Circuit, o.name, err)
			errJSON(w, http.StatusBadGateway, "Schreiben fehlgeschlagen: "+err.Error())
			return
		}
	}
	v := s.readItem(ctx, it, true)
	if v.Error != "" {
		errJSON(w, http.StatusBadGateway, "Geschrieben, aber Rücklesen fehlgeschlagen: "+v.Error)
		return
	}
	writeJSON(w, 200, v)
}

var dayDE = map[string]string{"Monday": "Montag", "Tuesday": "Dienstag", "Wednesday": "Mittwoch", "Thursday": "Donnerstag", "Friday": "Freitag", "Saturday": "Samstag", "Sunday": "Sonntag"}

type ErrorEntry struct {
	Index  *int   `json:"index,omitempty"`
	Code   int    `json:"code"`
	Text   string `json:"text"`
	Source string `json:"source"`
	Date   string `json:"date,omitempty"`
	Time   string `json:"time,omitempty"`
}

func (s *Server) handleErrors(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := []ErrorEntry{}
	sources := []string{"700", "bai"}
	unreadable := []string{}
	for _, c := range sources {
		raw, err := s.Bus.Read(ctx, c, "currenterror", 10, "")
		if err != nil {
			unreadable = append(unreadable, c)
			continue
		}
		for _, f := range strings.Split(raw, ";") {
			if n, err := strconv.Atoi(strings.TrimSpace(f)); err == nil {
				current = append(current, ErrorEntry{Code: n, Text: errorText(n), Source: c})
			}
		}
	}
	if len(unreadable) == len(sources) {
		errJSON(w, http.StatusBadGateway, "Fehlerstatus konnte nicht gelesen werden")
		return
	}
	history := []ErrorEntry{}
	for i := 0; i < 10; i++ {
		raw, err := s.Bus.Read(ctx, "700", "errorhistory", 300, strconv.Itoa(i))
		if err != nil {
			break
		}
		f := strings.Split(raw, ";")
		if len(f) < 4 {
			break
		}
		n, err := strconv.Atoi(strings.TrimSpace(f[3]))
		if err != nil {
			break
		}
		idx := i
		e := ErrorEntry{Index: &idx, Code: n, Text: errorText(n), Source: "700", Time: f[1]}
		if t, err := time.Parse("02.01.2006", f[2]); err == nil {
			e.Date = t.Format("2006-01-02")
		}
		history = append(history, e)
	}
	writeJSON(w, 200, map[string]any{"current": current, "history": history, "unreadable": unreadable})
}

func (s *Server) legacyStatus(w http.ResponseWriter, r *http.Request) {
	raw, err := s.Bus.Read(r.Context(), "700", "OpMode", 30, "")
	if err != nil {
		http.Error(w, "Es ist ein Fehler aufgetreten.", 500)
		return
	}
	if strings.TrimSpace(raw) != "off" {
		w.Write([]byte("1"))
		return
	}
	v, err := s.desiredTemp(r.Context(), 30)
	if err != nil {
		log.Printf("legacy status: %v", err)
	}
	if err == nil && v > frostTemp {
		w.Write([]byte("1"))
	} else {
		w.Write([]byte("0"))
	}
}

const frostTemp = 5

func (s *Server) desiredTemp(ctx context.Context, maxAge int) (float64, error) {
	raw, err := s.Bus.Read(ctx, "700", "z1ActualRoomTempDesired", maxAge, "")
	if err != nil {
		return 0, err
	}
	f, _, _ := strings.Cut(raw, ";")
	return strconv.ParseFloat(strings.TrimSpace(f), 64)
}

func (s *Server) legacyPump(w http.ResponseWriter, r *http.Request) {
	raw, err := s.Bus.Read(r.Context(), "bai", "WP", 30, "")
	if err != nil {
		http.Error(w, "Es ist ein Fehler aufgetreten.", 500)
		return
	}
	if strings.TrimSpace(raw) == "on" {
		w.Write([]byte("1"))
	} else {
		w.Write([]byte("0"))
	}
}

func (s *Server) legacyWrite(name, value, msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.ReadOnly {
			http.Error(w, "Nur-Lese-Modus", http.StatusForbidden)
			return
		}
		log.Printf("write 700 %s = %q (legacy %s from %s)", name, value, r.URL.Path, r.RemoteAddr)
		if err := s.Bus.Write(r.Context(), "700", name, value); err != nil {
			log.Printf("legacy write failed: %v", err)
			http.Error(w, "Es ist ein Fehler aufgetreten.", 500)
			return
		}
		w.Write([]byte(msg))
	}
}

func (s *Server) legacyOpMode(target, other, msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.ReadOnly {
			http.Error(w, "Nur-Lese-Modus", http.StatusForbidden)
			return
		}
		path := r.URL.Path
		ctx, done := s.supersede()
		log.Printf("write 700 OpMode = %q (legacy %s from %s)", target, path, r.RemoteAddr)
		if err := s.Bus.Write(r.Context(), "700", "OpMode", target); err != nil {
			log.Printf("legacy write failed: %v", err)
			done()
			http.Error(w, "Es ist ein Fehler aufgetreten.", 500)
			return
		}
		w.Write([]byte(msg))
		s.enfWG.Add(1)
		go func() {
			defer s.enfWG.Done()
			defer done()
			s.enforce(ctx, path, target, other)
		}()
	}
}

// supersede cancels the running enforcement, waits for it, and registers a new one.
func (s *Server) supersede() (context.Context, func()) {
	s.enfMu.Lock()
	defer s.enfMu.Unlock()
	if s.enfCancel != nil {
		s.enfCancel()
		<-s.enfDone
	}
	settle := s.settle
	if settle == 0 {
		settle = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*settle+30*time.Second)
	done := make(chan struct{})
	s.enfCancel, s.enfDone = cancel, done
	return ctx, func() {
		cancel()
		close(done)
	}
}

// The VRC 700 can keep OpMode=off without applying it; only a real transition does.
func (s *Server) enforce(ctx context.Context, path, target, other string) {
	logf := func(format string, a ...any) { log.Printf("legacy %s: "+format, append([]any{path}, a...)...) }
	start := time.Now()
	ok, v, err := s.waitEffective(ctx, target)
	if err != nil {
		if ctx.Err() == nil {
			logf("enforcement aborted: %v", err)
		}
		return
	}
	if ok {
		logf("applied after %s", time.Since(start).Round(time.Millisecond))
		return
	}
	logf("controller ignored OpMode=%s (z1ActualRoomTempDesired=%s), forcing transition via %s", target, fmtTemp(v), other)
	if err := s.Bus.Write(ctx, "700", "OpMode", other); err != nil {
		if ctx.Err() == nil {
			logf("enforcement aborted: %v", err)
		}
		return
	}
	if _, _, err := s.waitEffective(ctx, other); err != nil {
		if ctx.Err() == nil {
			logf("enforcement aborted: %v", err)
		}
		return
	}
	if ctx.Err() != nil {
		return
	}
	if err := s.Bus.Write(ctx, "700", "OpMode", target); err != nil {
		if ctx.Err() == nil {
			logf("enforcement aborted: %v", err)
		}
		return
	}
	ok, v, err = s.waitEffective(ctx, target)
	switch {
	case err != nil:
		if ctx.Err() == nil {
			logf("enforcement aborted: %v", err)
		}
	case ok:
		logf("applied after forced transition")
	default:
		logf("FAILED, z1ActualRoomTempDesired still %s", fmtTemp(v))
	}
}

func fmtTemp(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func (s *Server) waitEffective(ctx context.Context, mode string) (bool, float64, error) {
	poll, settle := s.pollEvery, s.settle
	if poll == 0 {
		poll = 5 * time.Second
	}
	if settle == 0 {
		settle = 60 * time.Second
	}
	deadline := time.Now().Add(settle)
	for {
		select {
		case <-ctx.Done():
			return false, 0, ctx.Err()
		case <-time.After(poll):
		}
		v, err := s.desiredTemp(ctx, -1)
		if err != nil {
			return false, 0, err
		}
		if (mode == "off") == (v <= frostTemp) {
			return true, v, nil
		}
		if !time.Now().Before(deadline) {
			return false, v, nil
		}
	}
}

type DumpItem struct {
	Label string `json:"label"`
	Value any    `json:"value"`
	Unit  string `json:"unit,omitempty"`
	Text  string `json:"text,omitempty"`
	Error string `json:"error,omitempty"`
}

// handleDump exports every catalog value plus the raw value of every readable ebusd message.
func (s *Server) handleDump(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()
	items := map[string]DumpItem{}
	for id, it := range s.Catalog.Items {
		if ctx.Err() != nil {
			return
		}
		v := s.readItem(ctx, it, false)
		items[id] = DumpItem{Label: it.Label, Value: v.Value, Unit: it.Unit, Text: v.Text, Error: v.Error}
	}
	raw := map[string]string{}
	defs, err := s.Bus.ReadDefs(ctx)
	if err != nil {
		errJSON(w, http.StatusBadGateway, err.Error())
		return
	}
	for _, d := range defs {
		if ctx.Err() != nil {
			return
		}
		if d[1] == "errorhistory" {
			continue
		}
		v, err := s.Bus.Read(ctx, d[0], d[1], 600, "")
		if err != nil {
			v = "ERR: " + readErr(err)
		}
		raw[d[0]+"."+d[1]] = v
	}
	var devices []ebusd.Device
	if m, err := s.getMeta(ctx); err == nil {
		devices = m.Devices
	}
	w.Header().Set("Content-Disposition", `attachment; filename="vaillant-config-`+now.Format("2006-01-02-1504")+`.json"`)
	writeJSON(w, 200, map[string]any{"time": now.Format(time.RFC3339), "version": s.Version, "devices": devices, "items": items, "raw": raw})
}
