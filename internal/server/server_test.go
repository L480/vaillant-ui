package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/L480/vaillant-ui/internal/catalog"
	"github.com/L480/vaillant-ui/internal/ebusd"
	"github.com/L480/vaillant-ui/internal/ebusd/fake"
)

func setup(t *testing.T, readonly bool) (*httptest.Server, *fake.Server) {
	vals := map[string]string{
		"700.z1DayTemp": "23", "700.OpMode": "auto", "700.SFMode": "auto",
		"700.HolidayStartPeriod": "26.04.2023", "700.Time": "22:16:11",
		"700.z1Shortname": "ZONE 1", "700.AdaptHeatCurve": "no",
		"700.z1RoomTemp": "empty", "bai.FlowTemp": "32.19;ok", "bai.WP": "on",
		"bai.Statenumber": "8", "700.currenterror": "-;-;28;-;-", "bai.currenterror": "-;-;-;-;-",
		"700.errorhistory#0": "0;12:10;03.01.2026;22", "700.Hc1HeatCurve": "1.1",
		"700.z1Name1": "", "bai.StorageTemp": "-14.94;cutoff", "bai.HeatingSwitch": "on",
	}
	for _, d := range Days {
		vals["700.z1Timer."+d] = "05:00;22:00;-:-;-:-;-:-;-:-"
	}
	f := fake.New(vals)
	addr, err := f.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.Close)
	s := &Server{Bus: ebusd.New(addr), Catalog: catalog.Build(), ReadOnly: readonly, AllowedHosts: []string{"ebus.example.org"}}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts, f
}

func do(t *testing.T, method, url, body string, hdr ...string) (int, string) {
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		if hdr[i] == "Host" {
			req.Host = hdr[i+1]
			continue
		}
		req.Header.Set(hdr[i], hdr[i+1])
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func TestValues(t *testing.T) {
	ts, _ := setup(t, false)
	code, body := do(t, "GET", ts.URL+"/api/values?ids=700.z1DayTemp,bai.FlowTemp,700.z1RoomTemp,700.HolidayStartPeriod,700.Time,700.z1Timer,bai.Statenumber,700.AdaptHeatCurve,700.nope", "")
	if code != 200 {
		t.Fatal(code, body)
	}
	var r struct{ Values map[string]Value }
	json.Unmarshal([]byte(body), &r)
	want := map[string]any{"700.z1DayTemp": 23.0, "bai.FlowTemp": 32.19, "700.z1RoomTemp": nil, "700.HolidayStartPeriod": "2023-04-26", "700.Time": "22:16", "700.AdaptHeatCurve": false}
	for k, v := range want {
		if r.Values[k].Value != v {
			t.Errorf("%s = %#v, want %#v", k, r.Values[k].Value, v)
		}
	}
	if !strings.HasPrefix(r.Values["bai.Statenumber"].Text, "S.08") {
		t.Error(r.Values["bai.Statenumber"])
	}
	if r.Values["700.nope"].Error == "" {
		t.Error("unknown id should error")
	}
	if !strings.Contains(body, `"Monday":[["05:00","22:00"]]`) {
		t.Error(body)
	}
}

func TestWrites(t *testing.T) {
	ts, f := setup(t, false)
	cases := []struct {
		id, body string
		code     int
		write    string
	}{
		{"700.z1DayTemp", `{"value":21.5}`, 200, "700.z1DayTemp=21.5"},
		{"700.z1DayTemp", `{"value":21.3}`, 400, ""},
		{"700.z1DayTemp", `{"value":40}`, 400, ""},
		{"700.z1DayTemp", `{"value":"x"}`, 400, ""},
		{"700.OpMode", `{"value":"night"}`, 200, "700.OpMode=night"},
		{"700.OpMode", `{"value":"bogus"}`, 400, ""},
		{"700.Hc1HeatCurve", `{"value":1.25}`, 200, "700.Hc1HeatCurve=1.25"},
		{"700.AdaptHeatCurve", `{"value":true}`, 200, "700.AdaptHeatCurve=yes"},
		{"700.HolidayStartPeriod", `{"value":"2026-12-24"}`, 200, "700.HolidayStartPeriod=24.12.2026"},
		{"700.HolidayStartPeriod", `{"value":"24.12.2026"}`, 400, ""},
		{"700.Time", `{"value":"07:05"}`, 200, "700.Time=07:05:00"},
		{"700.z1Shortname", `{"value":"WOHNEN"}`, 200, "700.z1Shortname=WOHNEN"},
		{"700.z1Shortname", `{"value":"WOHNZIMMER"}`, 400, ""},
		{"700.z1Shortname", `{"value":"A;B"}`, 400, ""},
		{"700.z1Timer", `{"value":{"Monday":[["16:00","22:00"],["06:00","08:30"]]}}`, 200, "700.z1Timer.Monday=06:00;08:30;16:00;22:00;-:-;-:-"},
		{"700.z1Timer", `{"value":{"Monday":[["06:05","08:00"]]}}`, 400, ""},
		{"700.z1Timer", `{"value":{"Monday":[["06:00","08:00"],["07:00","09:00"]]}}`, 400, ""},
		{"700.z1Timer", `{"value":{"Funday":[]}}`, 400, ""},
		{"700.z1Timer", `{"value":{"Sunday":[]}}`, 200, "700.z1Timer.Sunday=-:-;-:-;-:-;-:-;-:-;-:-"},
		{"700.z1RoomTemp", `{"value":20}`, 403, ""},
		{"700.nope", `{"value":1}`, 404, ""},
	}
	for _, c := range cases {
		n := len(f.Writes)
		code, body := do(t, "PUT", ts.URL+"/api/values/"+c.id, c.body)
		if code != c.code {
			t.Errorf("%s %s: code %d (%s), want %d", c.id, c.body, code, body, c.code)
		}
		var got string
		if len(f.Writes) > n {
			got = f.Writes[len(f.Writes)-1]
		}
		if got != c.write {
			t.Errorf("%s %s: wrote %q, want %q", c.id, c.body, got, c.write)
		}
	}
	if code, _ := do(t, "PUT", ts.URL+"/api/values/700.OpMode", `{"value":"off"}`, "Origin", "https://evil.example"); code != 403 {
		t.Error("cross-origin write allowed")
	}
	if code, _ := do(t, "PUT", ts.URL+"/api/values/700.OpMode", ""); code != 415 {
		t.Error("non-JSON write allowed")
	}
}

func TestLegacy(t *testing.T) {
	ts, f := setup(t, false)
	if c, b := do(t, "GET", ts.URL+"/status", ""); c != 200 || b != "1" {
		t.Error(c, b)
	}
	if c, b := do(t, "GET", ts.URL+"/pump-status", ""); c != 200 || b != "1" {
		t.Error(c, b)
	}
	if c, b := do(t, "POST", ts.URL+"/off", ""); c != 200 || b != "Heizung ist nun ausgeschaltet." || f.Values["700.OpMode"] != "off" {
		t.Error(c, b)
	}
	if c, b := do(t, "GET", ts.URL+"/status", ""); c != 200 || b != "0" {
		t.Error(c, b)
	}
	if c, _ := do(t, "POST", ts.URL+"/party", ""); c != 200 || f.Values["700.SFMode"] != "party" {
		t.Error(c)
	}
}

func TestReadOnly(t *testing.T) {
	ts, f := setup(t, true)
	if c, _ := do(t, "PUT", ts.URL+"/api/values/700.z1DayTemp", `{"value":21}`); c != 403 {
		t.Error(c)
	}
	if c, _ := do(t, "POST", ts.URL+"/off", ""); c != 403 {
		t.Error(c)
	}
	if len(f.Writes) != 0 {
		t.Error(f.Writes)
	}
	_, body := do(t, "GET", ts.URL+"/api/catalog", "")
	if strings.Contains(body, `"writable":true`) {
		t.Error("readonly catalog has writable items")
	}
}

func TestErrors(t *testing.T) {
	ts, _ := setup(t, false)
	_, body := do(t, "GET", ts.URL+"/api/errors", "")
	if !strings.Contains(body, `"code":28`) || !strings.Contains(body, `"date":"2026-01-03"`) {
		t.Error(body)
	}
}

func TestCatalogConsistency(t *testing.T) {
	c := catalog.Build()
	for _, sec := range c.Sections {
		for _, g := range sec.Groups {
			for _, id := range g.Items {
				it := c.Items[id]
				if it == nil {
					t.Fatalf("group %s: missing %s", g.ID, id)
				}
				if it.AllowWrite && it.Kind == "number" && (it.Min == nil || it.Max == nil || it.Step == nil) {
					t.Errorf("%s writable number without range", id)
				}
			}
		}
	}
}

func TestDump(t *testing.T) {
	ts, _ := setup(t, false)
	req, _ := http.NewRequest("GET", ts.URL+"/api/dump", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var d struct {
		Items map[string]DumpItem
		Raw   map[string]string
	}
	json.NewDecoder(res.Body).Decode(&d)
	if !strings.Contains(res.Header.Get("Content-Disposition"), "vaillant-config-") {
		t.Error(res.Header)
	}
	if d.Items["700.z1DayTemp"].Value != 23.0 || d.Raw["bai.FlowTemp"] != "32.19;ok" {
		t.Errorf("%+v %+v", d.Items["700.z1DayTemp"], d.Raw["bai.FlowTemp"])
	}
}

func TestReviewRegressions(t *testing.T) {
	ts, f := setup(t, false)
	_, body := do(t, "GET", ts.URL+"/api/values?ids=700.z1Name1,700.z1DayTemp,700.OpMode,bai.StorageTemp", "")
	var r struct{ Values map[string]Value }
	json.Unmarshal([]byte(body), &r)
	if r.Values["700.z1Name1"].Value != "" || r.Values["700.z1DayTemp"].Value != 23.0 || r.Values["700.OpMode"].Value != "auto" {
		t.Errorf("responses shifted after empty value: %s", body)
	}
	if r.Values["bai.StorageTemp"].Error == "" {
		t.Errorf("sensor fault not reported: %+v", r.Values["bai.StorageTemp"])
	}
	n := len(f.Writes)
	for _, c := range []struct{ id, body string }{
		{"bai.HeatingSwitch", `{"value":null}`},
		{"700.z1DayTemp", `{"value":null}`},
		{"700.z1Timer", `{"value":{"Monday":null}}`},
	} {
		if code, b := do(t, "PUT", ts.URL+"/api/values/"+c.id, c.body); code != 400 {
			t.Errorf("%s %s: %d %s", c.id, c.body, code, b)
		}
	}
	if len(f.Writes) != n {
		t.Errorf("null wrote: %v", f.Writes[n:])
	}
	if code, _ := do(t, "PUT", ts.URL+"/api/values/700.OpMode", `{"value":"off"}`, "Host", "evil.example", "Origin", "http://evil.example"); code != 403 {
		t.Errorf("rebinding host accepted: %d", code)
	}
	if code, _ := do(t, "GET", ts.URL+"/api/dump", "", "Host", "evil.example"); code != 403 {
		t.Errorf("rebinding host accepted on GET: %d", code)
	}
	if code, _ := do(t, "GET", ts.URL+"/status", "", "Host", "ebus.example.org"); code != 200 {
		t.Errorf("allowed host rejected: %d", code)
	}
	if code, _ := do(t, "POST", ts.URL+"/off", "", "Host", "evil.example"); code != 403 {
		t.Errorf("rebinding host accepted on legacy: %d", code)
	}
}
