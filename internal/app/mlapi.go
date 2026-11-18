// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.

package app

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aiotsim/internal/ml"
	"aiotsim/internal/mqtt"
	"aiotsim/internal/pdf"
	"aiotsim/internal/sim"
)

var comfortFeatures = []string{"temperature", "humidity", "light"}

type comfortSet struct {
	Xtr, Xte [][]float64
	ytr, yte []int
	Neg, Pos int
	Source   string
}

// comfort builds the labelled dataset of Lab 6 (comfortable = 20..28 C and humidity <= 60 %).
func (a *App) comfort() (*comfortSet, error) {
	a.mu.Lock()
	c, rows, src := a.clean, a.rows, a.dataSource
	a.mu.Unlock()
	var use []Row
	if c != nil {
		use = c.Rows
	} else {
		for _, r := range rows { // df.dropna()
			ok := true
			for k := 0; k < 4 && k < len(r.Raw); k++ {
				if math.IsNaN(r.Vals[k]) {
					ok = false
				}
			}
			if ok {
				use = append(use, r)
			}
		}
	}
	if len(use) < 60 {
		return nil, bad("Not enough clean rows (%d). Complete Lab 3 first: log 200+ rows (or generate the instructor sample dataset) and clean them.", len(use))
	}
	var X [][]float64
	var y []int
	cs := &comfortSet{Source: src}
	for _, r := range use {
		t, h, l := r.Vals[0], r.Vals[1], r.Vals[3]
		lab := 0
		if t >= 20 && t <= 28 && h <= 60 {
			lab = 1
			cs.Pos++
		} else {
			cs.Neg++
		}
		X = append(X, []float64{t, h, l})
		y = append(y, lab)
	}
	if cs.Pos < 5 || cs.Neg < 5 {
		return nil, bad("The dataset has too few examples of one class (comfortable=%d, not comfortable=%d). Vary the conditions while logging (turn Auto-vary on, or use the heat / cover events).", cs.Pos, cs.Neg)
	}
	cs.Xtr, cs.Xte, cs.ytr, cs.yte = ml.Split(X, y, 0.25, 42)
	return cs, nil
}

func predictAll(X [][]float64, f func([]float64) int) []int {
	p := make([]int, len(X))
	for i, x := range X {
		p[i] = f(x)
	}
	return p
}

func (a *App) saveJSON(name string, v any) string {
	p := filepath.Join(a.Dir, name)
	b, _ := json.MarshalIndent(v, "", "  ")
	os.WriteFile(p, b, 0o644)
	return p
}

// ---------------------------------------------------------------- Lab 7 / 9 types

type nnResult struct {
	NN        *ml.NN
	Q         *ml.NN
	Logs      []ml.EpochLog
	Set       *comfortSet
	TestAcc   float64
	QAcc      float64
	MaxErr    float64
	Converted bool
	QConv     bool
}

type mcuResult struct {
	Header string
	Len    int
	Name   string
	Need   int
	Best   int
}

type mcuBoard struct {
	Name         string
	MHz          float64
	FlashKB, RAM int
	RuntimeKB    int
	CyclesPerMAC float64
}

var mcuBoards = map[string]mcuBoard{
	"nano33": {"Arduino Nano 33 BLE Sense (nRF52840, Cortex-M4F)", 64, 1024, 256, 140, 9},
	"esp32":  {"ESP32 DevKit (Xtensa LX6, dual core)", 240, 4096, 520, 320, 6},
}

type sizeEst struct {
	Keras, TFLite, Quant int
	Params               int
}

// estimateSizes approximates the file sizes of the three artefacts of Lab 7.
// Real sizes depend on the TensorFlow version; the fixed overheads below are typical
// for tiny models, so very small networks show little or no size reduction.
func estimateSizes(m *ml.NN) sizeEst {
	w := m.In*m.H + m.H
	b := m.H + 1
	return sizeEst{Keras: 15200 + 4*(w+b), TFLite: 1400 + 4*(w+b), Quant: 1650 + w + 4*b, Params: w + b}
}

func tflmArena(m *ml.NN) int {
	peak := m.In + m.H
	if m.H+2 > peak {
		peak = m.H + 2
	}
	return 1536 + peak + 16
}

// ---------------------------------------------------------------- PdM helpers

func (a *App) pdmFeatures(window int, extra string, n int, seed int64) (X [][]float64, y []int, healthy, degraded []float64) {
	gen := func(faulty bool) []float64 {
		s := sim.NewSensors(seed + map[bool]int64{true: 1, false: 0}[faulty])
		e := s.GetEnv()
		e.Glitch = 0
		s.SetEnv(e)
		s.SetFaulty(faulty)
		v := make([]float64, n)
		for i := range v {
			v[i] = s.Read(float64(i)).Vib
		}
		return v
	}
	healthy, degraded = gen(false), gen(true)
	for lab, v := range [][]float64{healthy, degraded} {
		m, sd, mx := ml.RollingMean(v, window), ml.RollingStd(v, window), ml.RollingMax(v, window)
		var ex []float64
		if extra != "none" {
			ex = ml.RollingFeature(v, window, extra)
		}
		for i := window - 1; i < len(v); i++ {
			row := []float64{m[i], sd[i], mx[i]}
			if ex != nil {
				row = append(row, ex[i])
			}
			X = append(X, row)
			y = append(y, lab)
		}
	}
	return
}

func (a *App) routesML(get, post func(string, func(*http.Request) (any, error)), mux *http.ServeMux) {
	post("/api/ml/train", a.hTrain)
	post("/api/nn/train", a.hNNTrain)
	post("/api/nn/convert", a.hNNConvert)
	post("/api/nn/verify", a.hNNVerify)
	post("/api/mcu/header", a.hMCUHeader)
	post("/api/mcu/run", a.hMCURun)
	post("/api/anomaly/train", a.hAnomalyTrain)
	post("/api/anomaly/live", a.hAnomalyLive)
	post("/api/pdm/run", a.hPdm)
}

// ---------------------------------------------------------------- Lab 6

func (a *App) hTrain(r *http.Request) (any, error) {
	var in struct {
		Algo        string `json:"algo"`
		MaxDepth    int    `json:"maxDepth"`
		K           int    `json:"k"`
		Standardise bool   `json:"standardise"`
		Trees       int    `json:"trees"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	cs, err := a.comfort()
	if err != nil {
		return nil, err
	}
	if in.MaxDepth < 0 || in.MaxDepth > 30 {
		in.MaxDepth = 0
	}
	if in.K < 1 || in.K > 50 {
		in.K = 5
	}
	if in.Trees < 1 || in.Trees > 300 {
		in.Trees = 100
	}
	idx := make([]int, len(cs.Xtr))
	for i := range idx {
		idx[i] = i
	}
	out := map[string]any{"algo": in.Algo, "trainN": len(cs.Xtr), "testN": len(cs.Xte),
		"labelCounts": map[string]int{"0": cs.Neg, "1": cs.Pos}, "features": comfortFeatures}
	var pred []int
	start := time.Now()
	switch in.Algo {
	case "dt":
		t := ml.FitTree(cs.Xtr, cs.ytr, idx, ml.TreeOpts{MaxDepth: in.MaxDepth})
		pred = predictAll(cs.Xte, t.Predict)
		out["importances"] = t.Importances()
		out["depth"], out["leaves"] = t.Depth(), t.Leaves()
		out["rules"] = t.Rules(comfortFeatures)
	case "rf":
		f := ml.FitForest(cs.Xtr, cs.ytr, in.Trees, in.MaxDepth, 42)
		pred = predictAll(cs.Xte, f.Predict)
		out["importances"] = f.Importances()
		out["trees"] = in.Trees
	case "knn":
		k := ml.FitKNN(cs.Xtr, cs.ytr, in.K, in.Standardise)
		pred = predictAll(cs.Xte, k.Predict)
		out["k"], out["standardised"] = in.K, in.Standardise
	default:
		return nil, bad("algo must be dt, rf or knn")
	}
	rep := ml.Evaluate(cs.yte, pred)
	out["report"] = rep
	out["trainMs"] = round(float64(time.Since(start).Microseconds())/1000, 2)
	file := a.saveJSON("comfort_model_"+in.Algo+".json", map[string]any{"algorithm": in.Algo, "features": comfortFeatures,
		"label_rule": "20<=temperature<=28 and humidity<=60", "metrics": rep, "note": "Simulator model description (not a scikit-learn joblib file)"})
	out["file"] = file
	a.mu.Lock()
	a.hist["cmp_"+in.Algo] = rep
	dtRep, hasDT := a.hist["cmp_dt"].(ml.Report)
	a.mu.Unlock()
	if in.Algo == "dt" {
		a.mark("dt_trained", fmt.Sprintf("decision tree accuracy %.3f on %d test rows", rep.Accuracy, len(cs.Xte)))
	} else if hasDT {
		better := "second model"
		if dtRep.Accuracy > rep.Accuracy {
			better = "decision tree"
		} else if dtRep.Accuracy == rep.Accuracy {
			better = "tie"
		}
		out["comparison"] = map[string]any{"dtAccuracy": dtRep.Accuracy, "thisAccuracy": rep.Accuracy, "better": better}
		a.mark("second_model", fmt.Sprintf("%s accuracy %.3f vs decision tree %.3f", in.Algo, rep.Accuracy, dtRep.Accuracy))
	}
	if in.Algo == "dt" {
		a.mu.Lock()
		a.hist["rules_dt"], _ = out["rules"].(string)
		a.mu.Unlock()
	}
	var sb strings.Builder
	sb.WriteString("## Lab 6 — Supervised learning on the sensor dataset\n\nLabel rule: comfortable = 20 <= temperature <= 28 and humidity <= 60. Split 75 / 25.\n\n| Model | Accuracy | Precision | Recall |\n|---|---|---|---|\n")
	for _, al := range []string{"dt", "rf", "knn"} {
		a.mu.Lock()
		rp, ok := a.hist["cmp_"+al].(ml.Report)
		a.mu.Unlock()
		if ok {
			fmt.Fprintf(&sb, "| %s | %.3f | %.3f | %.3f |\n", al, rp.Accuracy, rp.Precision, rp.Recall)
		}
	}
	for _, al := range []string{"dt", "rf", "knn"} {
		a.mu.Lock()
		rp, ok := a.hist["cmp_"+al].(ml.Report)
		a.mu.Unlock()
		if ok {
			c := rp.Confusion
			fmt.Fprintf(&sb, "\n### Confusion matrix (%s)\n\n| | predicted 0 | predicted 1 |\n|---|---|---|\n| true 0 | %d | %d |\n| true 1 | %d | %d |\n", al, c[0][0], c[0][1], c[1][0], c[1][1])
		}
	}
	a.mu.Lock()
	if rules, _ := a.hist["rules_dt"].(string); rules != "" {
		fmt.Fprintf(&sb, "\n### Decision-tree rules\n\n```\n%s```\n", rules)
	}
	a.mu.Unlock()
	a.setHist("lab06", sb.String())
	return out, nil
}

// ---------------------------------------------------------------- Lab 7

func (a *App) hNNTrain(r *http.Request) (any, error) {
	var in struct {
		Hidden    int     `json:"hidden"`
		Epochs    int     `json:"epochs"`
		LR        float64 `json:"lr"`
		Normalise bool    `json:"normalise"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if in.Hidden < 1 || in.Hidden > 256 {
		in.Hidden = 8
	}
	if in.Epochs < 1 || in.Epochs > 200 {
		in.Epochs = 30
	}
	if in.LR <= 0 || in.LR > 1 {
		in.LR = 0.01
	}
	cs, err := a.comfort()
	if err != nil {
		return nil, err
	}
	nn, logs := ml.TrainNN(cs.Xtr, cs.ytr, in.Hidden, in.Epochs, in.LR, in.Normalise, 42)
	acc := ml.Evaluate(cs.yte, predictAll(cs.Xte, nn.Predict)).Accuracy
	q, maxErr := nn.Quantise()
	qacc := ml.Evaluate(cs.yte, predictAll(cs.Xte, q.Predict)).Accuracy
	a.mu.Lock()
	a.nn = &nnResult{NN: nn, Q: q, Logs: logs, Set: cs, TestAcc: acc, QAcc: qacc, MaxErr: maxErr}
	a.mu.Unlock()
	a.mark("nn_trained", fmt.Sprintf("Keras-style model %d-%d-1 trained %d epochs, test accuracy %.3f", nn.In, in.Hidden, in.Epochs, acc))
	sz := estimateSizes(nn)
	a.saveJSON("comfort_model.keras.json", map[string]any{"architecture": fmt.Sprintf("Input(3) -> Dense(%d, relu) -> Dense(1, sigmoid)", in.Hidden),
		"params": sz.Params, "test_accuracy": acc, "note": "Simulator description of the trained model (not a real .keras archive)"})
	a.setNNHist()
	var tr, va, vl []float64
	for _, h := range logs {
		tr, va, vl = append(tr, h.Acc), append(va, h.VAcc), append(vl, h.VLoss)
	}
	a.setCharts("lab07", chartSpec{Kind: "line", Title: "Training curves", XLabel: "epoch", Series: []pdf.Series{{Name: "train accuracy", Y: tr}, {Name: "validation accuracy", Y: va}, {Name: "validation loss", Y: vl}}})
	return map[string]any{"history": logs, "testAcc": acc, "params": sz.Params, "kerasBytes": sz.Keras, "hidden": in.Hidden,
		"normalised": in.Normalise, "lr": in.LR,
		"summary": fmt.Sprintf("Layer 1: Dense(%d, relu) params %d | Layer 2: Dense(1, sigmoid) params %d | Total %d", in.Hidden, nn.In*in.Hidden+in.Hidden, in.Hidden+1, sz.Params)}, nil
}

func (a *App) setNNHist() {
	a.mu.Lock()
	n := a.nn
	a.mu.Unlock()
	if n == nil {
		return
	}
	s := estimateSizes(n.NN)
	a.setHist("lab07", fmt.Sprintf("## Lab 7 — TensorFlow Lite conversion (simulated)\n\n| Artefact | Size (bytes, estimated) |\n|---|---|\n| Keras model | %d |\n| TFLite (float32) | %d |\n| TFLite (int8-quantised) | %d |\n\nTest accuracy: float %.3f, quantised %.3f (max weight error %.5f)\n", s.Keras, s.TFLite, s.Quant, n.TestAcc, n.QAcc, n.MaxErr))
}

func (a *App) hNNConvert(r *http.Request) (any, error) {
	var in struct {
		Quantise bool `json:"quantise"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	a.mu.Lock()
	n := a.nn
	a.mu.Unlock()
	if n == nil {
		return nil, bad("Train the network first (Steps 3-6).")
	}
	s := estimateSizes(n.NN)
	a.mu.Lock()
	if in.Quantise {
		n.QConv = true
	} else {
		n.Converted = true
	}
	both := n.Converted && n.QConv
	a.mu.Unlock()
	res := map[string]any{"keras": s.Keras, "tflite": s.TFLite, "quant": s.Quant, "quantise": in.Quantise}
	if in.Quantise {
		a.mark("quant", fmt.Sprintf("comfort_model_quant.tflite ≈ %d bytes", s.Quant))
		res["file"] = a.saveJSON("comfort_model_quant.tflite.json", map[string]any{"format": "simulated int8 TFLite", "bytes_estimate": s.Quant, "max_weight_error": n.MaxErr})
		res["accuracy"], res["maxErr"] = n.QAcc, n.MaxErr
		res["note"] = "int8 weights, float biases (dynamic-range quantisation). For very small networks the fixed flatbuffer overhead dominates, so the file may not shrink; try a larger hidden layer to see the saving."
	} else {
		a.mark("tflite", fmt.Sprintf("comfort_model.tflite ≈ %d bytes", s.TFLite))
		res["file"] = a.saveJSON("comfort_model.tflite.json", map[string]any{"format": "simulated float32 TFLite", "bytes_estimate": s.TFLite})
	}
	if both {
		a.mark("sizes", fmt.Sprintf("Keras %d B, TFLite %d B, quantised %d B", s.Keras, s.TFLite, s.Quant))
	}
	a.setNNHist()
	return res, nil
}

func (a *App) hNNVerify(r *http.Request) (any, error) {
	var in struct {
		Index int `json:"index"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	a.mu.Lock()
	n := a.nn
	a.mu.Unlock()
	if n == nil || !n.QConv {
		return nil, bad("Convert and quantise the model first (Steps 7-8).")
	}
	if in.Index < 0 || in.Index >= len(n.Set.Xte) {
		in.Index = 0
	}
	x := n.Set.Xte[in.Index]
	k, q := n.NN.Proba(x), n.Q.Proba(x)
	// timing on this workstation
	t0 := time.Now()
	for i := 0; i < 2000; i++ {
		n.NN.Proba(x)
	}
	kt := float64(time.Since(t0).Nanoseconds()) / 2000 / 1000
	t0 = time.Now()
	for i := 0; i < 2000; i++ {
		n.Q.Proba(x)
	}
	qt := float64(time.Since(t0).Nanoseconds()) / 2000 / 1000
	same := (k >= 0.5) == (q >= 0.5)
	ok := same && math.Abs(k-q) < 0.05
	if ok {
		a.mark("verified", fmt.Sprintf("TFLite %.4f vs Keras %.4f on the same input", q, k))
	}
	return map[string]any{"input": x, "tflite": q, "keras": k, "diff": math.Abs(k - q), "sameClass": same, "equivalent": ok,
		"kerasUs": round(kt, 2), "tfliteUs": round(qt, 2), "label": n.Set.yte[in.Index]}, nil
}

// ---------------------------------------------------------------- Lab 9

func (a *App) buildModelBlob(n *nnResult) []byte {
	q := n.Q
	var b []byte
	b = append(b, 'S', 'I', 'M', '3') // simulated flatbuffer identifier
	b = append(b, byte(q.In), byte(q.H))
	f32 := func(v float64) { b = binary.LittleEndian.AppendUint32(b, math.Float32bits(float32(v))) }
	var mx1, mx2 float64
	for _, r := range q.W1 {
		for _, v := range r {
			mx1 = math.Max(mx1, math.Abs(v))
		}
	}
	for _, v := range q.W2 {
		mx2 = math.Max(mx2, math.Abs(v))
	}
	f32(mx1 / 127)
	f32(mx2 / 127)
	for _, r := range q.W1 {
		for _, v := range r {
			b = append(b, byte(int8(math.Round(v/(mx1/127+1e-12)))))
		}
	}
	for _, v := range q.W2 {
		b = append(b, byte(int8(math.Round(v/(mx2/127+1e-12)))))
	}
	for _, v := range q.B1 {
		f32(v)
	}
	f32(q.B2)
	for j := 0; j < q.In; j++ {
		f32(q.Mean[j])
	}
	for j := 0; j < q.In; j++ {
		f32(q.Std[j])
	}
	for len(b)%16 != 0 {
		b = append(b, 0)
	}
	return b
}

func (a *App) hMCUHeader(r *http.Request) (any, error) {
	a.mu.Lock()
	n := a.nn
	a.mu.Unlock()
	if n == nil || !n.QConv {
		return nil, bad("Lab 7 first: train, convert and quantise the model.")
	}
	blob := a.buildModelBlob(n)
	name := "comfort_model_quant_tflite"
	var sb strings.Builder
	fmt.Fprintf(&sb, "unsigned char %s[] = {", name)
	for i, v := range blob {
		if i%12 == 0 {
			sb.WriteString("\n ")
		}
		fmt.Fprintf(&sb, " 0x%02x,", v)
	}
	s := strings.TrimSuffix(sb.String(), ",")
	s += fmt.Sprintf("\n};\nunsigned int %s_len = %d;\n", name, len(blob))
	need := tflmArena(n.Q)
	a.mu.Lock()
	a.mcu = &mcuResult{Header: s, Len: len(blob), Name: name, Need: need}
	a.mu.Unlock()
	os.WriteFile(filepath.Join(a.Dir, "model_data.h"), []byte(s), 0o644)
	a.mark("header", fmt.Sprintf("model_data.h generated: %s_len = %d", name, len(blob)))
	return map[string]any{"header": s, "name": name, "len": len(blob), "command": "xxd -i comfort_model_quant.tflite > model_data.h",
		"file": filepath.Join(a.Dir, "model_data.h"), "note": "The simulator emits a simulated model container (identifier SIM3), not a real TFLite flatbuffer."}, nil
}

func (a *App) hMCURun(r *http.Request) (any, error) {
	var in struct {
		Board string    `json:"board"`
		Arena int       `json:"arena"`
		Input []float64 `json:"input"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	bd, ok := mcuBoards[in.Board]
	if !ok {
		return nil, bad("board must be nano33 or esp32")
	}
	a.mu.Lock()
	n, m := a.nn, a.mcu
	a.mu.Unlock()
	if n == nil || m == nil {
		return nil, bad("Generate model_data.h first (Step 3).")
	}
	if in.Arena < 256 || in.Arena > 200000 {
		return nil, bad("arena must be between 256 and 200000 bytes")
	}
	x := in.Input
	if len(x) != 3 {
		x = n.Set.Xte[0]
	}
	flash := m.Len + bd.RuntimeKB*1024
	ram := in.Arena + 14*1024
	lines := []string{
		fmt.Sprintf("Sketch uses %d bytes (%d%%) of program storage space. Maximum is %d bytes.", flash, flash*100/(bd.FlashKB*1024), bd.FlashKB*1024),
		fmt.Sprintf("Global variables use %d bytes (%d%%) of dynamic memory (tensor arena = %d).", ram, ram*100/(bd.RAM*1024), in.Arena),
		"Upload complete. Opening Serial Monitor @ 9600 baud...",
	}
	need := m.Need
	res := map[string]any{"board": bd.Name, "need": need, "arena": in.Arena, "flash": flash, "ram": ram}
	if ram > bd.RAM*1024 {
		lines = append(lines, "Sketch too big: not enough dynamic memory.")
		res["ok"], res["lines"] = false, lines
		return res, nil
	}
	if in.Arena < need {
		lines = append(lines, fmt.Sprintf("Arena size is too small for all buffers. Needed %d but only %d was available.", need, in.Arena),
			"AllocateTensors() failed")
		res["ok"], res["lines"] = false, lines
		return res, nil
	}
	p := n.Q.Proba(x)
	cycles := 18000 + float64(n.Q.MACs())*bd.CyclesPerMAC
	us := cycles / bd.MHz
	ms := int(us / 1000)
	lines = append(lines, fmt.Sprintf("Inference output: %.4f (class %d)", p, map[bool]int{true: 1, false: 0}[p >= 0.5]),
		fmt.Sprintf("Inference time (ms): %d", ms), fmt.Sprintf("Inference time (us): %d", int(us)))
	want := n.Q.Proba(x)
	match := math.Abs(p-want) < 0.03
	res["ok"], res["lines"], res["output"], res["us"], res["ms"], res["match"] = true, lines, p, round(us, 0), ms, match
	res["workstationMicros"] = "see Lab 7 verify step"
	if match {
		a.mark("deployed", fmt.Sprintf("%s: output %.4f matches workstation, ≈%d µs per inference", in.Board, p, int(us)))
	}
	a.mu.Lock()
	if m.Best == 0 || in.Arena < m.Best {
		m.Best = in.Arena
	}
	best := m.Best
	a.mu.Unlock()
	res["smallestWorking"] = best
	if in.Arena-need <= 128 {
		a.mark("arena_min", fmt.Sprintf("smallest working tensor arena ≈ %d bytes (need %d)", in.Arena, need))
	}
	a.setHist("lab09", fmt.Sprintf("## Lab 9 — On-device inference (simulated %s)\n\n- model_data.h length: %d bytes\n- Smallest working tensor arena found: %d bytes\n- Estimated inference time: %d µs\n", bd.Name, m.Len, best, int(us)))
	return res, nil
}

// ---------------------------------------------------------------- Lab 11

func (a *App) anomalyData() ([][]float64, error) {
	cs, err := a.comfort()
	if err != nil {
		return nil, err
	}
	X := append(append([][]float64{}, cs.Xtr...), cs.Xte...)
	return X, nil
}

func (a *App) hAnomalyTrain(r *http.Request) (any, error) {
	var in struct {
		Contamination float64   `json:"contamination"`
		Sweep         []float64 `json:"sweep"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if in.Contamination <= 0 || in.Contamination >= 0.5 {
		in.Contamination = 0.05
	}
	X, err := a.anomalyData()
	if err != nil {
		return nil, err
	}
	f := ml.FitIForest(X, in.Contamination, 42)
	flag := make([]bool, len(X))
	n := 0
	for i, x := range X {
		if f.Predict(x) == -1 {
			flag[i] = true
			n++
		}
	}
	a.mu.Lock()
	a.iforest = f
	a.anomaly.Contamination = in.Contamination
	a.mu.Unlock()
	a.mark("iforest", fmt.Sprintf("Isolation Forest trained, contamination=%.2f: %d of %d rows flagged", in.Contamination, n, len(X)))
	a.saveJSON("isolation_forest.json", map[string]any{"contamination": in.Contamination, "trees": 100, "threshold": f.Thr, "flagged": n, "rows": len(X), "note": "Simulator model description"})
	// synthetic faults to quantify the false-positive / missed-anomaly trade-off
	inject := func(f *ml.IForest) (rate float64) {
		hit := 0
		for i := 0; i < 100; i++ {
			x := append([]float64(nil), X[i%len(X)]...)
			x[0], x[1], x[2] = x[0]+4+float64(i%6), x[1]-6-float64(i%9), x[2]*0.45 // moderately hot, dry and dim
			if f.Predict(x) == -1 {
				hit++
			}
		}
		return float64(hit) / 100
	}
	out := map[string]any{"normal": len(X) - n, "anomalies": n, "rows": len(X), "contamination": in.Contamination,
		"detectRate": inject(f), "temp": nil}
	t := make([]float64, 0, len(X))
	for _, x := range X {
		t = append(t, x[0])
	}
	if len(t) > 400 {
		t, flag = t[:400], flag[:400]
	}
	a.setCharts("lab11", chartSpec{Kind: "line", Title: "Temperature with anomalies (red)", XLabel: "row", Series: []pdf.Series{{Name: "temperature", Y: t, Marks: flag}}})
	out["temp"], out["flags"] = jfs(t), flag
	a.setHist("lab11", fmt.Sprintf("## Lab 11 — Isolation Forest\n\n- contamination %.2f: %d of %d rows flagged as anomalies\n- injected faults detected: %.0f%%\n", in.Contamination, n, len(X), inject(f)*100))
	if len(in.Sweep) > 0 {
		var table []map[string]any
		for _, c := range in.Sweep {
			if c <= 0 || c >= 0.5 {
				continue
			}
			ff := ml.FitIForest(X, c, 42)
			k := 0
			for _, x := range X {
				if ff.Predict(x) == -1 {
					k++
				}
			}
			table = append(table, map[string]any{"contamination": c, "flagged": k, "detectRate": inject(ff)})
		}
		out["sweep"] = table
		if len(table) >= 2 {
			a.mark("sweep", fmt.Sprintf("%d contamination values compared", len(table)))
		}
		var sb strings.Builder
		sb.WriteString("## Lab 11 — Isolation Forest contamination sweep\n\n| contamination | rows flagged | injected faults detected |\n|---|---|---|\n")
		for _, row := range table {
			fmt.Fprintf(&sb, "| %.2f | %v | %.0f%% |\n", row["contamination"], row["flagged"], row["detectRate"].(float64)*100)
		}
		a.setHist("lab11", sb.String())
	}
	return out, nil
}

type anomalyCfg struct {
	Enabled       bool
	Contamination float64
	Flagged       int
	Checked       int
	Last          string
	cancel        func()
}

func (a *App) anomalyView() map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()
	return map[string]any{"trained": a.iforest != nil, "enabled": a.anomaly.Enabled, "contamination": a.anomaly.Contamination,
		"flagged": a.anomaly.Flagged, "checked": a.anomaly.Checked, "last": a.anomaly.Last}
}

func (a *App) attachDetector() {
	a.mu.Lock()
	if a.anomaly.cancel != nil {
		a.anomaly.cancel()
	}
	b := a.Broker
	a.mu.Unlock()
	if b == nil {
		return
	}
	cancel := b.Subscribe("aiot/lab5/light", func(m mqtt.Message) {
		a.mu.Lock()
		f, en := a.iforest, a.anomaly.Enabled
		t, okT := a.lastVals["temperature"]
		h, okH := a.lastVals["humidity"]
		l, okL := a.lastVals["light"]
		a.mu.Unlock()
		if f == nil || !en || !okT || !okH || !okL || math.IsNaN(t+h+l) {
			return
		}
		verdict := f.Predict([]float64{t, h, l})
		a.mu.Lock()
		a.anomaly.Checked++
		if verdict == -1 {
			a.anomaly.Flagged++
			a.anomaly.Last = fmt.Sprintf("T=%.1f H=%.1f L=%.0f", t, h, l)
		}
		a.mu.Unlock()
		if verdict == -1 {
			b.Publish("aiot/lab11/anomaly", []byte("1"), false)
			a.mark("live_detect", fmt.Sprintf("anomaly published for T=%.1f H=%.1f L=%.0f", t, h, l))
		}
		a.hub.Send("anomaly", a.anomalyView())
	})
	a.mu.Lock()
	a.anomaly.cancel = cancel
	a.mu.Unlock()
}

func (a *App) hAnomalyLive(r *http.Request) (any, error) {
	var in struct {
		Enabled bool `json:"enabled"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	a.mu.Lock()
	if in.Enabled && a.iforest == nil {
		a.mu.Unlock()
		return nil, bad("Train the Isolation Forest first (Steps 1-2).")
	}
	a.anomaly.Enabled = in.Enabled
	a.mu.Unlock()
	return a.anomalyView(), nil
}

// ---------------------------------------------------------------- Lab 12

func (a *App) hPdm(r *http.Request) (any, error) {
	var in struct {
		Window int    `json:"window"`
		Extra  string `json:"extra"`
		N      int    `json:"n"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if in.Window < 3 || in.Window > 100 {
		in.Window = 20
	}
	if in.N < 200 || in.N > 5000 {
		in.N = 800
	}
	switch in.Extra {
	case "p2p", "rms", "crest", "kurtosis":
	default:
		in.Extra = "none"
	}
	seed := time.Now().UnixNano() % 100000
	eval := func(extra string) (ml.Report, []float64) {
		X, y, _, _ := a.pdmFeatures(in.Window, extra, in.N, seed)
		Xtr, Xte, ytr, yte := ml.Split(X, y, 0.25, 42)
		f := ml.FitForest(Xtr, ytr, 60, 0, 42)
		return ml.Evaluate(yte, predictAll(Xte, f.Predict)), f.Importances()
	}
	base, imp := eval("none")
	_, _, healthy, degraded := a.pdmFeatures(in.Window, "none", in.N, seed)
	cut := func(v []float64) []float64 {
		if len(v) > 300 {
			return v[:300]
		}
		return v
	}
	out := map[string]any{"window": in.Window, "base": base, "importances": imp, "features": []string{"roll_mean", "roll_std", "roll_max"},
		"healthy": jfs(cut(healthy)), "degraded": jfs(cut(degraded)), "samplesPerClass": in.N - in.Window + 1,
		"note": "Overlapping windows leak information between train and test sets, so these scores are optimistic. Discuss this in your report."}
	a.setCharts("lab12", chartSpec{Kind: "line", Title: "Vibration magnitude (g): healthy vs degraded", XLabel: "sample", Series: []pdf.Series{{Name: "healthy", Y: cut(healthy)}, {Name: "degraded", Y: cut(degraded)}}})
	a.mark("pdm", fmt.Sprintf("window %d: accuracy %.3f precision %.3f recall %.3f", in.Window, base.Accuracy, base.Precision, base.Recall))
	md := fmt.Sprintf("## Lab 12 — Predictive maintenance\n\n| Features | Accuracy | Precision | Recall |\n|---|---|---|---|\n| mean, std, max (window %d) | %.3f | %.3f | %.3f |\n", in.Window, base.Accuracy, base.Precision, base.Recall)
	if in.Extra != "none" {
		ext, _ := eval(in.Extra)
		out["extraName"], out["extra"], out["recallDelta"] = in.Extra, ext, ext.Recall-base.Recall
		a.mark("pdm_extra", fmt.Sprintf("extra feature %s: recall %.3f vs baseline %.3f", in.Extra, ext.Recall, base.Recall))
		md += fmt.Sprintf("| + %s | %.3f | %.3f | %.3f |\n", in.Extra, ext.Accuracy, ext.Precision, ext.Recall)
	}
	a.setHist("lab12", md)
	return out, nil
}
