// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"aiotsim/internal/pdf"
	"aiotsim/internal/sim"
)

type chartSpec struct {
	Kind   string // line | bar
	Title  string
	XLabel string
	Series []pdf.Series
	Labels []string
	Values []float64
}

func (a *App) setCharts(lab string, cs ...chartSpec) {
	a.mu.Lock()
	a.charts[lab] = cs
	a.mu.Unlock()
}

func tail(v []float64, n int) []float64 {
	if len(v) > n {
		return v[len(v)-n:]
	}
	return v
}

var labCLO = map[int]string{
	1: "Lab CLO-1 (GA-2, P3)", 2: "Lab CLO-1 (GA-2, P3)", 3: "Lab CLO-1 (GA-2, P3)",
	4: "Lab CLO-1, CLO-2 (GA-2, GA-4)", 5: "Lab CLO-2 (GA-4, P4)", 6: "Lab CLO-1, CLO-2 (GA-2, GA-4)",
	7: "Lab CLO-1 (GA-2, P3)", 8: "Lab CLO-1, CLO-2 (GA-2, GA-4)", 9: "Lab CLO-1 (GA-2, P3)",
	10: "Lab CLO-2 (GA-4, P4)", 11: "Lab CLO-2 (GA-4, P4)", 12: "Lab CLO-2 (GA-4, P4)",
	13: "Lab CLO-1, CLO-2, CLO-3 (GA-2, GA-4, GA-6)", 14: "Lab CLO-1, CLO-2, CLO-3 (GA-2, GA-4, GA-6)",
	15: "Lab CLO-3 (GA-6, A2)", 16: "Lab CLO-3 (GA-6, A2)",
}

var labReflect = map[int]string{
	1:  "Which kit item or setup step caused the most difficulty, and how did you resolve it?",
	2:  "Explain how the LDR voltage divider turns light into an ADC value. What happens when a sensor is wired to the wrong pin?",
	3:  "Which rows needed cleaning and why? Compare a rolling mean with a median filter.",
	4:  "Describe how the parse function turns one serial line into dashboard values.",
	5:  "Why must a production MQTT broker require authentication? Compare QoS 0, 1 and 2.",
	6:  "Which algorithm performed better on your data and what is your hypothesis for the difference?",
	7:  "Explain why quantisation suits microcontrollers and what you trade off.",
	8:  "What was the hardest part of the practical and what would you do differently?",
	9:  "What limits a model on a microcontroller (RAM, flash, speed)? What did the tensor-arena experiment show?",
	10: "How does the device access token authenticate telemetry? Explain your alarm rule.",
	11: "Justify your contamination value using the false-positive versus missed-anomaly trade-off.",
	12: "Why is recall the key metric in predictive maintenance? Did your extra feature help?",
	13: "State the main technical risk of your project and its mitigation.",
	14: "Which scenarios failed first and what design change fixed them?",
	15: "Which stage dominated latency and what design change would reduce it?",
	16: "What did you learn from the whole course and what would you improve in your project?",
}

func (a *App) routesPDF(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/report.pdf", func(w http.ResponseWriter, r *http.Request) {
		if !a.hostOK(w, r) {
			return
		}
		st := a.getStudent()
		if err := validStudent(st); err != nil {
			http.Error(w, "Enter the student name and registration number first (Student details).", http.StatusBadRequest)
			return
		}
		var labs []int
		if q := r.URL.Query().Get("lab"); q == "" || q == "all" {
			for i := 1; i <= 16; i++ {
				labs = append(labs, i)
			}
		} else if n, err := strconv.Atoi(q); err == nil && n >= 1 && n <= 16 {
			labs = []int{n}
		} else {
			http.Error(w, "lab must be 1-16 or all", http.StatusBadRequest)
			return
		}
		b, name := a.buildPDF(labs, st)
		a.markQuiet("report_export")
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		w.Write(b)
	})
}

var safeRe = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

func (a *App) buildPDF(labs []int, st Student) ([]byte, string) {
	now := time.Now()
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%v|%d", st.Name, st.Reg, labs, now.UnixNano())
	a.mu.Lock()
	keys := make([]string, 0, len(a.ev))
	for k := range a.ev {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(h, "|%s=%s", k, a.ev[k])
	}
	a.mu.Unlock()
	id := strings.ToUpper(hex.EncodeToString(h.Sum(nil))[:12])
	single := len(labs) == 1

	d := pdf.New()
	d.Author = st.Name
	d.Title = fmt.Sprintf("%s Lab Report - %s (%s)", CourseCode, st.Name, st.Reg)
	d.Subject = CourseCode + " " + CourseName
	d.HeaderL = CourseCode + " " + CourseName + " - Simulator Lab Report"
	d.HeaderR = "Course teacher: " + Author
	d.FooterL = fmt.Sprintf("Student: %s | Reg. No.: %s | Report ID %s", st.Name, st.Reg, id)

	title, sub := CourseCode+" Lab Portfolio", "Artificial Intelligence of Things - all laboratories"
	if single {
		title, sub = fmt.Sprintf("Lab %d Report", labs[0]), labDefs[labs[0]].Title
	}
	d.Banner(title, sub)
	info := [][]string{
		{"Student name", st.Name}, {"Registration number", st.Reg},
	}
	if st.Section != "" {
		info = append(info, []string{"Section", st.Section})
	}
	info = append(info, []string{"Course", CourseCode + " " + CourseName},
		[]string{"Course teacher", fmt.Sprintf("%s, %s, %s", Author, AuthorTitle, AuthorDept)},
		[]string{"Generated", now.Format("02 Jan 2006, 15:04")},
		[]string{"Simulator version", a.Version},
		[]string{"Report ID", id})
	d.Table([]string{"Field", "Details"}, info, []float64{1, 3})
	d.P("Evidence in this report was produced by the AIoT Simulator. The laboratory manual also requires supervised work on real hardware; this report does not replace it.")

	if !single {
		d.H2("Completion summary")
		var rows [][]string
		a.mu.Lock()
		for n := 1; n <= 16; n++ {
			def := labDefs[n]
			cnt := func(items []item) string {
				if len(items) == 0 {
					return "-"
				}
				c := 0
				for _, it := range items {
					if a.prog[it.Key] {
						c++
					}
				}
				return fmt.Sprintf("%d/%d", c, len(items))
			}
			st := "Pending"
			all := len(def.Req) > 0
			for _, it := range def.Req {
				if !a.prog[it.Key] {
					all = false
				}
			}
			if all {
				st = "Complete"
			}
			rows = append(rows, []string{strconv.Itoa(n), def.Title, cnt(def.Req), cnt(def.Task), st})
		}
		a.mu.Unlock()
		d.Table([]string{"Lab", "Title", "Required", "Task", "Status"}, rows, []float64{0.6, 5, 1.2, 1.1, 1.4})
	}
	for _, n := range labs {
		if !single {
			d.AddPage()
		}
		a.pdfLab(d, n)
	}
	name := "AI-4301_" + map[bool]string{true: fmt.Sprintf("Lab%02d", labs[0]), false: "All-Labs"}[single] + "_" + safeRe.ReplaceAllString(st.Reg, "-") + ".pdf"
	return d.Bytes(), name
}

func (a *App) pdfLab(d *pdf.Doc, n int) {
	def := labDefs[n]
	d.H1(fmt.Sprintf("Lab %d: %s", n, def.Title))
	d.Para(labCLO[n], pdf.Italic, 9, pdf.Gray, 0)
	d.Space(4)

	a.mu.Lock()
	row := func(it item, prefix string) []string {
		st, ev := "Not done", a.ev[it.Key]
		if a.prog[it.Key] {
			st = "Done"
		}
		if ev == "ticked by student" {
			ev = "confirmed by student"
		}
		return []string{prefix + it.Label, st, ev}
	}
	var rows [][]string
	reqDone := 0
	for _, it := range def.Req {
		rows = append(rows, row(it, ""))
		if a.prog[it.Key] {
			reqDone++
		}
	}
	for _, it := range def.Task {
		rows = append(rows, row(it, "Task: "))
	}
	md, _ := a.hist[fmt.Sprintf("lab%02d", n)].(string)
	charts := append([]chartSpec(nil), a.charts[fmt.Sprintf("lab%02d", n)]...)
	a.mu.Unlock()

	status := "Incomplete"
	if reqDone == len(def.Req) {
		status = "Complete"
	}
	d.H2(fmt.Sprintf("Outcomes - %s (%d of %d required)", status, reqDone, len(def.Req)))
	d.Table([]string{"Outcome", "Status", "Evidence recorded by the simulator"}, rows, []float64{4, 1, 4})

	a.pdfExtras(d, n)
	if md != "" {
		d.H2("Results")
		renderMD(d, md)
	}
	for _, c := range charts {
		if c.Kind == "bar" {
			d.BarChart(c.Title, c.Labels, c.Values, 130)
		} else {
			d.LineChart(c.Title, c.XLabel, c.Series, 150)
		}
	}
	d.H2("Student reflection")
	d.P(labReflect[n])
	d.Ruled(6)
	d.Ensure(70)
	d.H3("Instructor use only")
	d.Table([]string{"Marks awarded", "Remarks", "Signature and date"}, [][]string{{"   /\n\n ", " \n\n ", " \n\n "}}, []float64{1.2, 3, 2})
}

var mdClean = strings.NewReplacer("**", "", "`", "")

// renderMD draws the small Markdown subset the lab handlers store (headings, bullets, tables, code).
func renderMD(d *pdf.Doc, md string) {
	lines := strings.Split(md, "\n")
	for i := 0; i < len(lines); i++ {
		ln := strings.TrimRight(lines[i], " \r")
		switch {
		case strings.HasPrefix(ln, "```"):
			var code []string
			for i++; i < len(lines) && !strings.HasPrefix(lines[i], "```"); i++ {
				code = append(code, lines[i])
			}
			d.Code(strings.Join(code, "\n"))
		case strings.HasPrefix(ln, "|"):
			var tbl [][]string
			for ; i < len(lines) && strings.HasPrefix(lines[i], "|"); i++ {
				if strings.Contains(lines[i], "---") {
					continue
				}
				cells := strings.Split(strings.Trim(strings.TrimSpace(lines[i]), "|"), "|")
				for k := range cells {
					cells[k] = strings.TrimSpace(mdClean.Replace(cells[k]))
				}
				tbl = append(tbl, cells)
			}
			i--
			if len(tbl) > 0 {
				d.Table(tbl[0], tbl[1:], nil)
			}
		case strings.HasPrefix(ln, "## "):
			if !strings.HasPrefix(ln, "## Lab") {
				d.H3(strings.TrimPrefix(ln, "## "))
			}
		case strings.HasPrefix(ln, "### "):
			d.H3(strings.TrimPrefix(ln, "### "))
		case strings.HasPrefix(ln, "- "):
			d.Bullet(mdClean.Replace(strings.TrimPrefix(ln, "- ")))
		case strings.TrimSpace(ln) == "":
		default:
			d.P(mdClean.Replace(ln))
		}
	}
}

func (a *App) serialExcerpt(d *pdf.Doc) {
	a.mu.Lock()
	t := a.serial
	if len(t) > 12 {
		t = t[len(t)-12:]
	}
	var ls []string
	for _, e := range t {
		ls = append(ls, e.Line)
	}
	a.mu.Unlock()
	d.H3("Serial Monitor excerpt (latest lines)")
	if len(ls) == 0 {
		d.P("No serial output captured in this session.")
		return
	}
	d.Code(strings.Join(ls, "\n"))
}

func (a *App) pdfExtras(d *pdf.Doc, n int) {
	switch n {
	case 2:
		var rows [][]string
		for _, c := range sim.WiringCheck(a.S.GetWiring()) {
			act := fmt.Sprint(c["actual"])
			if act == "" {
				act = "-"
			}
			rows = append(rows, []string{fmt.Sprint(c["sensor"]), fmt.Sprint(c["expected"]), act, fmt.Sprint(c["status"])})
		}
		bs := a.B.State()
		d.H2("Wiring and sketch")
		d.Table([]string{"Sensor pin", "Expected", "Actual", "Status"}, rows, []float64{3, 1.5, 1.5, 1.3})
		d.P(fmt.Sprintf("Uploaded sketch: %s, Serial.begin(%d).", bs.Sketch, bs.Baud))
		a.serialExcerpt(d)
	case 3:
		a.mu.Lock()
		rows, fields, src := a.rows, a.logFields, a.dataSource
		a.mu.Unlock()
		d.H2("Dataset")
		if len(rows) == 0 {
			d.P("No dataset logged yet.")
			return
		}
		d.P(fmt.Sprintf("%d rows logged (%s).", len(rows), src))
		names := []string{"temperature", "humidity", "motion", "light", "distance"}
		var tr [][]string
		for i := 0; i < fields; i++ {
			c := describe(names[i], rows, i)
			tr = append(tr, []string{c.Name, c.Dtype, strconv.Itoa(c.NonNull), fx(c.Mean), fx(c.Std), fx(c.Min), fx(c.Max)})
		}
		d.Table([]string{"column", "dtype", "non-null", "mean", "std", "min", "max"}, tr, nil)
		head := rows
		if len(head) > 8 {
			head = head[:8]
		}
		d.H3("First rows of sensor_log.csv")
		d.Code(a.csvText(head, fields))
	case 4:
		a.serialExcerpt(d)
	case 5:
		ms := a.mqttState()
		br, _ := ms["bridge"].(map[string]any)
		d.H2("Broker and publisher")
		d.Table([]string{"Setting", "Value"}, [][]string{
			{"Port", fmt.Sprint(ms["port"])}, {"Listening on LAN", fmt.Sprint(ms["lan"])},
			{"allow_anonymous", fmt.Sprint(ms["allowAnonymous"])}, {"Users defined", fmt.Sprint(len(ms["users"].([]string)))},
			{"Publisher batches sent", fmt.Sprint(br["sent"])}, {"Publisher interval (s)", fmt.Sprint(br["interval"])},
		}, []float64{2, 3})
		a.mu.Lock()
		ref := a.ev["mqtt_refused"]
		var tv []*topicVal
		for _, t := range a.topics {
			tv = append(tv, t)
		}
		a.mu.Unlock()
		if ref != "" {
			d.P("Last refused connection: " + ref)
		}
		sort.Slice(tv, func(i, j int) bool { return tv[i].Topic < tv[j].Topic })
		if len(tv) > 14 {
			tv = tv[:14]
		}
		var rows [][]string
		for _, t := range tv {
			rows = append(rows, []string{t.Topic, t.Payload, strconv.Itoa(t.Count)})
		}
		if len(rows) > 0 {
			d.H3("Topic tree (MQTT Explorer view)")
			d.Table([]string{"Topic", "Last value", "Messages"}, rows, []float64{3, 3, 1})
		}
	case 9:
		a.mu.Lock()
		m := a.mcu
		a.mu.Unlock()
		if m != nil {
			d.H2("model_data.h")
			ls := strings.Split(m.Header, "\n")
			if len(ls) > 7 {
				ls = append(ls[:6], "  ...", "};", fmt.Sprintf("unsigned int %s_len = %d;", m.Name, m.Len))
			}
			d.Code(strings.Join(ls, "\n"))
		}
	case 10:
		a.mu.Lock()
		var dr [][]string
		for _, dv := range a.devices {
			tok := dv.Token
			if len(tok) > 4 {
				tok = tok[:4] + strings.Repeat("*", len(tok)-4)
			}
			dr = append(dr, []string{dv.Name, tok, strconv.Itoa(dv.Count)})
		}
		var rr [][]string
		for _, r := range a.rules {
			rr = append(rr, []string{r.Device, fmt.Sprintf("%s %s %g", r.Key, r.Op, r.Threshold)})
		}
		al := a.alarms
		if len(al) > 12 {
			al = al[len(al)-12:]
		}
		var ar [][]string
		for _, x := range al {
			ar = append(ar, []string{time.UnixMilli(x.TS).Format("15:04:05"), x.Device, x.Rule, fmt.Sprintf("%.2f", x.Value), x.Status})
		}
		a.mu.Unlock()
		d.H2("Platform state")
		if len(dr) > 0 {
			d.Table([]string{"Device", "Access token (masked)", "Telemetry messages"}, dr, nil)
		}
		if len(rr) > 0 {
			d.H3("Alarm rules")
			d.Table([]string{"Device", "Rule"}, rr, nil)
		}
		if len(ar) > 0 {
			d.H3("Alarms raised")
			d.Table([]string{"Time", "Device", "Rule", "Value", "Status"}, ar, []float64{1, 1.5, 2, 1, 1.5})
		}
	case 13:
		a.mu.Lock()
		p := a.proj
		a.mu.Unlock()
		if p != nil {
			d.H2("Selected project")
			d.P(p.Name)
			var rows [][]string
			for _, l := range architecture(p.Key) {
				rows = append(rows, []string{fmt.Sprint(l["layer"]), fmt.Sprint(l["items"])})
			}
			d.Table([]string{"Layer", "Components"}, rows, []float64{1, 4})
		}
	}
}

func fx(v any) string {
	if f, ok := v.(float64); ok {
		return strconv.FormatFloat(f, 'f', 2, 64)
	}
	return "-"
}
