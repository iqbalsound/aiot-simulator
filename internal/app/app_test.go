// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.

package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aiotsim/internal/web"
)

func call(t *testing.T, srv *httptest.Server, path string, body any) map[string]any {
	t.Helper()
	var req *http.Request
	if body == nil {
		req, _ = http.NewRequest("GET", srv.URL+path, nil)
	} else {
		b, _ := json.Marshal(body)
		req, _ = http.NewRequest("POST", srv.URL+path, bytes.NewReader(b))
		req.Header.Set("X-AIoT", "1")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)
	if resp.StatusCode >= 400 {
		m = map[string]any{"error": m["error"], "status": float64(resp.StatusCode)}
	}
	return m
}

func TestEndToEndAPI(t *testing.T) {
	a := New("test", t.TempDir(), 1)
	a.StartBroker(0, false)
	defer a.Broker.Close()
	srv := httptest.NewServer(a.Handler(web.FS()))
	defer srv.Close()

	// security: POST without the custom header and foreign Host are rejected
	r, _ := http.Post(srv.URL+"/api/quit", "application/json", nil)
	if r.StatusCode != 403 {
		t.Fatalf("POST without X-AIoT should be 403, got %d", r.StatusCode)
	}
	req, _ := http.NewRequest("GET", srv.URL+"/api/state", nil)
	req.Host = "evil.example:80"
	if r, _ = http.DefaultClient.Do(req); r.StatusCode != 403 {
		t.Fatalf("foreign Host should be 403, got %d", r.StatusCode)
	}
	// lab flow
	if m := call(t, srv, "/api/ml/train", map[string]any{"algo": "dt"}); m["error"] == nil {
		t.Fatal("training before Lab 3 data must fail with a helpful error")
	}
	call(t, srv, "/api/logger/sample", map[string]any{"n": 400})
	if m := call(t, srv, "/api/preprocess", map[string]any{"mode": "dropna", "window": 5}); m["kept"].(float64) < 300 {
		t.Fatalf("preprocess kept %v", m["kept"])
	}
	if m := call(t, srv, "/api/ml/train", map[string]any{"algo": "dt"}); m["report"].(map[string]any)["accuracy"].(float64) < 0.9 {
		t.Fatalf("dt: %v", m)
	}
	call(t, srv, "/api/nn/train", map[string]any{"hidden": 8, "epochs": 20, "normalise": true})
	call(t, srv, "/api/nn/convert", map[string]any{"quantise": false})
	call(t, srv, "/api/nn/convert", map[string]any{"quantise": true})
	if m := call(t, srv, "/api/nn/verify", map[string]any{"index": 0}); m["equivalent"] != true {
		t.Fatalf("verify: %v", m)
	}
	h := call(t, srv, "/api/mcu/header", map[string]any{})
	if h["len"].(float64) <= 0 {
		t.Fatal("header")
	}
	if m := call(t, srv, "/api/mcu/run", map[string]any{"board": "nano33", "arena": 300}); m["ok"] != false {
		t.Fatalf("tiny arena must fail: %v", m)
	}
	if m := call(t, srv, "/api/mcu/run", map[string]any{"board": "nano33", "arena": 4096}); m["ok"] != true {
		t.Fatalf("4096 arena must work: %v", m)
	}
	// MQTT authentication scenario of Lab 5
	call(t, srv, "/api/mqtt/config", map[string]any{"AddUser": map[string]string{"U": "s", "P": "p"}, "allowAnonymous": false})
	if m := call(t, srv, "/api/mqtt/test", map[string]any{"action": "connect"}); m["ok"] != false {
		t.Fatal("anonymous must be refused")
	}
	if m := call(t, srv, "/api/mqtt/test", map[string]any{"action": "connect", "useAuth": true, "user": "s", "pass": "p"}); m["ok"] != true {
		t.Fatal("authenticated must succeed")
	}
	// project tests: validation layer matters
	if m := call(t, srv, "/api/project/run", map[string]any{"template": "irrigation", "validate": true}); m["passed"] != m["total"] {
		t.Fatalf("irrigation with validation: %v", m["passed"])
	}
	if m := call(t, srv, "/api/project/run", map[string]any{"template": "irrigation", "validate": false}); m["passed"] == m["total"] {
		t.Fatal("without validation the fault scenarios must fail")
	}
	// student details, PDF export and resets
	if m := call(t, srv, "/api/student", map[string]any{"name": "A", "reg": "1"}); m["error"] == nil {
		t.Fatal("invalid student must be rejected")
	}
	r2, _ := http.Get(srv.URL + "/api/report.pdf?lab=3")
	if r2.StatusCode != 400 {
		t.Fatalf("PDF without student must be 400, got %d", r2.StatusCode)
	}
	call(t, srv, "/api/student", map[string]any{"name": "Ali Raza", "reg": "2023-CS-001", "section": "A"})
	for _, q := range []string{"3", "6", "7", "all"} {
		r3, err := http.Get(srv.URL + "/api/report.pdf?lab=" + q)
		if err != nil || r3.StatusCode != 200 || r3.Header.Get("Content-Type") != "application/pdf" {
			t.Fatalf("pdf lab=%s: %v %v", q, err, r3.StatusCode)
		}
		buf := new(bytes.Buffer)
		buf.ReadFrom(r3.Body)
		if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) || buf.Len() < 3000 {
			t.Fatalf("pdf lab=%s too small: %d", q, buf.Len())
		}
	}
	call(t, srv, "/api/reset", map[string]any{"scope": "lab", "lab": 6})
	st0 := call(t, srv, "/api/state", nil)
	for _, p := range st0["progress"].([]any) {
		pm := p.(map[string]any)
		if pm["lab"].(float64) == 6 && pm["done"] == true {
			t.Fatal("lab 6 should be reset")
		}
		if pm["lab"].(float64) == 3 && pm["done"] != true {
			t.Fatal("lab 3 must stay complete after resetting lab 6")
		}
	}
	call(t, srv, "/api/reset", map[string]any{"scope": "all", "files": true})
	st1 := call(t, srv, "/api/state", nil)
	if st1["rows"].(float64) != 0 {
		t.Fatal("rows must be cleared")
	}
	for _, p := range st1["progress"].([]any) {
		if p.(map[string]any)["done"] == true {
			t.Fatal("nothing may be complete after full reset")
		}
	}
	if call(t, srv, "/api/student", nil)["name"] != "Ali Raza" {
		t.Fatal("student details must survive a normal reset")
	}
	st := call(t, srv, "/api/state", nil)
	if st["version"] != "test" {
		t.Fatal("state")
	}
}
