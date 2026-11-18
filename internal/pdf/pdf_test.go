// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.

package pdf

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

func sample() *Doc {
	d := New()
	d.Title, d.Author, d.HeaderL, d.HeaderR, d.FooterL = "Lab test", "Iqbal Uddin Khan", "AI-4301 Lab Report", "Lab 3", "Student: Ali Raza | Reg. No.: 2023-CS-001"
	d.Banner("Lab 3: Sensor Data Collection", "Student: Ali Raza · Reg. No. 2023-CS-001 · ≥ ≤ → °C µs")
	d.H2("Status")
	d.Table([]string{"Item", "Status", "Evidence"}, [][]string{{"200 rows logged", "Done", "412 rows"}, {"Cleaned", "Pending", ""}, {"A long cell that must wrap over several lines to prove wrapping works inside table cells", "FAIL", "x"}}, []float64{3, 1, 2})
	d.H3("Code")
	d.Code("if len(parts) == 4:\n    writer.writerow([time.time()] + parts)")
	y := make([]float64, 80)
	z := make([]float64, 80)
	m := make([]bool, 80)
	for i := range y {
		y[i] = 25 + 5*math.Sin(float64(i)/8)
		z[i] = y[i] + 0.5*math.Cos(float64(i))
		m[i] = i%17 == 0
	}
	y[30] = math.NaN()
	d.LineChart("Temperature", "sample", []Series{{Name: "raw", Y: y, Marks: m}, {Name: "smooth", Y: z}}, 150)
	d.BarChart("Stages", []string{"sense", "tx", "infer", "act"}, []float64{24, 8, 0.2, 3}, 120)
	for i := 0; i < 90; i++ {
		d.Bullet("Bullet line number that goes on for a while to force multiple pages of content in the document")
	}
	return d
}

func TestPDFStructure(t *testing.T) {
	b := sample().Bytes()
	if !bytes.HasPrefix(b, []byte("%PDF-1.4")) || !bytes.HasSuffix(b, []byte("%%EOF\n")) {
		t.Fatal("bad header/trailer")
	}
	// every xref offset must point at "N 0 obj"
	i := bytes.LastIndex(b, []byte("startxref\n"))
	var xr int
	if _, err := fmtSscan(string(b[i+10:]), &xr); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(b[xr:], []byte("xref\n0 ")) {
		t.Fatal("startxref does not point to xref")
	}
	lines := strings.Split(string(b[xr:]), "\n")
	var n int
	fmtSscan(strings.Fields(lines[1])[1], &n)
	for k := 1; k < n; k++ {
		off, _ := strconv.Atoi(lines[2+k][:10])
		if !bytes.HasPrefix(b[off:], []byte(itoa(k)+" 0 obj")) {
			t.Fatalf("xref entry %d wrong offset %d", k, off)
		}
	}
	if bytes.Count(b, []byte("/Type /Page ")) < 2 {
		t.Fatal("expected multiple pages")
	}
	if os.Getenv("PDF_OUT") != "" {
		os.WriteFile(os.Getenv("PDF_OUT"), b, 0o644)
	}
}

func TestEncodeAndWidth(t *testing.T) {
	if string(encode("a≥b → c")) != "a>=b -> c" {
		t.Fatalf("encode: %q", encode("a≥b → c"))
	}
	if w := TextWidth(Regular, 10, "Hello"); w < 20 || w > 30 {
		t.Fatalf("width %v", w)
	}
	if got := wrap(Regular, 10, "aaa bbb ccc ddd", 40); len(got) < 2 {
		t.Fatalf("wrap %v", got)
	}
}

func itoa(n int) string                      { return strconv.Itoa(n) }
func fmtSscan(s string, a *int) (int, error) { return fmt.Sscan(s, a) }
