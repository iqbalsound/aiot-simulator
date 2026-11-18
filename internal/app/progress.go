// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.

package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type item struct{ Key, Label string }

type labDef struct {
	Title string
	Req   []item
	Task  []item
}

// labDefs mirrors the deliverables of the AI-4301 Laboratory Manual (v1.0).
var labDefs = map[int]labDef{
	1: {"Orientation to IoT Kits and Open-Source Workstation Setup",
		[]item{{"inventory", "Kit inventory checklist completed"}, {"verify", "Workstation toolchain verified (python3, git, ping, ssh)"}},
		[]item{{"ssh_task", "Key-based SSH steps researched for viva"}}},
	2: {"Interfacing Sensors with Arduino/Raspberry Pi GPIO Pins",
		[]item{{"wiring_ok", "All three sensors wired to the correct pins"}, {"serial_dht", "DHT11-only sketch read in Serial Monitor"}, {"serial_combined", "Combined CSV sketch printing one line per cycle"}},
		[]item{{"task2", "Extra sensor wired and reported as a 5th field"}, {"circuit_run", "Circuit built visually and run in the Circuit Lab"}}},
	3: {"Sensor Data Collection and Preprocessing",
		[]item{{"rows200", "sensor_log.csv holds at least 200 logged rows"}, {"cleaned", "Missing / malformed rows removed or interpolated"}, {"smoothed", "Rolling-mean (window 5) temperature plotted"}},
		[]item{{"median", "Median filter vs rolling mean compared on the light column"}}},
	4: {"Real-Time Sensor Data Visualization Dashboards",
		[]item{{"dash_live", "Live dashboard (gauges, chart, motion) updating"}},
		[]item{{"alert_set", "Threshold ALERT state demonstrated"}}},
	5: {"Setting Up an MQTT Broker and Publish/Subscribe Telemetry",
		[]item{{"broker_ok", "Broker tested with publish/subscribe round trip"}, {"mqtt_pub", "Sensor telemetry published to aiot/lab5/*"}},
		[]item{{"auth_ok", "Authenticated client connected"}, {"auth_refused", "Anonymous connection refused after enabling authentication"}}},
	6: {"Machine Learning Model Training in Python on Sensor Datasets",
		[]item{{"dt_trained", "Decision tree trained and evaluated"}},
		[]item{{"second_model", "Second algorithm compared against the decision tree"}}},
	7: {"Converting Trained Models to TensorFlow Lite Format",
		[]item{{"nn_trained", "Keras-style network trained"}, {"tflite", "Converted to TFLite"}, {"quant", "Post-training quantisation applied"}, {"verified", "Interpreter output compared with original model"}},
		[]item{{"sizes", "Size table (Keras / TFLite / quantised) recorded"}}},
	8: {"Mid-Term Practical Assessment: Consolidation of Labs 1 to 7",
		[]item{{"midterm", "Assessment task drawn and submitted"}}, nil},
	9: {"Deploying TensorFlow Lite Micro Models to ESP32 / Nano 33 BLE Sense",
		[]item{{"header", "model_data.h C array generated"}, {"deployed", "On-device inference ran and matched workstation"}},
		[]item{{"arena_min", "Smallest working tensor arena found"}}},
	10: {"MQTT-Based Communication with an Open-Source IoT Dashboard Platform",
		[]item{{"device_created", "Device provisioned (access token)"}, {"tb_telemetry", "Telemetry received on v1/devices/me/telemetry"}},
		[]item{{"alarm_rule", "Alarm rule configured"}, {"alarm_fired", "Alarm demonstrated"}}},
	11: {"Implementing AI-Based Anomaly Detection on Streaming Sensor Data",
		[]item{{"iforest", "Isolation Forest trained"}, {"live_detect", "Live anomaly alert published to aiot/lab11/anomaly"}},
		[]item{{"sweep", "Contamination values compared"}}},
	12: {"Predictive Maintenance Case-Study Implementation",
		[]item{{"pdm", "Rolling-window features engineered and classifier evaluated (recall)"}},
		[]item{{"pdm_extra", "Extra feature tested against baseline recall"}}},
	13: {"Mini-Project Phase 1: Design and Sensor Integration",
		[]item{{"project_selected", "Project theme selected"}, {"arch_done", "Four-layer architecture drawn"}, {"proposal", "One-page proposal submitted"}},
		[]item{{"risk", "Individual technical-risk paragraph written"}}},
	14: {"Mini-Project Phase 2: Integration and Testing",
		[]item{{"project_tests", "At least five test scenarios run (normal, boundary, fault)"}},
		[]item{{"testlog", "Test log table exported"}}},
	15: {"Mini-Project Performance Benchmarking",
		[]item{{"bench", "At least 20 trials benchmarked with results table"}},
		[]item{{"bottleneck", "Latency bottleneck identified with a design change"}}},
	16: {"Mini-Project Presentations and Viva",
		[]item{{"slides", "Slides and fallback demo prepared"}, {"report_export", "Evidence report exported"}},
		[]item{{"reflection", "Individual reflection written"}}},
}

// manualKeys are the checkboxes a student may tick by hand in the UI.
var manualKeys = map[string]bool{"inventory": true, "verify": true, "ssh_task": true, "dash_live": true,
	"arch_done": true, "proposal": true, "risk": true, "bottleneck": true, "slides": true, "reflection": true,
	"midterm": true, "report_export": true, "alert_set": true, "circuit_run": true}

func (a *App) progressPath() string { return filepath.Join(a.Dir, "progress.json") }

func (a *App) loadProgress() {
	b, err := os.ReadFile(a.progressPath())
	if err != nil {
		return
	}
	var f struct {
		Prog map[string]bool   `json:"progress"`
		Ev   map[string]string `json:"evidence"`
	}
	if json.Unmarshal(b, &f) == nil {
		if f.Prog != nil {
			a.prog = f.Prog
		}
		if f.Ev != nil {
			a.ev = f.Ev
		}
	}
}

func (a *App) saveProgressLocked() {
	b, _ := json.MarshalIndent(map[string]any{"progress": a.prog, "evidence": a.ev}, "", "  ")
	os.WriteFile(a.progressPath(), b, 0o644)
}

// mark records a completed step with human-readable evidence.
func (a *App) mark(key, evidence string) {
	a.mu.Lock()
	changed := !a.prog[key]
	a.prog[key] = true
	if evidence != "" {
		a.ev[key] = evidence
	}
	if changed {
		a.saveProgressLocked()
	}
	a.mu.Unlock()
	if changed {
		a.hub.Send("progress", a.progressView())
	}
}

func (a *App) markQuiet(key string) { a.mark(key, "") }

func (a *App) hMark(r *http.Request) (any, error) {
	var in struct {
		Key      string `json:"key"`
		Value    *bool  `json:"value"`
		Evidence string `json:"evidence"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if !manualKeys[in.Key] {
		return nil, bad("%q cannot be ticked manually", in.Key)
	}
	v := in.Value == nil || *in.Value
	if v {
		ev := "ticked by student"
		if e := strings.TrimSpace(in.Evidence); e != "" && len(e) <= 200 {
			ev = e
		}
		a.mark(in.Key, ev)
	} else {
		a.mu.Lock()
		delete(a.prog, in.Key)
		a.saveProgressLocked()
		a.mu.Unlock()
		a.hub.Send("progress", a.progressView())
	}
	return a.progressView(), nil
}

func (a *App) progressView() []map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()
	view := func(items []item) []map[string]any {
		o := []map[string]any{}
		for _, it := range items {
			o = append(o, map[string]any{"key": it.Key, "label": it.Label, "done": a.prog[it.Key],
				"manual": manualKeys[it.Key], "evidence": a.ev[it.Key]})
		}
		return o
	}
	all := func(items []item) bool {
		for _, it := range items {
			if !a.prog[it.Key] {
				return false
			}
		}
		return len(items) > 0
	}
	out := []map[string]any{}
	for n := 1; n <= 16; n++ {
		d := labDefs[n]
		out = append(out, map[string]any{"lab": n, "title": d.Title, "req": view(d.Req), "task": view(d.Task),
			"done": all(d.Req), "taskDone": all(d.Task)})
	}
	return out
}

func (a *App) setHist(lab string, md string) {
	a.mu.Lock()
	a.hist[lab] = md
	a.mu.Unlock()
}

// report builds a Markdown evidence file the student can paste into the lab report.
func (a *App) report() string {
	a.markQuiet("report_export")
	a.mu.Lock()
	defer a.mu.Unlock()
	var sb strings.Builder
	fmt.Fprintf(&sb, "# AI-4301 AIoT Simulator — Lab Evidence\n\n")
	fmt.Fprintf(&sb, "- Generated: %s\n- Simulator version: %s\n- Workspace: `%s`\n\n", time.Now().Format("2006-01-02 15:04"), a.Version, a.Dir)
	sb.WriteString("> Evidence produced by the **simulator**. The manual requires every student to also complete a supervised session with real hardware.\n\n")
	sb.WriteString("## Completion summary\n\n| Lab | Title | Required | Student task |\n|---|---|---|---|\n")
	chk := func(items []item) string {
		if len(items) == 0 {
			return "—"
		}
		n := 0
		for _, it := range items {
			if a.prog[it.Key] {
				n++
			}
		}
		return fmt.Sprintf("%d/%d", n, len(items))
	}
	for n := 1; n <= 16; n++ {
		d := labDefs[n]
		fmt.Fprintf(&sb, "| %d | %s | %s | %s |\n", n, d.Title, chk(d.Req), chk(d.Task))
	}
	sb.WriteString("\n## Recorded evidence\n\n")
	keys := make([]string, 0, len(a.ev))
	for k := range a.ev {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if a.ev[k] != "ticked by student" {
			fmt.Fprintf(&sb, "- **%s**: %s\n", k, a.ev[k])
		}
	}
	hk := make([]string, 0, len(a.hist))
	for k := range a.hist {
		hk = append(hk, k)
	}
	sort.Strings(hk)
	for _, k := range hk {
		if s, ok := a.hist[k].(string); ok {
			sb.WriteString("\n" + s + "\n")
		}
	}
	return sb.String()
}
