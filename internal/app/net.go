package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"aiotsim/internal/mqtt"
)

// ---------------------------------------------------------------- Lab 5 bridge

type bridgeCfg struct {
	Enabled  bool
	Interval float64
	UseAuth  bool
	User     string
	Pass     string
	QoS      int
	last     float64
	busy     bool
	Err      string
	Sent     int
}

func (b bridgeCfg) view() map[string]any {
	return map[string]any{"enabled": b.Enabled, "interval": b.Interval, "useAuth": b.UseAuth, "user": b.User,
		"qos": b.QoS, "error": b.Err, "sent": b.Sent}
}

func (a *App) brokerAddr() string { return fmt.Sprintf("127.0.0.1:%d", a.mqttPortSafe()) }
func (a *App) mqttPortSafe() int  { a.mu.Lock(); defer a.mu.Unlock(); return a.mqttPort }

func (a *App) bridgeTick(ts float64, line string) {
	a.mu.Lock()
	cfg := &a.bridge
	if !cfg.Enabled || line == "" || cfg.busy || ts-cfg.last < cfg.Interval {
		a.mu.Unlock()
		return
	}
	parts := strings.Split(line, ",")
	if len(parts) != 4 { // the publisher script only accepts four fields
		a.mu.Unlock()
		return
	}
	cfg.last, cfg.busy = ts, true
	c := *cfg
	cl := a.bridgeCl
	a.mu.Unlock()
	go func() {
		defer func() { a.mu.Lock(); a.bridge.busy = false; a.mu.Unlock() }()
		var err error
		if cl == nil {
			cl, err = mqtt.Dial(a.brokerAddr(), "aiot-lab5-publisher", c.User, c.Pass, c.UseAuth)
			if err != nil {
				a.mu.Lock()
				a.bridge.Err = "Publisher could not connect: " + err.Error()
				a.mu.Unlock()
				a.log("warn", "publisher.py: "+err.Error())
				a.hub.Send("mqttstate", a.mqttState())
				return
			}
			a.mu.Lock()
			a.bridgeCl = cl
			a.bridge.Err = ""
			nu := len(a.users)
			a.mu.Unlock()
			if c.UseAuth && nu > 0 {
				a.mark("auth_ok", "publisher authenticated as "+c.User)
			}
		}
		names := []string{"temperature", "humidity", "motion", "light"}
		for i, n := range names {
			if err = cl.Publish("aiot/lab5/"+n, []byte(parts[i]), byte(c.QoS), false); err != nil {
				break
			}
		}
		a.mu.Lock()
		if err != nil {
			a.bridge.Err = "publish failed: " + err.Error()
			cl.Close()
			a.bridgeCl = nil
		} else {
			a.bridge.Sent++
			a.bridge.Err = ""
		}
		sent := a.bridge.Sent
		a.mu.Unlock()
		if err == nil && sent >= 1 {
			a.mark("mqtt_pub", fmt.Sprintf("%d telemetry batches published to aiot/lab5/*", sent))
		}
	}()
}

func (a *App) resetClients() {
	a.mu.Lock()
	if a.bridgeCl != nil {
		a.bridgeCl.Close()
		a.bridgeCl = nil
	}
	if a.tbCl != nil {
		a.tbCl.Close()
		a.tbCl = nil
	}
	a.mu.Unlock()
}

// ---------------------------------------------------------------- topic tracking

func (a *App) onPublish(m mqtt.Message) {
	pl := string(m.Payload)
	if len(pl) > 2000 {
		pl = pl[:2000] + "…"
	}
	now := float64(time.Now().UnixMilli()) / 1000
	a.mu.Lock()
	tv := a.topics[m.Topic]
	if tv == nil {
		tv = &topicVal{Topic: m.Topic}
		a.topics[m.Topic] = tv
	}
	tv.Payload, tv.Count, tv.TS, tv.From = pl, tv.Count+1, now, m.From
	if strings.HasPrefix(m.Topic, "aiot/lab5/") {
		if v, err := strconv.ParseFloat(strings.TrimSpace(pl), 64); err == nil {
			a.lastVals[strings.TrimPrefix(m.Topic, "aiot/lab5/")] = v
		}
	}
	a.mu.Unlock()
	a.hub.Send("mqtt", map[string]any{"topic": m.Topic, "payload": pl, "from": m.From, "ts": now, "qos": m.QoS})
	if m.Topic == "v1/devices/me/telemetry" {
		a.tbIngest(m.User, m.Payload)
	}
}

// ---------------------------------------------------------------- Lab 10 platform

type Device struct {
	Name    string
	Token   string
	Created time.Time
	Count   int
	Points  []tbPoint
}

type tbPoint struct {
	TS int64
	V  map[string]float64
}

type AlarmRule struct {
	ID        int
	Device    string
	Key, Op   string
	Threshold float64
	Active    bool
}

type Alarm struct {
	ID     int     `json:"id"`
	TS     int64   `json:"ts"`
	Device string  `json:"device"`
	Rule   string  `json:"rule"`
	Value  float64 `json:"value"`
	Status string  `json:"status"`
}

type tbCfg struct {
	Enabled  bool    `json:"enabled"`
	Token    string  `json:"token"`
	Interval float64 `json:"interval"`
	Err      string  `json:"error"`
	Sent     int     `json:"sent"`
	last     float64
	busy     bool
}

func newToken() string {
	b := make([]byte, 10)
	rand.Read(b)
	return strings.ToUpper(hex.EncodeToString(b))
}

func (a *App) tbView() map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()
	ds := []map[string]any{}
	for _, d := range a.devices {
		ds = append(ds, map[string]any{"name": d.Name, "token": d.Token, "count": d.Count, "created": d.Created.Format("15:04:05")})
	}
	rs := []map[string]any{}
	for _, r := range a.rules {
		rs = append(rs, map[string]any{"id": r.ID, "device": r.Device, "key": r.Key, "op": r.Op, "threshold": r.Threshold, "active": r.Active})
	}
	al := a.alarms
	if len(al) > 30 {
		al = al[len(al)-30:]
	}
	return map[string]any{"devices": ds, "rules": rs, "alarms": al, "publisher": a.tbPub}
}

func (a *App) tbIngest(token string, payload []byte) {
	var raw map[string]any
	if json.Unmarshal(payload, &raw) != nil {
		a.log("warn", "ThingsBoard: telemetry payload is not a JSON object")
		return
	}
	vals := map[string]float64{}
	for k, v := range raw {
		if f, ok := v.(float64); ok && !math.IsNaN(f) {
			vals[k] = f
		}
	}
	a.mu.Lock()
	var dev *Device
	for _, d := range a.devices {
		if d.Token == token && token != "" {
			dev = d
		}
	}
	if dev == nil {
		a.mu.Unlock()
		a.log("warn", "ThingsBoard: telemetry ignored (username is not a device access token)")
		return
	}
	now := time.Now().UnixMilli()
	dev.Count++
	dev.Points = append(dev.Points, tbPoint{now, vals})
	if len(dev.Points) > 600 {
		dev.Points = dev.Points[len(dev.Points)-600:]
	}
	name := dev.Name
	var fired []Alarm
	for _, r := range a.rules {
		if r.Device != name {
			continue
		}
		v, ok := vals[r.Key]
		if !ok {
			continue
		}
		hit := (r.Op == ">" && v > r.Threshold) || (r.Op == "<" && v < r.Threshold)
		desc := fmt.Sprintf("%s %s %g", r.Key, r.Op, r.Threshold)
		if hit && !r.Active {
			r.Active = true
			al := Alarm{ID: len(a.alarms) + 1, TS: now, Device: name, Rule: desc, Value: v, Status: "ACTIVE_UNACK"}
			a.alarms = append(a.alarms, al)
			fired = append(fired, al)
		} else if !hit && r.Active {
			r.Active = false
			a.alarms = append(a.alarms, Alarm{ID: len(a.alarms) + 1, TS: now, Device: name, Rule: desc, Value: v, Status: "CLEARED_UNACK"})
		}
	}
	count := dev.Count
	a.mu.Unlock()
	a.hub.Send("tb", map[string]any{"device": name, "ts": now, "values": vals, "count": count})
	a.mark("tb_telemetry", fmt.Sprintf("%d telemetry messages received by device %q", count, name))
	for _, al := range fired {
		a.hub.Send("tbalarm", al)
		a.mark("alarm_fired", fmt.Sprintf("alarm %q raised at value %.2f", al.Rule, al.Value))
		a.log("warn", fmt.Sprintf("ThingsBoard ALARM on %s: %s (value %.2f)", al.Device, al.Rule, al.Value))
	}
}

func (a *App) tbTick(ts float64, line string) {
	a.mu.Lock()
	cfg := &a.tbPub
	if !cfg.Enabled || line == "" || cfg.busy || ts-cfg.last < cfg.Interval {
		a.mu.Unlock()
		return
	}
	parts := strings.Split(line, ",")
	if len(parts) != 4 {
		a.mu.Unlock()
		return
	}
	cfg.last, cfg.busy = ts, true
	token := cfg.Token
	cl := a.tbCl
	a.mu.Unlock()
	go func() {
		defer func() { a.mu.Lock(); a.tbPub.busy = false; a.mu.Unlock() }()
		setErr := func(s string) { a.mu.Lock(); a.tbPub.Err = s; a.mu.Unlock() }
		t, e1 := strconv.ParseFloat(parts[0], 64)
		h, e2 := strconv.ParseFloat(parts[1], 64)
		m, e3 := strconv.Atoi(parts[2])
		l, e4 := strconv.Atoi(parts[3])
		if e := errors.Join(e1, e2, e3, e4); e != nil {
			setErr("ValueError: " + line + " (line skipped)")
			return
		}
		if math.IsNaN(t) || math.IsNaN(h) {
			setErr("DHT read failed (nan) – line skipped")
			return
		}
		var err error
		if cl == nil {
			cl, err = mqtt.Dial(a.brokerAddr(), "aiot-lab10-publisher", token, "", true)
			if err != nil {
				setErr("MQTT connect failed: " + err.Error())
				return
			}
			a.mu.Lock()
			a.tbCl = cl
			a.mu.Unlock()
		}
		body, _ := json.Marshal(map[string]any{"temperature": t, "humidity": h, "motion": m, "light": l})
		if err = cl.Publish("v1/devices/me/telemetry", body, 0, false); err != nil {
			setErr("publish failed: " + err.Error())
			cl.Close()
			a.mu.Lock()
			a.tbCl = nil
			a.mu.Unlock()
			return
		}
		a.mu.Lock()
		a.tbPub.Sent++
		a.tbPub.Err = ""
		a.mu.Unlock()
	}()
}

// ---------------------------------------------------------------- routes

func (a *App) routesNet(get, post func(string, func(*http.Request) (any, error))) {
	get("/api/mqtt/topics", func(r *http.Request) (any, error) {
		a.mu.Lock()
		defer a.mu.Unlock()
		ts := make([]*topicVal, 0, len(a.topics))
		for _, t := range a.topics {
			ts = append(ts, t)
		}
		sort.Slice(ts, func(i, j int) bool { return ts[i].Topic < ts[j].Topic })
		return ts, nil
	})
	post("/api/mqtt/config", func(r *http.Request) (any, error) {
		var in struct {
			AllowAnonymous *bool                  `json:"allowAnonymous"`
			LAN            *bool                  `json:"lan"`
			AddUser        *struct{ U, P string } `json:"addUser"`
			RemoveUser     string                 `json:"removeUser"`
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		a.mu.Lock()
		if in.AllowAnonymous != nil {
			a.allowAnon = *in.AllowAnonymous
		}
		if in.AddUser != nil {
			u := strings.TrimSpace(in.AddUser.U)
			if u == "" || in.AddUser.P == "" || len(u) > 64 || len(in.AddUser.P) > 128 {
				a.mu.Unlock()
				return nil, bad("a user name and a password are required (mosquitto_passwd -c /etc/mosquitto/passwd USER)")
			}
			a.users[u] = in.AddUser.P
		}
		if in.RemoveUser != "" {
			delete(a.users, in.RemoveUser)
		}
		lan, port := a.mqttAll, a.mqttPort
		a.mu.Unlock()
		if in.LAN != nil && *in.LAN != lan {
			a.StartBroker(port, *in.LAN)
		}
		a.resetClients() // existing sessions re-authenticate with the new rules
		a.hub.Send("mqttstate", a.mqttState())
		return a.mqttState(), nil
	})
	post("/api/mqtt/bridge", func(r *http.Request) (any, error) {
		var in struct {
			Enabled  bool    `json:"enabled"`
			Interval float64 `json:"interval"`
			UseAuth  bool    `json:"useAuth"`
			User     string  `json:"user"`
			Pass     string  `json:"pass"`
			QoS      int     `json:"qos"`
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		if in.Interval < 1 || in.Interval > 60 {
			in.Interval = 2
		}
		if in.QoS < 0 || in.QoS > 1 {
			in.QoS = 0
		}
		a.resetClients()
		a.mu.Lock()
		a.bridge = bridgeCfg{Enabled: in.Enabled, Interval: in.Interval, UseAuth: in.UseAuth, User: in.User, Pass: in.Pass, QoS: in.QoS}
		a.mu.Unlock()
		a.log("info", fmt.Sprintf("publisher.py enabled=%v every %gs (auth=%v)", in.Enabled, in.Interval, in.UseAuth))
		return a.mqttState(), nil
	})
	post("/api/mqtt/test", a.hMQTTTest)
	post("/api/tb/device", func(r *http.Request) (any, error) {
		var in struct {
			Name string `json:"name"`
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		in.Name = strings.TrimSpace(in.Name)
		if in.Name == "" || len(in.Name) > 60 {
			return nil, bad("device name is required")
		}
		a.mu.Lock()
		for _, d := range a.devices {
			if d.Name == in.Name {
				a.mu.Unlock()
				return nil, bad("a device named %q already exists", in.Name)
			}
		}
		d := &Device{Name: in.Name, Token: newToken(), Created: time.Now()}
		a.devices = append(a.devices, d)
		a.mu.Unlock()
		a.mark("device_created", fmt.Sprintf("device %q provisioned", d.Name))
		return a.tbView(), nil
	})
	post("/api/tb/rule", func(r *http.Request) (any, error) {
		var in struct {
			Device, Key, Op string
			Threshold       float64
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		if in.Op != ">" && in.Op != "<" {
			return nil, bad("operator must be > or <")
		}
		if in.Key == "" {
			in.Key = "temperature"
		}
		a.mu.Lock()
		found := false
		for _, d := range a.devices {
			if d.Name == in.Device {
				found = true
			}
		}
		if !found {
			a.mu.Unlock()
			return nil, bad("create a device first")
		}
		a.rules = append(a.rules, &AlarmRule{ID: len(a.rules) + 1, Device: in.Device, Key: in.Key, Op: in.Op, Threshold: in.Threshold})
		a.mu.Unlock()
		a.mark("alarm_rule", fmt.Sprintf("rule: %s %s %g on %s", in.Key, in.Op, in.Threshold, in.Device))
		return a.tbView(), nil
	})
	post("/api/tb/publisher", func(r *http.Request) (any, error) {
		var in struct {
			Enabled  bool    `json:"enabled"`
			Token    string  `json:"token"`
			Interval float64 `json:"interval"`
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		if in.Interval < 1 || in.Interval > 60 {
			in.Interval = 2
		}
		a.resetClients()
		a.mu.Lock()
		a.tbPub = tbCfg{Enabled: in.Enabled, Token: strings.TrimSpace(in.Token), Interval: in.Interval}
		a.mu.Unlock()
		return a.tbView(), nil
	})
	get("/api/tb/telemetry", func(r *http.Request) (any, error) {
		name := r.URL.Query().Get("device")
		n, _ := strconv.Atoi(r.URL.Query().Get("n"))
		if n <= 0 || n > 600 {
			n = 120
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		for _, d := range a.devices {
			if d.Name == name {
				pts := d.Points
				if len(pts) > n {
					pts = pts[len(pts)-n:]
				}
				out := map[string]any{"ts": []int64{}, "temperature": []any{}, "humidity": []any{}, "motion": []any{}, "light": []any{}}
				var tsl []int64
				series := map[string][]any{}
				for _, p := range pts {
					tsl = append(tsl, p.TS)
					for _, k := range []string{"temperature", "humidity", "motion", "light"} {
						series[k] = append(series[k], p.V[k])
					}
				}
				out["ts"] = tsl
				for k, v := range series {
					out[k] = v
				}
				return out, nil
			}
		}
		return nil, bad("unknown device")
	})
}

// hMQTTTest runs the equivalent of mosquitto_sub / mosquitto_pub from the UI and
// reports a terminal-style transcript. It talks to the real broker over TCP.
func (a *App) hMQTTTest(r *http.Request) (any, error) {
	var in struct {
		Action  string `json:"action"`
		User    string `json:"user"`
		Pass    string `json:"pass"`
		Auth    bool   `json:"useAuth"`
		Topic   string `json:"topic"`
		Payload string `json:"payload"`
		QoS     int    `json:"qos"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if in.Topic == "" {
		in.Topic = "aiot/lab5/test"
	}
	if in.Payload == "" {
		in.Payload = "hello aiot"
	}
	if in.QoS < 0 || in.QoS > 1 {
		in.QoS = 0
	}
	addr := a.brokerAddr()
	auth := ""
	if in.Auth {
		auth = fmt.Sprintf(" -u %s -P ****", in.User)
	}
	var tr []string
	fail := func(err error) (any, error) {
		tr = append(tr, "Error: "+err.Error())
		return map[string]any{"ok": false, "transcript": tr, "error": err.Error()}, nil
	}
	switch in.Action {
	case "connect":
		tr = append(tr, fmt.Sprintf("$ mosquitto_pub -h localhost -p %d -t %q -m %q%s", a.mqttPortSafe(), in.Topic, "ping", auth))
		c, err := mqtt.Dial(addr, "aiot-test-client", in.User, in.Pass, in.Auth)
		if err != nil {
			return fail(err)
		}
		c.Publish(in.Topic, []byte("ping"), 0, false)
		c.Close()
		tr = append(tr, "Connected and published (CONNACK code 0).")
		a.mu.Lock()
		nu := len(a.users)
		a.mu.Unlock()
		if in.Auth && nu > 0 {
			a.mark("auth_ok", "client authenticated as "+in.User)
		}
		return map[string]any{"ok": true, "transcript": tr}, nil
	case "roundtrip", "publish":
		sub, err := mqtt.Dial(addr, "aiot-test-sub", in.User, in.Pass, in.Auth)
		if err != nil {
			tr = append(tr, fmt.Sprintf("$ mosquitto_sub -h localhost -p %d -t %q%s", a.mqttPortSafe(), in.Topic, auth))
			return fail(err)
		}
		defer sub.Close()
		got := make(chan string, 4)
		sub.OnMessage = func(t string, p []byte) { got <- string(p) }
		if err := sub.Subscribe(in.Topic); err != nil {
			return fail(err)
		}
		tr = append(tr, fmt.Sprintf("# Terminal 1\n$ mosquitto_sub -h localhost -p %d -t %q%s", a.mqttPortSafe(), in.Topic, auth))
		pub, err := mqtt.Dial(addr, "aiot-test-pub", in.User, in.Pass, in.Auth)
		if err != nil {
			return fail(err)
		}
		defer pub.Close()
		tr = append(tr, fmt.Sprintf("# Terminal 2\n$ mosquitto_pub -h localhost -p %d -t %q -m %q -q %d%s", a.mqttPortSafe(), in.Topic, in.Payload, in.QoS, auth))
		if err := pub.Publish(in.Topic, []byte(in.Payload), byte(in.QoS), false); err != nil {
			return fail(err)
		}
		select {
		case m := <-got:
			tr = append(tr, "# Terminal 1 output\n"+m)
			if m == in.Payload && in.Topic == "aiot/lab5/test" {
				a.mark("broker_ok", "subscriber received \""+m+"\" on aiot/lab5/test")
			}
			return map[string]any{"ok": true, "transcript": tr, "received": m}, nil
		case <-time.After(2 * time.Second):
			return fail(errors.New("subscriber did not receive the message within 2 s"))
		}
	}
	return nil, bad("action must be connect, roundtrip or publish")
}
