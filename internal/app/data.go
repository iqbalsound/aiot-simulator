// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.

package app

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"aiotsim/internal/ml"
	"aiotsim/internal/pdf"
	"aiotsim/internal/sim"
)

// varier slowly moves the environment between random targets so that a logged
// dataset covers comfortable and uncomfortable conditions.
type varier struct {
	target sim.Env
	until  float64
	n      int64
}

func (v *varier) step(ts float64, e *sim.Env) {
	if ts >= v.until {
		v.n++
		r := time.Now().UnixNano() + v.n*7919
		f := func(k int64, lo, hi float64) float64 { return lo + float64((r/k)%1000)/1000*(hi-lo) }
		v.target = sim.Env{Temp: f(7, 17, 34), Hum: f(13, 32, 78), Light: f(31, 4, 96)}
		v.until = ts + 12
	}
	e.Temp += (v.target.Temp - e.Temp) * 0.2
	e.Hum += (v.target.Hum - e.Hum) * 0.2
	e.Light += (v.target.Light - e.Light) * 0.25
}

// Row is one line the Python logging script wrote to sensor_log.csv.
type Row struct {
	TS   float64
	Raw  []string
	Vals [5]float64 // temperature, humidity, motion, light, extra (NaN = not numeric)
	Bad  string     // "", "dht_nan", "non_numeric"
}

// Cleaned is the dataset after Lab 3 pre-processing.
type Cleaned struct {
	Mode    string
	Window  int
	Rows    []Row
	Temp    []float64
	Smooth  []float64
	Light   []float64
	LightMd []float64
	LightRM []float64
}

func parseRow(ts float64, parts []string) Row {
	r := Row{TS: ts, Raw: parts}
	for i := range r.Vals {
		r.Vals[i] = math.NaN()
	}
	for i, p := range parts {
		if i > 4 {
			break
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			r.Bad = "non_numeric"
			continue
		}
		r.Vals[i] = v
		if math.IsNaN(v) && r.Bad == "" {
			r.Bad = "dht_nan"
		}
	}
	return r
}

// logLine mimics the manual's Python logger: keep only lines with the expected field count.
func (a *App) logLine(ts float64, line string, fields int) {
	parts := strings.Split(line, ",")
	if len(parts) != fields {
		return // "if len(parts) == 4" in the student script
	}
	row := parseRow(ts, parts)
	a.mu.Lock()
	if len(a.rows) < 200000 {
		a.rows = append(a.rows, row)
		a.clean = nil
	}
	n := len(a.rows)
	a.dataSource = "logged from the virtual Arduino"
	a.mu.Unlock()
	if n%10 == 0 {
		a.hub.Send("logger", map[string]any{"rows": n})
	}
	if n >= 200 {
		a.mark("rows200", fmt.Sprintf("%d rows logged to sensor_log.csv", n))
	}
}

func (a *App) csvText(rows []Row, fields int) string {
	var sb strings.Builder
	sb.WriteString("timestamp,temperature,humidity,motion,light")
	if fields == 5 {
		sb.WriteString(",distance")
	}
	sb.WriteString("\n")
	for _, r := range rows {
		sb.WriteString(strconv.FormatFloat(r.TS, 'f', 6, 64))
		for _, p := range r.Raw {
			sb.WriteString("," + p)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func (a *App) saveRaw() {
	a.mu.Lock()
	rows, f := a.rows, a.logFields
	a.mu.Unlock()
	os.WriteFile(filepath.Join(a.Dir, "sensor_log.csv"), []byte(a.csvText(rows, f)), 0o644)
}

func (a *App) routesData(get, post func(string, func(*http.Request) (any, error)), mux *http.ServeMux) {
	post("/api/logger", func(r *http.Request) (any, error) {
		var in struct {
			Action string `json:"action"`
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		switch in.Action {
		case "start":
			a.mu.Lock()
			a.logging = true
			a.mu.Unlock()
			a.log("info", "log_sensors.py started: appending to sensor_log.csv")
		case "stop":
			a.mu.Lock()
			a.logging = false
			a.mu.Unlock()
			a.saveRaw()
			a.log("info", "log_sensors.py stopped; sensor_log.csv saved")
		case "clear":
			a.mu.Lock()
			a.logging, a.rows, a.clean = false, nil, nil
			a.mu.Unlock()
			a.saveRaw()
		default:
			return nil, bad("action must be start, stop or clear")
		}
		return map[string]any{"ok": true, "rows": a.rowCount()}, nil
	})
	post("/api/logger/sample", func(r *http.Request) (any, error) {
		var in struct {
			N int `json:"n"`
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		if in.N < 50 || in.N > 5000 {
			in.N = 600
		}
		s := sim.NewSensors(0)
		var v varier
		e := s.GetEnv()
		e.Glitch = 0.04
		s.SetEnv(e)
		var rows []Row
		clock := float64(time.Now().Unix()) - float64(in.N)
		for len(rows) < in.N {
			clock++
			e := s.GetEnv()
			v.step(clock, &e)
			s.SetEnv(e)
			smp := s.Read(clock)
			line := s.Format("combined", smp)
			if parts := strings.Split(line, ","); len(parts) == 4 {
				rows = append(rows, parseRow(clock, parts))
			}
		}
		a.mu.Lock()
		a.rows, a.clean, a.logFields, a.dataSource = rows, nil, 4, "instructor sample dataset (generated)"
		a.mu.Unlock()
		a.saveRaw()
		a.mark("rows200", fmt.Sprintf("%d rows (instructor sample dataset)", len(rows)))
		a.log("info", fmt.Sprintf("Sample dataset generated: %d rows", len(rows)))
		return map[string]any{"rows": len(rows)}, nil
	})
	get("/api/dataset", func(r *http.Request) (any, error) { return a.datasetView() })
	post("/api/preprocess", a.hPreprocess)
	get("/api/dataset.csv", func(r *http.Request) (any, error) {
		a.mu.Lock()
		rows, f := a.rows, a.logFields
		a.mu.Unlock()
		return textResult{"text/csv", "sensor_log.csv", a.csvText(rows, f)}, nil
	})
}

func (a *App) rowCount() int { a.mu.Lock(); defer a.mu.Unlock(); return len(a.rows) }

type colStat struct {
	Name    string `json:"name"`
	Dtype   string `json:"dtype"`
	NonNull int    `json:"nonNull"`
	Count   int    `json:"count"`
	Mean    any    `json:"mean"`
	Std     any    `json:"std"`
	Min     any    `json:"min"`
	Q25     any    `json:"q25"`
	Q50     any    `json:"q50"`
	Q75     any    `json:"q75"`
	Max     any    `json:"max"`
}

func describe(name string, rows []Row, idx int) colStat {
	var v []float64
	obj := false
	nonNull := 0
	for _, r := range rows {
		if idx < len(r.Raw) {
			if _, err := strconv.ParseFloat(strings.TrimSpace(r.Raw[idx]), 64); err != nil {
				obj = true
			}
		}
		if !math.IsNaN(r.Vals[idx]) {
			v = append(v, r.Vals[idx])
		}
		nonNull++
		if math.IsNaN(r.Vals[idx]) {
			nonNull--
		}
	}
	cs := colStat{Name: name, Dtype: "float64", NonNull: nonNull, Count: len(v)}
	if obj {
		cs.Dtype = "object"
	}
	if len(v) > 0 {
		sorted := append([]float64(nil), v...)
		sort.Float64s(sorted)
		cs.Mean, cs.Std = round(ml.Mean(v), 3), round(ml.Std(v), 3)
		cs.Min, cs.Max = sorted[0], sorted[len(sorted)-1]
		cs.Q25, cs.Q50, cs.Q75 = round(ml.Percentile(v, 25), 3), round(ml.Percentile(v, 50), 3), round(ml.Percentile(v, 75), 3)
	}
	return cs
}

func (a *App) datasetView() (any, error) {
	a.mu.Lock()
	rows, f, src := a.rows, a.logFields, a.dataSource
	a.mu.Unlock()
	names := []string{"temperature", "humidity", "motion", "light", "distance"}
	var cols []colStat
	for i := 0; i < f; i++ {
		cols = append(cols, describe(names[i], rows, i))
	}
	head := func(from, to int) [][]string {
		var o [][]string
		for i := from; i < to && i < len(rows); i++ {
			o = append(o, append([]string{strconv.FormatFloat(rows[i].TS, 'f', 3, 64)}, rows[i].Raw...))
		}
		return o
	}
	tailFrom := len(rows) - 5
	if tailFrom < 0 {
		tailFrom = 0
	}
	return map[string]any{"rows": len(rows), "source": src, "columns": cols, "head": head(0, 8), "tail": head(tailFrom, len(rows)),
		"path": filepath.Join(a.Dir, "sensor_log.csv"), "fields": f}, nil
}

func (a *App) hPreprocess(r *http.Request) (any, error) {
	var in struct {
		Mode   string `json:"mode"` // dropna | interpolate
		Window int    `json:"window"`
		Median bool   `json:"median"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if in.Mode != "dropna" && in.Mode != "interpolate" {
		in.Mode = "dropna"
	}
	if in.Window < 2 || in.Window > 60 {
		in.Window = 5
	}
	a.mu.Lock()
	rows := append([]Row(nil), a.rows...)
	a.mu.Unlock()
	if len(rows) < 10 {
		return nil, bad("Log at least 10 rows first (Lab 3, Step 4) or generate the instructor sample dataset.")
	}
	n := len(rows)
	// problem rows (reflection deliverable)
	type prob struct {
		Index int    `json:"index"`
		Raw   string `json:"raw"`
		Issue string `json:"issue"`
		Cause string `json:"cause"`
	}
	var probs []prob
	nanRows, coerced := 0, 0
	temps := make([]float64, n)
	for i, rw := range rows {
		temps[i] = rw.Vals[0]
	}
	med := ml.Median3(ml.Interpolate(temps))
	spikes := 0
	for i, rw := range rows {
		issue, cause := "", ""
		switch {
		case rw.Bad == "non_numeric":
			coerced++
			issue, cause = "non-numeric field", "a byte was corrupted on the serial line (electrical noise / baud mismatch)"
		case rw.Bad == "dht_nan":
			nanRows++
			issue, cause = "NaN reading", "the DHT11 read failed (checksum/timing error), Arduino prints nan"
		case !math.IsNaN(rw.Vals[0]) && math.Abs(rw.Vals[0]-med[i]) > 8:
			spikes++
			issue, cause = "temperature spike", "a momentary glitch: value jumps far from its neighbours (outlier, not removed by dropna)"
		}
		if issue != "" && len(probs) < 25 {
			probs = append(probs, prob{i, strings.Join(rw.Raw, ","), issue, cause})
		}
	}
	// clean
	var kept []Row
	switch in.Mode {
	case "dropna":
		for _, rw := range rows {
			ok := true
			for k := 0; k < 4 && k < len(rw.Raw); k++ {
				if math.IsNaN(rw.Vals[k]) {
					ok = false
				}
			}
			if ok {
				kept = append(kept, rw)
			}
		}
	default:
		cols := [4][]float64{}
		for k := 0; k < 4; k++ {
			cols[k] = make([]float64, n)
			for i, rw := range rows {
				cols[k][i] = rw.Vals[k]
			}
			cols[k] = ml.Interpolate(cols[k])
		}
		for i, rw := range rows {
			ok := true
			for k := 0; k < 4; k++ {
				rw.Vals[k] = cols[k][i]
				if math.IsNaN(cols[k][i]) {
					ok = false
				}
			}
			if ok {
				kept = append(kept, rw)
			}
		}
	}
	c := &Cleaned{Mode: in.Mode, Window: in.Window, Rows: kept}
	for _, rw := range kept {
		c.Temp = append(c.Temp, rw.Vals[0])
		c.Light = append(c.Light, rw.Vals[3])
	}
	if len(kept) < 5 {
		return nil, bad("too few valid rows remain after cleaning")
	}
	c.Smooth = ml.RollingMean(c.Temp, in.Window)
	c.LightMd = ml.Median3(c.Light)
	c.LightRM = ml.RollingMean(c.Light, 3)
	a.mu.Lock()
	a.clean = c
	a.mu.Unlock()
	// save sensor_log_clean.csv
	var sb strings.Builder
	sb.WriteString("timestamp,temperature,humidity,motion,light,temp_smoothed\n")
	for i, rw := range kept {
		sm := ""
		if !math.IsNaN(c.Smooth[i]) {
			sm = strconv.FormatFloat(c.Smooth[i], 'f', 3, 64)
		}
		fmt.Fprintf(&sb, "%.3f,%v,%v,%v,%v,%s\n", rw.TS, rw.Vals[0], rw.Vals[1], rw.Vals[2], rw.Vals[3], sm)
	}
	os.WriteFile(filepath.Join(a.Dir, "sensor_log_clean.csv"), []byte(sb.String()), 0o644)

	ev := fmt.Sprintf("%s: %d of %d rows kept (%d NaN, %d non-numeric, %d spikes)", in.Mode, len(kept), n, nanRows, coerced, spikes)
	a.mark("cleaned", ev)
	a.mark("smoothed", fmt.Sprintf("rolling mean window=%d on temperature", in.Window))
	if in.Median {
		a.mark("median", "3-point median vs rolling mean on light")
	}
	a.setHist("lab03", fmt.Sprintf("## Lab 3 — Data collection and preprocessing\n\n- Rows logged: %d (%s)\n- Cleaning (%s): %d rows kept; %d NaN readings, %d non-numeric lines, %d temperature spikes\n- Rolling-mean window: %d\n", n, a.dataSource, in.Mode, len(kept), nanRows, coerced, spikes, in.Window))
	a.setCharts("lab03",
		chartSpec{Kind: "line", Title: "Temperature: raw vs rolling mean", XLabel: "sample number (deg C)", Series: []pdf.Series{{Name: "raw", Y: tail(c.Temp, 300)}, {Name: fmt.Sprintf("smoothed (window %d)", in.Window), Y: tail(c.Smooth, 300)}}},
		chartSpec{Kind: "line", Title: "Light: raw vs median(3) vs rolling mean(3)", XLabel: "sample number (ADC)", Series: []pdf.Series{{Name: "raw", Y: tail(c.Light, 300)}, {Name: "median(3)", Y: tail(c.LightMd, 300)}, {Name: "rolling mean(3)", Y: tail(c.LightRM, 300)}}})
	cut := func(v []float64) []float64 {
		if len(v) > 400 {
			return v[len(v)-400:]
		}
		return v
	}
	return map[string]any{
		"total": n, "kept": len(kept), "removed": n - len(kept), "nanRows": nanRows, "nonNumeric": coerced, "spikes": spikes,
		"problems": probs, "window": in.Window, "mode": in.Mode,
		"temp": jfs(cut(c.Temp)), "smooth": jfs(cut(c.Smooth)),
		"light": jfs(cut(c.Light)), "median": jfs(cut(c.LightMd)), "lightRoll": jfs(cut(c.LightRM)),
		"file": filepath.Join(a.Dir, "sensor_log_clean.csv"),
	}, nil
}
