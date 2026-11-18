// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.

package app

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// Author and course constants shown in About, PDF reports and installers.
const (
	Author      = "Iqbal Uddin Khan"
	AuthorTitle = "Assistant Professor"
	AuthorDept  = "DIT | FEST - HU"
	CourseCode  = "AI-4301"
	CourseName  = "Artificial Intelligence of Things"
)

//go:embed license.txt
var licenseText string

// LicenseText returns the embedded license.
func LicenseText() string { return licenseText }

// Student identifies the person whose name appears on exported reports.
type Student struct {
	Name    string `json:"name"`
	Reg     string `json:"reg"`
	Section string `json:"section"`
}

var regRe = regexp.MustCompile(`^[A-Za-z0-9 ./_\-]{3,40}$`)

func validStudent(s Student) error {
	n := strings.TrimSpace(s.Name)
	if len([]rune(n)) < 2 || len([]rune(n)) > 80 {
		return bad("Enter the student's full name (2-80 characters).")
	}
	for _, r := range n {
		if !(unicode.IsLetter(r) || r == ' ' || r == '.' || r == '\'' || r == '-') {
			return bad("The name may contain only letters, spaces, dots, apostrophes and hyphens.")
		}
	}
	if !regRe.MatchString(strings.TrimSpace(s.Reg)) {
		return bad("The registration number must be 3-40 characters: letters, digits, space . / _ -")
	}
	if len([]rune(s.Section)) > 30 {
		return bad("Section is too long (max 30 characters).")
	}
	return nil
}

func (a *App) studentPath() string { return filepath.Join(a.Dir, "student.json") }

func (a *App) loadStudent() {
	b, err := os.ReadFile(a.studentPath())
	if err != nil {
		return
	}
	var s Student
	if json.Unmarshal(b, &s) == nil && validStudent(s) == nil {
		a.student = s
	}
}

func (a *App) setStudent(s Student) {
	a.mu.Lock()
	a.student = s
	a.mu.Unlock()
	if s == (Student{}) {
		os.Remove(a.studentPath())
		return
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(a.studentPath(), b, 0o644)
}

func (a *App) getStudent() Student { a.mu.Lock(); defer a.mu.Unlock(); return a.student }

func (a *App) routesMeta(get, post func(string, func(*http.Request) (any, error))) {
	get("/api/student", func(r *http.Request) (any, error) { return a.getStudent(), nil })
	post("/api/student", func(r *http.Request) (any, error) {
		var s Student
		if err := decode(r, &s); err != nil {
			return nil, err
		}
		s.Name, s.Reg, s.Section = strings.Join(strings.Fields(s.Name), " "), strings.TrimSpace(s.Reg), strings.TrimSpace(s.Section)
		if err := validStudent(s); err != nil {
			return nil, err
		}
		a.setStudent(s)
		a.log("info", "Student details saved: "+s.Name+" ("+s.Reg+")")
		return s, nil
	})
	get("/api/about", func(r *http.Request) (any, error) {
		return map[string]any{"name": "AIoT Simulator", "version": a.Version, "course": CourseCode + " " + CourseName,
			"author": Author, "title": AuthorTitle, "dept": AuthorDept, "license": licenseText,
			"copyright": "Copyright (c) 2026 " + Author}, nil
	})
	post("/api/reset", a.hReset)
}
