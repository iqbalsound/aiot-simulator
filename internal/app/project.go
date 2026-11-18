// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.

package app

import (
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aiotsim/internal/ml"
	"aiotsim/internal/mqtt"
)

type projectModel struct {
	Key      string
	Name     string
	Features []string
	Ranges   [][2]float64
	Labels   [2]string
	Tree     *ml.Tree
	IF       *ml.IForest
	Xte      [][]float64
	Yte      []int
	Log      []testRow
}

type testRow struct {
	N        int    `json:"n"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Input    []any  `json:"input"`
	Expected string `json:"expected"`
	Observed string `json:"observed"`
	Pass     bool   `json:"pass"`
	raw      []float64
}

func buildProject(key string) (*projectModel, error) {
	rng := rand.New(rand.NewSource(7))
	switch key {
	case "irrigation":
		p := &projectModel{Key: key, Name: "Smart agriculture: automated irrigation",
			Features: []string{"soil_moisture_%", "temperature_C", "humidity_%"},
			Ranges:   [][2]float64{{0, 100}, {-10, 60}, {0, 100}}, Labels: [2]string{"HOLD", "IRRIGATE"}}
		var X [][]float64
		var y []int
		for i := 0; i < 1500; i++ {
			s, t, h := rng.Float64()*100, 10+rng.Float64()*35, 20+rng.Float64()*70
			l := 0
			if s < 35 || (s < 45 && t > 30) {
				l = 1
			}
			X, y = append(X, []float64{s, t, h}), append(y, l)
		}
		Xtr, Xte, ytr, yte := ml.Split(X, y, 0.25, 42)
		idx := make([]int, len(Xtr))
		for i := range idx {
			idx[i] = i
		}
		p.Tree = ml.FitTree(Xtr, ytr, idx, ml.TreeOpts{MaxDepth: 5})
		p.Xte, p.Yte = Xte, yte
		return p, nil
	case "health":
		p := &projectModel{Key: key, Name: "Health monitoring: vital-sign anomaly alert (education demo, not a medical device)",
			Features: []string{"heart_rate_bpm", "skin_temp_C", "motion_per_min"},
			Ranges:   [][2]float64{{30, 220}, {30, 43}, {0, 200}}, Labels: [2]string{"OK", "ALERT"}}
		var X [][]float64
		for i := 0; i < 800; i++ {
			m := rng.Float64() * 120
			X = append(X, []float64{62 + 0.3*m + rng.NormFloat64()*3, 36.6 + rng.NormFloat64()*0.25, m})
		}
		p.IF = ml.FitIForest(X, 0.02, 42)
		// held-out set: normal rows and injected faults
		for i := 0; i < 150; i++ {
			m := rng.Float64() * 120
			p.Xte, p.Yte = append(p.Xte, []float64{62 + 0.3*m + rng.NormFloat64()*3, 36.6 + rng.NormFloat64()*0.25, m}), append(p.Yte, 0)
		}
		for i := 0; i < 50; i++ {
			p.Xte, p.Yte = append(p.Xte, []float64{125 + rng.Float64()*50, 38.5 + rng.Float64()*2, rng.Float64() * 10}), append(p.Yte, 1)
		}
		return p, nil
	}
	return nil, bad("template must be irrigation or health")
}

// infer is the project pipeline's decision stage. validate=true adds an input-validation layer.
func (p *projectModel) infer(x []float64, validate bool) string {
	if validate {
		for i, v := range x {
			if math.IsNaN(v) || v < p.Ranges[i][0] || v > p.Ranges[i][1] {
				return "SENSOR_FAULT"
			}
		}
	}
	for _, v := range x {
		if math.IsNaN(v) {
			return "CRASH"
		}
	}
	if p.Tree != nil {
		return p.Labels[p.Tree.Predict(x)]
	}
	if p.IF.Predict(x) == -1 {
		return p.Labels[1]
	}
	return p.Labels[0]
}

func (p *projectModel) scenarios() []testRow {
	nan := math.NaN()
	if p.Key == "irrigation" {
		return []testRow{
			{Name: "Wet soil, mild weather", Kind: "normal", raw: []float64{60, 24, 50}, Expected: "HOLD"},
			{Name: "Dry soil, mild weather", Kind: "normal", raw: []float64{20, 24, 50}, Expected: "IRRIGATE"},
			{Name: "Soil just above 35 % threshold", Kind: "boundary", raw: []float64{37, 25, 50}, Expected: "HOLD"},
			{Name: "Soil just below 35 % threshold", Kind: "boundary", raw: []float64{33, 25, 50}, Expected: "IRRIGATE"},
			{Name: "Damp soil but hot day (>30 C)", Kind: "boundary", raw: []float64{42, 33, 40}, Expected: "IRRIGATE"},
			{Name: "Soil sensor disconnected (NaN)", Kind: "fault", raw: []float64{nan, 24, 50}, Expected: "SENSOR_FAULT"},
			{Name: "Soil sensor shorted (250 %)", Kind: "fault", raw: []float64{250, 24, 50}, Expected: "SENSOR_FAULT"},
		}
	}
	return []testRow{
		{Name: "Resting adult", Kind: "normal", raw: []float64{68, 36.6, 10}, Expected: "OK"},
		{Name: "Brisk walk", Kind: "normal", raw: []float64{95, 36.9, 105}, Expected: "OK"},
		{Name: "Upper-normal resting values", Kind: "boundary", raw: []float64{72, 37.1, 20}, Expected: "OK"},
		{Name: "Fever with racing pulse at rest", Kind: "anomaly", raw: []float64{150, 39.8, 3}, Expected: "ALERT"},
		{Name: "Very low pulse and temperature", Kind: "anomaly", raw: []float64{45, 34.0, 0}, Expected: "ALERT"},
		{Name: "Pulse sensor disconnected (NaN)", Kind: "fault", raw: []float64{nan, 36.6, 10}, Expected: "SENSOR_FAULT"},
		{Name: "Sensor stuck at zero", Kind: "fault", raw: []float64{0, 0, 0}, Expected: "SENSOR_FAULT"},
	}
}

func (a *App) project(key string) (*projectModel, error) {
	a.mu.Lock()
	p := a.proj
	a.mu.Unlock()
	if p != nil && p.Key == key {
		return p, nil
	}
	p, err := buildProject(key)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.proj = p
	a.mu.Unlock()
	return p, nil
}

func (a *App) routesProject(get, post func(string, func(*http.Request) (any, error)), mux *http.ServeMux) {
	post("/api/project/run", func(r *http.Request) (any, error) {
		var in struct {
			Template string `json:"template"`
			Validate bool   `json:"validate"`
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		p, err := a.project(in.Template)
		if err != nil {
			return nil, err
		}
		a.mark("project_selected", p.Name)
		var rows []testRow
		pass := 0
		for i, sc := range p.scenarios() {
			sc.N = i + 1
			sc.Observed = p.infer(sc.raw, in.Validate)
			sc.Pass = sc.Observed == sc.Expected
			if sc.Pass {
				pass++
			}
			for _, v := range sc.raw {
				sc.Input = append(sc.Input, jf(v))
			}
			rows = append(rows, sc)
		}
		a.mu.Lock()
		p.Log = rows
		a.mu.Unlock()
		a.mark("project_tests", fmt.Sprintf("%d/%d scenarios passed (validation layer %v)", pass, len(rows), in.Validate))
		var sb strings.Builder
		fmt.Fprintf(&sb, "## Lab 14 — Functional test log (%s)\n\n| # | Scenario | Type | Expected | Observed | Result |\n|---|---|---|---|---|---|\n", p.Name)
		for _, r := range rows {
			fmt.Fprintf(&sb, "| %d | %s | %s | %s | %s | %s |\n", r.N, r.Name, r.Kind, r.Expected, r.Observed, map[bool]string{true: "PASS", false: "FAIL"}[r.Pass])
		}
		a.setHist("lab14", sb.String())
		return map[string]any{"name": p.Name, "features": p.Features, "rows": rows, "passed": pass, "total": len(rows), "validate": in.Validate,
			"architecture": architecture(p.Key)}, nil
	})
	post("/api/project/export", func(r *http.Request) (any, error) {
		a.mu.Lock()
		p := a.proj
		a.mu.Unlock()
		if p == nil || len(p.Log) == 0 {
			return nil, bad("Run the test scenarios first (Step 5).")
		}
		var sb strings.Builder
		sb.WriteString("scenario,type,input,expected,observed,result\n")
		for _, r := range p.Log {
			in := []string{}
			for _, v := range r.Input {
				in = append(in, fmt.Sprint(v))
			}
			fmt.Fprintf(&sb, "%q,%s,%q,%s,%s,%s\n", r.Name, r.Kind, strings.Join(in, " | "), r.Expected, r.Observed, map[bool]string{true: "PASS", false: "FAIL"}[r.Pass])
		}
		path := filepath.Join(a.Dir, "test_log.csv")
		if err := writeFile(path, sb.String()); err != nil {
			return nil, err
		}
		a.mark("testlog", "test_log.csv exported")
		return map[string]any{"file": path}, nil
	})
	post("/api/bench/run", a.hBench)
}

func writeFile(p, s string) error { return os.WriteFile(p, []byte(s), 0o644) }

func architecture(key string) []map[string]any {
	if key == "irrigation" {
		return []map[string]any{
			{"layer": "Perception", "items": "Soil-moisture probe (ADC), DHT11 (temp/humidity), relay + pump (actuator)"},
			{"layer": "Edge", "items": "ESP32 firmware: sampling, input validation, decision tree (TFLite-Micro class)"},
			{"layer": "Network", "items": "Wi-Fi + MQTT (topic farm/field1/telemetry), QoS 1"},
			{"layer": "Cloud / Application", "items": "Mosquitto broker, dashboard (Node-RED / ThingsBoard), irrigation history and alarms"},
		}
	}
	return []map[string]any{
		{"layer": "Perception", "items": "Pulse sensor, skin-temperature sensor, accelerometer, buzzer (actuator)"},
		{"layer": "Edge", "items": "nRF52 / ESP32 firmware: windowing, validation, Isolation-Forest scoring"},
		{"layer": "Network", "items": "BLE to phone gateway, MQTT to broker"},
		{"layer": "Cloud / Application", "items": "Broker, clinician dashboard, alert notifications (privacy and consent required)"},
	}
}

// ---------------------------------------------------------------- Lab 15

func (a *App) hBench(r *http.Request) (any, error) {
	var in struct {
		Trials   int    `json:"trials"`
		Link     string `json:"link"`  // loopback | wifi | cellular | internet
		Where    string `json:"where"` // edge | gateway | cloud
		Template string `json:"template"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if in.Trials < 5 || in.Trials > 300 {
		in.Trials = 30
	}
	if in.Template == "" {
		in.Template = "irrigation"
	}
	p, err := a.project(in.Template)
	if err != nil {
		return nil, err
	}
	link := map[string][2]float64{"loopback": {0, 0}, "wifi": {8, 3}, "cellular": {60, 30}, "internet": {80, 25}}
	lk, ok := link[in.Link]
	if !ok {
		in.Link, lk = "wifi", link["wifi"]
	}
	if in.Where != "edge" && in.Where != "cloud" {
		in.Where = "gateway"
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	norm := func(m, s float64) float64 { return math.Max(0.05, m+rng.NormFloat64()*s) }

	// real MQTT round trip through the simulator's broker
	var rt time.Duration
	var sub *mqtt.Client
	got := make(chan time.Time, 1)
	pub, err := mqtt.Dial(a.brokerAddr(), "aiot-bench-pub", "", "", false)
	if err == nil {
		sub, err = mqtt.Dial(a.brokerAddr(), "aiot-bench-sub", "", "", false)
	}
	useMQTT := err == nil
	if !useMQTT {
		// anonymous access disabled: fall back to the internal path so benchmarking still works
		a.log("warn", "benchmark: broker requires credentials; using internal publish path")
	} else {
		defer pub.Close()
		defer sub.Close()
		sub.OnMessage = func(t string, b []byte) {
			select {
			case got <- time.Now():
			default:
			}
		}
		sub.Subscribe("aiot/bench")
	}
	_ = rt
	var tot, sens, tx, inf, act []float64
	x := p.Xte[0]
	for i := 0; i < in.Trials; i++ {
		x = p.Xte[i%len(p.Xte)]
		s := norm(24, 3) + 0.2 // DHT11 conversion + ADC (modelled hardware delay)
		var t float64
		if in.Where != "edge" {
			start := time.Now()
			if useMQTT {
				pub.Publish("aiot/bench", []byte("x"), 0, false)
				select {
				case e := <-got:
					t = float64(e.Sub(start).Microseconds()) / 1000
				case <-time.After(time.Second):
					t = 1000
				}
			} else {
				t = float64(time.Since(start).Microseconds()) / 1000
			}
			t += norm(lk[0], lk[1])
		}
		t0 := time.Now()
		for k := 0; k < 200; k++ {
			p.infer(x, true)
		}
		iMs := float64(time.Since(t0).Nanoseconds()) / 200 / 1e6
		switch in.Where {
		case "edge":
			iMs = (20000 + 60*float64(depthOf(p))) / 64000 // Cortex-M4 @ 64 MHz, in ms
		case "cloud":
			iMs += norm(5, 1)
		}
		ac := norm(3, 1)
		sens, tx, inf, act = append(sens, s), append(tx, t), append(inf, iMs), append(act, ac)
		tot = append(tot, s+t+iMs+ac)
	}
	names := []string{"sensing", "transmission", "inference", "actuation"}
	means := []float64{ml.Mean(sens), ml.Mean(tx), ml.Mean(inf), ml.Mean(act)}
	bi := 0
	for i, m := range means {
		if m > means[bi] {
			bi = i
		}
	}
	// histogram
	lo, hi := tot[0], tot[0]
	for _, v := range tot {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	bins := make([]int, 10)
	w := (hi - lo) / 10
	if w == 0 {
		w = 1
	}
	for _, v := range tot {
		k := int((v - lo) / w)
		if k > 9 {
			k = 9
		}
		bins[k]++
	}
	pred := make([]int, len(p.Xte))
	for i, xx := range p.Xte {
		if p.infer(xx, true) == p.Labels[1] {
			pred[i] = 1
		}
	}
	rep := ml.Evaluate(p.Yte, pred)
	modelBytes := 0
	if p.Tree != nil {
		modelBytes = p.Tree.Leaves() * 2 * 12
	} else {
		modelBytes = 100 * 40 * 8
	}
	advice := map[string]string{
		"sensing":      "Sampling delay dominates: the DHT11 needs ≈25 ms per conversion. Use a faster sensor (DHT22/SHT31), read it asynchronously, or sample at the rate the process actually changes.",
		"transmission": "Network delay dominates: move inference to the edge (Lab 9), batch less / use QoS 0, or keep the broker on the local network instead of the cloud.",
		"inference":    "Inference dominates: shrink the model (quantisation, pruning, shallower tree) or run it on faster hardware.",
		"actuation":    "Actuation dominates: use a solid-state relay or reduce relay settling time.",
	}[names[bi]]
	hl := make([]string, 10)
	for i := range hl {
		hl[i] = fmt.Sprintf("%.0f", lo+float64(i)*w)
	}
	hv := make([]float64, 10)
	for i, b := range bins {
		hv[i] = float64(b)
	}
	a.setCharts("lab15", chartSpec{Kind: "bar", Title: "Mean latency per stage (ms)", Labels: names, Values: means},
		chartSpec{Kind: "bar", Title: "Response-time histogram (ms, bin start)", Labels: hl, Values: hv})
	a.mark("bench", fmt.Sprintf("%d trials, %s inference over %s: mean %.1f ms, std %.1f ms, accuracy %.3f", in.Trials, in.Where, in.Link, ml.Mean(tot), ml.Std(tot), rep.Accuracy))
	a.setHist("lab15", fmt.Sprintf("## Lab 15 — Benchmark (%s, inference at %s, link %s)\n\n| Metric | Value |\n|---|---|\n| Trials | %d |\n| Mean response time | %.2f ms |\n| Std dev | %.2f ms |\n| 95th percentile | %.2f ms |\n| Held-out accuracy | %.3f (precision %.3f, recall %.3f) |\n| Inference time | %.3f ms |\n| Model size (est.) | %d bytes |\n| Largest latency contributor | %s |\n",
		p.Name, in.Where, in.Link, in.Trials, ml.Mean(tot), ml.Std(tot), ml.Percentile(tot, 95), rep.Accuracy, rep.Precision, rep.Recall, ml.Mean(inf), modelBytes, names[bi]))
	return map[string]any{"trials": in.Trials, "where": in.Where, "link": in.Link, "project": p.Name,
		"total": map[string]any{"mean": round(ml.Mean(tot), 2), "std": round(ml.Std(tot), 2), "min": round(lo, 2), "max": round(hi, 2), "p95": round(ml.Percentile(tot, 95), 2)},
		"stages": []map[string]any{{"name": names[0], "mean": round(means[0], 2), "std": round(ml.Std(sens), 2), "measured": false},
			{"name": names[1], "mean": round(means[1], 2), "std": round(ml.Std(tx), 2), "measured": "partly (real MQTT hop + modelled link)"},
			{"name": names[2], "mean": round(means[2], 4), "std": round(ml.Std(inf), 4), "measured": in.Where == "gateway"},
			{"name": names[3], "mean": round(means[3], 2), "std": round(ml.Std(act), 2), "measured": false}},
		"bottleneck": names[bi], "advice": advice, "histogram": bins, "histLo": round(lo, 2), "histW": round(w, 3),
		"accuracy": rep, "modelBytes": modelBytes, "series": jfs(tot),
		"note": "Sensing and actuation delays are modelled hardware latencies; the MQTT hop and gateway inference are measured on this computer."}, nil
}

func depthOf(p *projectModel) int {
	if p.Tree != nil {
		return p.Tree.Depth()
	}
	return 8
}
