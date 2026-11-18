// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.

package app

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var circuitName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,40}$`)

func (a *App) circuitDir() string { return filepath.Join(a.Dir, "circuits") }

// routesCircuit stores Circuit Lab designs (JSON written by the browser) in the workspace.
func (a *App) routesCircuit(get, post func(string, func(*http.Request) (any, error))) {
	post("/api/circuit/save", func(r *http.Request) (any, error) {
		var in struct {
			Name string          `json:"name"`
			Data json.RawMessage `json:"data"`
		}
		r.Body = http.MaxBytesReader(nil, r.Body, 400<<10)
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		if !circuitName.MatchString(in.Name) {
			return nil, bad("Use 1-40 letters, digits, - or _ for the circuit name.")
		}
		var probe map[string]any
		if json.Unmarshal(in.Data, &probe) != nil || probe == nil {
			return nil, bad("circuit data must be a JSON object")
		}
		os.MkdirAll(a.circuitDir(), 0o755)
		p := filepath.Join(a.circuitDir(), in.Name+".json")
		if err := os.WriteFile(p, in.Data, 0o644); err != nil {
			return nil, err
		}
		return map[string]any{"file": p}, nil
	})
	get("/api/circuit/list", func(r *http.Request) (any, error) {
		ms, _ := filepath.Glob(filepath.Join(a.circuitDir(), "*.json"))
		names := []string{}
		for _, m := range ms {
			names = append(names, strings.TrimSuffix(filepath.Base(m), ".json"))
		}
		sort.Strings(names)
		return names, nil
	})
	get("/api/circuit/load", func(r *http.Request) (any, error) {
		n := r.URL.Query().Get("name")
		if !circuitName.MatchString(n) {
			return nil, bad("invalid circuit name")
		}
		b, err := os.ReadFile(filepath.Join(a.circuitDir(), n+".json"))
		if err != nil {
			return nil, apiErr{404, "no such circuit"}
		}
		var v any
		if json.Unmarshal(b, &v) != nil {
			return nil, bad("saved file is not valid JSON")
		}
		return v, nil
	})
}
