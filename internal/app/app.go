// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.

// Package app wires the simulator engine, MQTT broker and lab logic to an HTTP API
// consumed by the embedded web UI.
package app

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"aiotsim/internal/ml"
	"aiotsim/internal/mqtt"
	"aiotsim/internal/sim"
)

// ---------------------------------------------------------------- event hub (SSE)

type Hub struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

func newHub() *Hub { return &Hub{subs: map[chan []byte]struct{}{}} }

func (h *Hub) Send(typ string, data any) {
	b, err := json.Marshal(map[string]any{"type": typ, "data": data})
	if err != nil {
		return
	}
	h.mu.Lock()
	for c := range h.subs {
		select {
		case c <- b:
		default: // slow consumer: drop
		}
	}
	h.mu.Unlock()
}

func (h *Hub) sub() chan []byte {
	c := make(chan []byte, 512)
	h.mu.Lock()
	h.subs[c] = struct{}{}
	h.mu.Unlock()
	return c
}
func (h *Hub) unsub(c chan []byte) { h.mu.Lock(); delete(h.subs, c); h.mu.Unlock() }

// ---------------------------------------------------------------- JSON helpers

// jf converts NaN/Inf to null so encoding/json can serialise it.
func jf(v float64) any {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return v
}

func jfs(v []float64) []any {
	o := make([]any, len(v))
	for i, x := range v {
		o[i] = jf(x)
	}
	return o
}

func round(v float64, d int) float64 {
	p := math.Pow(10, float64(d))
	return math.Round(v*p) / p
}

type apiErr struct {
	code int
	msg  string
}

func (e apiErr) Error() string { return e.msg }
func bad(format string, a ...any) error {
	return apiErr{400, fmt.Sprintf(format, a...)}
}

// ---------------------------------------------------------------- App

type serialEntry struct {
	TS   float64 `json:"ts"`
	Line string  `json:"line"`
	Raw  string  `json:"raw,omitempty"`
}

type topicVal struct {
	Topic   string  `json:"topic"`
	Payload string  `json:"payload"`
	Count   int     `json:"count"`
	TS      float64 `json:"ts"`
	From    string  `json:"from"`
}

type App struct {
	Version string
	Dir     string
	Addr    string // HTTP listen address, set by the caller

	mu  sync.Mutex
	hub *Hub
	S   *sim.Sensors
	B   *sim.Board

	monitorBaud int
	serial      []serialEntry
	autoVary    bool
	vary        varier

	// Lab 3 dataset
	logging    bool
	logFields  int
	rows       []Row
	dataSource string
	clean      *Cleaned

	// MQTT
	Broker    *mqtt.Broker
	mqttPort  int
	mqttAll   bool
	mqttNote  string
	allowAnon bool
	users     map[string]string
	topics    map[string]*topicVal
	bridge    bridgeCfg
	bridgeCl  *mqtt.Client

	// Lab 10 ThingsBoard-style platform
	devices []*Device
	rules   []*AlarmRule
	alarms  []Alarm
	tbPub   tbCfg
	tbCl    *mqtt.Client

	// Lab 11
	iforest  *ml.IForest
	anomaly  anomalyCfg
	lastVals map[string]float64
	anomN    int

	// ML state (labs 6, 7, 9)
	cmf     *comfortSet
	nn      *nnResult
	mcu     *mcuResult
	proj    *projectModel
	hist    map[string]any // last result per feature for the report
	charts  map[string][]chartSpec
	student Student

	// progress
	prog map[string]bool
	ev   map[string]string

	lastPing time.Time
	pinged   bool
	started  time.Time
	quit     chan struct{}
	quitOnce sync.Once
}

// New creates the application. dir is the student workspace folder.
func New(version, dir string, seed int64) *App {
	a := &App{Version: version, Dir: dir, hub: newHub(), monitorBaud: 9600, logFields: 4,
		autoVary: true, users: map[string]string{}, allowAnon: true, topics: map[string]*topicVal{},
		lastVals: map[string]float64{}, prog: map[string]bool{}, ev: map[string]string{},
		hist: map[string]any{}, charts: map[string][]chartSpec{}, started: time.Now(), quit: make(chan struct{})}
	a.S = sim.NewSensors(seed)
	a.B = sim.NewBoard(a.S)
	a.B.OnTick = a.onTick
	a.bridge = bridgeCfg{Interval: 2}
	a.tbPub = tbCfg{Interval: 2}
	a.anomaly = anomalyCfg{}
	os.MkdirAll(dir, 0o755)
	a.loadProgress()
	a.loadStudent()
	return a
}

// Quit returns a channel closed when the UI asks the program to exit.
func (a *App) Quit() <-chan struct{} { return a.quit }

func (a *App) log(level, msg string) {
	a.hub.Send("log", map[string]any{"ts": time.Now().UnixMilli(), "level": level, "msg": msg})
}

// StartBroker (re)starts the MQTT broker on port, falling back to a free port.
func (a *App) StartBroker(port int, all bool) {
	if a.Broker == nil {
		b := mqtt.NewBroker()
		b.Auth = a.auth
		b.OnPublish = a.onPublish
		b.OnEvent = func(kind, detail string) {
			lvl := "info"
			if kind == "refused" {
				lvl = "warn"
				a.mu.Lock()
				a.ev["mqtt_refused"] = detail
				a.mu.Unlock()
				a.mark("auth_refused", detail)
			}
			a.log(lvl, "MQTT: "+detail)
			a.hub.Send("mqttstate", a.mqttState())
		}
		a.Broker = b
	}
	a.Broker.Close()
	host := "127.0.0.1"
	if all {
		host = "0.0.0.0"
	}
	note := ""
	err := a.Broker.Listen(fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		for _, p := range []int{18830, 18831, 0} {
			if a.Broker.Listen(fmt.Sprintf("%s:%d", host, p)) == nil {
				note = fmt.Sprintf("Port %d is in use (is a real Mosquitto running?). The simulator broker is listening on %s instead.", port, a.Broker.Addr())
				break
			}
		}
	}
	a.mu.Lock()
	a.mqttAll, a.mqttNote = all, note
	if _, ps, e := net.SplitHostPort(a.Broker.Addr()); e == nil {
		fmt.Sscanf(ps, "%d", &a.mqttPort)
	}
	a.mu.Unlock()
	// re-attach the in-process anomaly subscriber
	a.attachDetector()
}

func (a *App) auth(id, user, pass string, has bool) byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	if has {
		for _, d := range a.devices {
			if d.Token == user {
				return mqtt.ConnAccepted
			}
		}
		if pw, ok := a.users[user]; ok {
			if pw == pass {
				return mqtt.ConnAccepted
			}
			return mqtt.ConnBadCredential
		}
		if a.allowAnon && len(a.users) == 0 {
			return mqtt.ConnAccepted
		}
		return mqtt.ConnBadCredential
	}
	if a.allowAnon {
		return mqtt.ConnAccepted
	}
	return mqtt.ConnNotAuthorized
}

func (a *App) mqttState() map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()
	us := []string{}
	for u := range a.users {
		us = append(us, u)
	}
	host := "localhost"
	conf := fmt.Sprintf("listener %d%s\nallow_anonymous %v\n", a.mqttPort, map[bool]string{true: "", false: " 127.0.0.1"}[a.mqttAll], a.allowAnon)
	if len(a.users) > 0 {
		conf += "password_file /etc/mosquitto/passwd\n"
	}
	n := 0
	if a.Broker != nil {
		n = a.Broker.ClientCount()
	}
	return map[string]any{"port": a.mqttPort, "host": host, "lan": a.mqttAll, "allowAnonymous": a.allowAnon,
		"users": us, "note": a.mqttNote, "conf": conf, "clients": n,
		"bridge": a.bridge.view(), "tbPub": a.tbPub}
}

// ---------------------------------------------------------------- routes

func (a *App) Handler(static fs.FS) http.Handler {
	mux := http.NewServeMux()
	get := func(p string, f func(r *http.Request) (any, error)) { mux.HandleFunc("GET "+p, a.wrap(f)) }
	post := func(p string, f func(r *http.Request) (any, error)) { mux.HandleFunc("POST "+p, a.wrap(f)) }

	get("/api/state", func(r *http.Request) (any, error) { return a.state(), nil })
	get("/api/ping", func(r *http.Request) (any, error) {
		a.mu.Lock()
		a.lastPing, a.pinged = time.Now(), true
		a.mu.Unlock()
		return map[string]any{"ok": true}, nil
	})
	post("/api/quit", func(r *http.Request) (any, error) {
		a.quitOnce.Do(func() { close(a.quit) })
		return map[string]any{"ok": true}, nil
	})
	post("/api/board", a.hBoard)
	post("/api/wiring", a.hWiring)
	post("/api/env", a.hEnv)
	post("/api/event", a.hEvent)
	post("/api/serial/clear", func(r *http.Request) (any, error) {
		a.mu.Lock()
		a.serial = nil
		a.mu.Unlock()
		return map[string]any{"ok": true}, nil
	})
	post("/api/serial/baud", a.hMonitorBaud)
	post("/api/progress/mark", a.hMark)
	mux.HandleFunc("GET /api/report.md", func(w http.ResponseWriter, r *http.Request) {
		if !a.hostOK(w, r) {
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="AI-4301_lab_evidence.md"`)
		w.Write([]byte(a.report()))
	})
	mux.HandleFunc("GET /api/stream", a.hStream)
	a.routesMeta(get, post)
	a.routesPDF(mux)
	a.routesCircuit(get, post)
	a.routesData(get, post, mux)
	a.routesNet(get, post)
	a.routesML(get, post, mux)
	a.routesProject(get, post, mux)
	mux.Handle("GET /", http.FileServer(http.FS(static)))
	return a.guard(mux)
}

// hostOK defends the local server against DNS-rebinding: only loopback/localhost hosts.
func (a *App) hostOK(w http.ResponseWriter, r *http.Request) bool {
	h, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		h = r.Host
	}
	if h != "127.0.0.1" && h != "localhost" && h != "[::1]" && h != "::1" {
		http.Error(w, "forbidden host", http.StatusForbidden)
		return false
	}
	return true
}

func (a *App) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.hostOK(w, r) {
			return
		}
		if r.Method == http.MethodPost {
			// a custom header forces a CORS pre-flight, which we never grant
			if r.Header.Get("X-AIoT") != "1" {
				http.Error(w, "missing X-AIoT header", http.StatusForbidden)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

type handler func(r *http.Request) (any, error)

func (a *App) wrap(f handler) http.HandlerFunc {
	if f == nil {
		return func(http.ResponseWriter, *http.Request) {}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		v, err := f(r)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			code := 500
			if ae, ok := err.(apiErr); ok {
				code = ae.code
			}
			w.WriteHeader(code)
			json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		if tw, ok := v.(textResult); ok {
			w.Header().Set("Content-Type", tw.mime)
			if tw.name != "" {
				w.Header().Set("Content-Disposition", `attachment; filename="`+tw.name+`"`)
			}
			w.Write([]byte(tw.body))
			return
		}
		json.NewEncoder(w).Encode(v)
	}
}

type textResult struct{ mime, name, body string }

func decode(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil && err.Error() != "EOF" {
		return bad("invalid JSON: %v", err)
	}
	return nil
}

func (a *App) hStream(w http.ResponseWriter, r *http.Request) {
	if !a.hostOK(w, r) {
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	c := a.hub.sub()
	defer a.hub.unsub(c)
	fmt.Fprint(w, ": connected\n\n")
	fl.Flush()
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case b := <-c:
			fmt.Fprintf(w, "data: %s\n\n", b)
			fl.Flush()
		case <-tick.C:
			fmt.Fprint(w, ": keepalive\n\n")
			fl.Flush()
		}
	}
}

// ---------------------------------------------------------------- state & board handlers

func (a *App) state() map[string]any {
	a.mu.Lock()
	rows, logging, src := len(a.rows), a.logging, a.dataSource
	tail := a.serial
	if len(tail) > 60 {
		tail = tail[len(tail)-60:]
	}
	tail = append([]serialEntry(nil), tail...)
	st := map[string]any{
		"version": a.Version, "workspace": a.Dir, "monitorBaud": a.monitorBaud, "autoVary": a.autoVary,
		"logging": logging, "rows": rows, "dataSource": src, "logFields": a.logFields,
		"serialTail": tail, "dht22": a.S.DHT22, "student": a.student, "author": Author, "authorTitle": AuthorTitle, "authorDept": AuthorDept,
	}
	a.mu.Unlock()
	st["board"] = a.B.State()
	st["env"] = a.S.GetEnv()
	w := a.S.GetWiring()
	st["wiring"] = w
	st["wiringCheck"] = sim.WiringCheck(w)
	st["sketches"] = sim.Sketches
	st["mqtt"] = a.mqttState()
	st["progress"] = a.progressView()
	st["tb"] = a.tbView()
	st["anomaly"] = a.anomalyView()
	return st
}

func (a *App) hBoard(r *http.Request) (any, error) {
	var in struct {
		Sketch string  `json:"sketch"`
		Baud   int     `json:"baud"`
		Speed  float64 `json:"speed"`
		Run    *bool   `json:"running"`
		DHT22  *bool   `json:"dht22"`
		Auto   *bool   `json:"autoVary"`
		Fields int     `json:"logFields"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	valid := in.Sketch == ""
	for _, s := range sim.Sketches {
		if s.ID == in.Sketch {
			valid = true
		}
	}
	if !valid {
		return nil, bad("unknown sketch %q", in.Sketch)
	}
	if in.Baud != 0 && (in.Baud < 300 || in.Baud > 2000000) {
		return nil, bad("unsupported baud rate")
	}
	if in.Speed != 0 && (in.Speed < 0.5 || in.Speed > 100) {
		return nil, bad("speed must be between 0.5 and 100")
	}
	a.B.Configure(in.Sketch, in.Baud, in.Speed)
	if in.DHT22 != nil {
		a.S.SetDHT22(*in.DHT22)
	}
	a.mu.Lock()
	if in.Auto != nil {
		a.autoVary = *in.Auto
	}
	if in.Fields == 4 || in.Fields == 5 {
		a.logFields = in.Fields
	}
	a.mu.Unlock()
	if in.Run != nil {
		if *in.Run {
			a.B.Start()
		} else {
			a.B.Stop()
		}
	}
	if in.Sketch != "" {
		a.log("info", "Sketch uploaded: "+in.Sketch)
	}
	return a.state(), nil
}

func (a *App) hMonitorBaud(r *http.Request) (any, error) {
	var in struct {
		Baud int `json:"baud"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if in.Baud < 300 {
		return nil, bad("invalid baud")
	}
	a.mu.Lock()
	a.monitorBaud = in.Baud
	a.mu.Unlock()
	return map[string]any{"monitorBaud": in.Baud}, nil
}

var validPins = map[string]bool{"": true, "D2": true, "D3": true, "D4": true, "D5": true, "D6": true, "D7": true, "D8": true,
	"A0": true, "A1": true, "A2": true}

func (a *App) hWiring(r *http.Request) (any, error) {
	var w sim.Wiring
	if err := decode(r, &w); err != nil {
		return nil, err
	}
	for _, p := range []string{w.DHT, w.PIR, w.LDR, w.Extra} {
		if !validPins[p] {
			return nil, bad("invalid pin %q", p)
		}
	}
	a.S.SetWiring(w)
	chk := sim.WiringCheck(w)
	ok := w.DHT == "D2" && w.PIR == "D3" && w.LDR == "A0"
	if ok {
		a.mark("wiring_ok", "DHT11 DATA→D2, PIR OUT→D3, LDR divider→A0")
	}
	if w.Extra != "" && w.Extra != "D2" && w.Extra != "D3" && w.Extra != "A0" {
		a.mark("task2", "extra sensor on "+w.Extra)
	}
	return map[string]any{"check": chk, "ok": ok}, nil
}

func (a *App) hEnv(r *http.Request) (any, error) {
	e := a.S.GetEnv()
	if err := decode(r, &e); err != nil {
		return nil, err
	}
	e.Temp = math.Max(-10, math.Min(60, e.Temp))
	e.Hum = math.Max(0, math.Min(100, e.Hum))
	e.Light = math.Max(0, math.Min(100, e.Light))
	e.MotionProb = math.Max(0, math.Min(1, e.MotionProb))
	e.Noise = math.Max(0, math.Min(3, e.Noise))
	e.Glitch = math.Max(0, math.Min(0.3, e.Glitch))
	e.Distance = math.Max(2, math.Min(400, e.Distance))
	a.S.SetEnv(e)
	return e, nil
}

func (a *App) hEvent(r *http.Request) (any, error) {
	var in struct {
		Name string `json:"name"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if !a.S.Event(in.Name) {
		return nil, bad("unknown event %q", in.Name)
	}
	a.log("info", "Physical event: "+in.Name)
	return map[string]any{"ok": true}, nil
}

// ---------------------------------------------------------------- board tick

// garble mimics a wrong Serial Monitor baud rate.
func garble(s string, seed int) string {
	b := []byte(s)
	for i := range b {
		b[i] = byte(0x80 + (int(b[i])*7+seed*13+i*31)%0x7f)
	}
	return strings.ToValidUTF8(string(b), "?")
}

func (a *App) onTick(smp sim.Sample, line string) {
	a.mu.Lock()
	// automatic condition variation (hand warms the sensor, lights change, ...)
	if a.autoVary {
		e := a.S.GetEnv()
		a.vary.step(smp.TS, &e)
		a.S.SetEnv(e)
	}
	sk := a.B.State().Sketch
	var shown string
	if line != "" {
		shown = line
		if a.monitorBaud != a.B.State().Baud {
			shown = garble(line, int(smp.TS))
		}
		a.serial = append(a.serial, serialEntry{TS: smp.TS, Line: shown})
		if len(a.serial) > 400 {
			a.serial = a.serial[len(a.serial)-400:]
		}
	}
	logging, fields := a.logging, a.logFields
	wiringOK := a.S.GetWiring().DHT == "D2"
	a.mu.Unlock()

	if line != "" {
		a.hub.Send("serial", serialEntry{TS: smp.TS, Line: shown})
		if (sk == "combined" || sk == "combined_extra") && wiringOK && shown == line {
			a.markQuiet("serial_combined")
		}
		if sk == "dht" {
			a.markQuiet("serial_dht")
		}
	}
	if line != "" && logging {
		a.logLine(smp.TS, line, fields)
	}
	a.bridgeTick(smp.TS, line)
	a.tbTick(smp.TS, line)
}

// Idle reports whether a UI has ever pinged and how long ago the last ping was.
func (a *App) Idle() (pinged bool, since time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pinged {
		return true, time.Since(a.lastPing)
	}
	return false, time.Since(a.started)
}
