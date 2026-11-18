// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.

package app

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"aiotsim/internal/sim"
)

func (a *App) rm(names ...string) {
	for _, n := range names {
		ms, _ := filepath.Glob(filepath.Join(a.Dir, n))
		for _, m := range ms {
			os.Remove(m)
		}
	}
}

func (a *App) hReset(r *http.Request) (any, error) {
	var in struct {
		Scope   string `json:"scope"` // all | lab
		Lab     int    `json:"lab"`
		Files   bool   `json:"files"`
		Student bool   `json:"student"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	switch in.Scope {
	case "all":
		a.resetAll(in.Files, in.Student)
		a.log("warn", "Simulator reset to factory state")
	case "lab":
		if in.Lab < 1 || in.Lab > 16 {
			return nil, bad("lab must be between 1 and 16")
		}
		a.resetLab(in.Lab)
		a.log("warn", fmt.Sprintf("Lab %d reset – you can start it again", in.Lab))
	default:
		return nil, bad("scope must be all or lab")
	}
	a.hub.Send("reset", map[string]any{"scope": in.Scope, "lab": in.Lab})
	return a.state(), nil
}

func (a *App) resetAll(files, student bool) {
	a.B.Stop()
	a.resetClients()
	a.mu.Lock()
	a.logging, a.rows, a.clean, a.dataSource, a.logFields = false, nil, nil, "", 4
	a.serial, a.monitorBaud, a.autoVary = nil, 9600, true
	a.users, a.allowAnon = map[string]string{}, true
	a.topics, a.lastVals = map[string]*topicVal{}, map[string]float64{}
	a.bridge, a.tbPub = bridgeCfg{Interval: 2}, tbCfg{Interval: 2}
	a.devices, a.rules, a.alarms = nil, nil, nil
	a.iforest = nil
	a.anomaly.Enabled, a.anomaly.Flagged, a.anomaly.Checked, a.anomaly.Last, a.anomaly.Contamination = false, 0, 0, "", 0
	a.nn, a.mcu, a.proj = nil, nil, nil
	a.hist, a.charts = map[string]any{}, map[string][]chartSpec{}
	a.prog, a.ev = map[string]bool{}, map[string]string{}
	a.saveProgressLocked()
	lan, port := a.mqttAll, a.mqttPort
	a.mu.Unlock()
	a.S.SetEnv(sim.DefaultEnv())
	a.S.SetWiring(sim.CorrectWiring())
	a.S.SetDHT22(false)
	a.S.SetFaulty(false)
	a.B.Configure("combined", 9600, 1)
	if lan {
		a.StartBroker(port, false)
	}
	if files {
		a.rm("sensor_log*.csv", "comfort_model*.json", "isolation_forest.json", "model_data.h", "test_log.csv")
		os.RemoveAll(filepath.Join(a.Dir, "circuits"))
	}
	if student {
		a.setStudent(Student{})
	}
}

// resetLab clears one lab's results, progress and generated files so it can be redone.
func (a *App) resetLab(n int) {
	clear := func(lab int) {
		d := labDefs[lab]
		for _, it := range append(append([]item{}, d.Req...), d.Task...) {
			delete(a.prog, it.Key)
			delete(a.ev, it.Key)
		}
		k := fmt.Sprintf("lab%02d", lab)
		delete(a.hist, k)
		delete(a.charts, k)
	}
	var restartBroker bool
	var port int
	var closeClients bool
	a.mu.Lock()
	clear(n)
	switch n {
	case 2:
		defer func() { a.B.Stop(); a.S.SetWiring(sim.Wiring{}); a.B.Configure("blink", 9600, 1) }()
	case 3:
		a.logging, a.rows, a.clean, a.dataSource = false, nil, nil, ""
		defer a.rm("sensor_log*.csv")
	case 5:
		a.users, a.allowAnon, a.topics, a.lastVals = map[string]string{}, true, map[string]*topicVal{}, map[string]float64{}
		a.bridge = bridgeCfg{Interval: 2}
		delete(a.ev, "mqtt_refused")
		restartBroker, port, closeClients = a.mqttAll, a.mqttPort, true
	case 6:
		for _, k := range []string{"cmp_dt", "cmp_rf", "cmp_knn", "cm_dt", "cm_rf", "cm_knn", "rules_dt"} {
			delete(a.hist, k)
		}
		defer a.rm("comfort_model_dt.json", "comfort_model_rf.json", "comfort_model_knn.json")
	case 7:
		a.nn, a.mcu = nil, nil
		clear(9) // Lab 9 depends on the Lab 7 model
		defer a.rm("comfort_model.keras.json", "comfort_model.tflite.json", "comfort_model_quant.tflite.json", "model_data.h")
	case 9:
		a.mcu = nil
		defer a.rm("model_data.h")
	case 10:
		a.devices, a.rules, a.alarms = nil, nil, nil
		a.tbPub = tbCfg{Interval: 2}
		closeClients = true
	case 11:
		a.iforest = nil
		a.anomaly.Enabled, a.anomaly.Flagged, a.anomaly.Checked, a.anomaly.Last, a.anomaly.Contamination = false, 0, 0, "", 0
		defer a.rm("isolation_forest.json")
	case 13:
		a.proj = nil
	case 14:
		if a.proj != nil {
			a.proj.Log = nil
		}
		defer a.rm("test_log.csv")
	}
	a.saveProgressLocked()
	a.mu.Unlock()
	if closeClients {
		a.resetClients()
	}
	if restartBroker {
		a.StartBroker(port, false)
	}
}
