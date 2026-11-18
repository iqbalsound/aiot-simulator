// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.

// Package pdf is a tiny, dependency-free PDF 1.4 writer used for lab reports:
// wrapped text, tables, code blocks, vector line/bar charts, page headers and footers.
// Only the standard 14 fonts are used (WinAnsi encoding), so no fonts are embedded.
package pdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"math"
	"strings"
	"time"
)

// Fonts.
const (
	Regular = 1
	Bold    = 2
	Mono    = 3
	Italic  = 4
)

type RGB [3]float64

var (
	Black   = RGB{0.1, 0.1, 0.12}
	Gray    = RGB{0.4, 0.43, 0.47}
	Teal    = RGB{0.05, 0.58, 0.53}
	Navy    = RGB{0.06, 0.16, 0.24}
	Green   = RGB{0.08, 0.5, 0.24}
	Red     = RGB{0.72, 0.11, 0.11}
	Amber   = RGB{0.7, 0.32, 0.04}
	White   = RGB{1, 1, 1}
	Light   = RGB{0.95, 0.96, 0.97}
	Line    = RGB{0.78, 0.82, 0.86}
	Palette = []RGB{{0.05, 0.58, 0.53}, {0.01, 0.41, 0.63}, {0.85, 0.47, 0.02}, {0.72, 0.11, 0.11}, {0.4, 0.3, 0.8}, {0.08, 0.5, 0.24}}
)

// Helvetica widths for ASCII 32..126 (units of 1/1000 em).
var helv = [95]int{278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556,
	1015, 667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778,
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 278, 278, 278, 469, 556,
	333, 556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556,
	556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, 334, 260, 334, 584}

var replacer = strings.NewReplacer("≥", ">=", "≤", "<=", "→", "->", "←", "<-", "✓", "[x]", "✗", "[ ]", "−", "-", "≈", "~",
	"⚠", "!", "●", "*", "■", "#", "▶", ">", "\u00a0", " ", "\t", "    ")

var winansi = map[rune]byte{'€': 0x80, '…': 0x85, '‘': 0x91, '’': 0x92, '“': 0x93, '”': 0x94, '•': 0x95, '–': 0x96, '—': 0x97, '™': 0x99}

func encode(s string) []byte {
	s = replacer.Replace(s)
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r':
			out = append(out, ' ')
		case r >= 32 && r < 127:
			out = append(out, byte(r))
		case r >= 0xA0 && r <= 0xFF:
			out = append(out, byte(r))
		default:
			if b, ok := winansi[r]; ok {
				out = append(out, b)
			} else {
				out = append(out, '?')
			}
		}
	}
	return out
}

func charW(font int, b byte) float64 {
	if font == Mono {
		return 600
	}
	w := 556.0
	if b >= 32 && b < 127 {
		w = float64(helv[b-32])
	}
	if font == Bold {
		w *= 1.07
	}
	return w
}

// TextWidth returns the width of s in points.
func TextWidth(font int, size float64, s string) float64 {
	var w float64
	for _, b := range encode(s) {
		w += charW(font, b)
	}
	return w * size / 1000
}

func esc(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		switch {
		case c == '\\' || c == '(' || c == ')':
			sb.WriteByte('\\')
			sb.WriteByte(c)
		case c < 32 || c > 126:
			fmt.Fprintf(&sb, "\\%03o", c)
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

type page struct{ buf bytes.Buffer }

// Doc is a document under construction. y is measured from the top of the page.
type Doc struct {
	pages                     []*page
	cur                       *page
	W, H, ML, MR, MT, MB, y   float64
	Title, Author, Subject    string
	HeaderL, HeaderR, FooterL string
}

func New() *Doc {
	d := &Doc{W: 595.28, H: 841.89, ML: 44, MR: 44, MT: 58, MB: 54}
	d.AddPage()
	return d
}

func (d *Doc) AddPage() {
	d.cur = &page{}
	d.pages = append(d.pages, d.cur)
	d.y = d.MT
}

// Y returns the current vertical position; Ensure starts a new page when h points do not fit.
func (d *Doc) Y() float64 { return d.y }
func (d *Doc) Ensure(h float64) {
	if d.y+h > d.H-d.MB {
		d.AddPage()
	}
}
func (d *Doc) Space(h float64) { d.y += h }
func (d *Doc) width() float64  { return d.W - d.ML - d.MR }

func (d *Doc) setFill(c RGB)   { fmt.Fprintf(&d.cur.buf, "%.3f %.3f %.3f rg\n", c[0], c[1], c[2]) }
func (d *Doc) setStroke(c RGB) { fmt.Fprintf(&d.cur.buf, "%.3f %.3f %.3f RG\n", c[0], c[1], c[2]) }

// Text draws one line with its baseline at top-based y.
func (d *Doc) Text(x, y float64, font int, size float64, c RGB, s string) {
	d.drawText(d.cur, x, y, font, size, c, s)
}

func (d *Doc) drawText(p *page, x, y float64, font int, size float64, c RGB, s string) {
	fmt.Fprintf(&p.buf, "BT /F%d %.2f Tf %.3f %.3f %.3f rg %.2f %.2f Td (%s) Tj ET\n", font, size, c[0], c[1], c[2], x, d.H-y, esc(encode(s)))
}

// Rect fills (and optionally strokes) a rectangle given by top-based y.
func (d *Doc) Rect(x, y, w, h float64, fill *RGB, stroke *RGB) {
	d.rect(d.cur, x, y, w, h, fill, stroke)
}

func (d *Doc) rect(p *page, x, y, w, h float64, fill *RGB, stroke *RGB) {
	if fill != nil {
		fmt.Fprintf(&p.buf, "%.3f %.3f %.3f rg\n", fill[0], fill[1], fill[2])
	}
	if stroke != nil {
		fmt.Fprintf(&p.buf, "%.3f %.3f %.3f RG 0.5 w\n", stroke[0], stroke[1], stroke[2])
	}
	op := "S"
	switch {
	case fill != nil && stroke != nil:
		op = "B"
	case fill != nil:
		op = "f"
	}
	fmt.Fprintf(&p.buf, "%.2f %.2f %.2f %.2f re %s\n", x, d.H-y-h, w, h, op)
}

func (d *Doc) lineSeg(x1, y1, x2, y2 float64, c RGB, w float64) {
	fmt.Fprintf(&d.cur.buf, "%.3f %.3f %.3f RG %.2f w %.2f %.2f m %.2f %.2f l S\n", c[0], c[1], c[2], w, x1, d.H-y1, x2, d.H-y2)
}

// wrap splits s into lines no wider than maxW.
func wrap(font int, size float64, s string, maxW float64) []string {
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		cur := ""
		flush := func() {
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
		}
		for _, w := range words {
			for TextWidth(font, size, w) > maxW { // break very long tokens
				n := len(w)
				for n > 1 && TextWidth(font, size, w[:n]) > maxW {
					n--
				}
				flush()
				lines = append(lines, w[:n])
				w = w[n:]
			}
			try := w
			if cur != "" {
				try = cur + " " + w
			}
			if TextWidth(font, size, try) <= maxW {
				cur = try
			} else {
				flush()
				cur = w
			}
		}
		flush()
	}
	return lines
}

// Para writes wrapped text at the left margin (indent in points).
func (d *Doc) Para(s string, font int, size float64, c RGB, indent float64) {
	lh := size * 1.38
	for _, ln := range wrap(font, size, s, d.width()-indent) {
		d.Ensure(lh)
		d.y += lh
		d.Text(d.ML+indent, d.y-size*0.28, font, size, c, ln)
	}
}

func (d *Doc) H1(s string) { d.Ensure(34); d.Space(8); d.Para(s, Bold, 17, Navy, 0); d.Space(2) }
func (d *Doc) H2(s string) {
	d.Ensure(40)
	d.Space(8)
	d.Para(s, Bold, 12.5, Teal, 0)
	d.lineSeg(d.ML, d.y+1, d.W-d.MR, d.y+1, Line, 0.6)
	d.Space(4)
}
func (d *Doc) H3(s string) { d.Ensure(28); d.Space(5); d.Para(s, Bold, 10, Navy, 0) }
func (d *Doc) P(s string)  { d.Para(s, Regular, 9.5, Black, 0); d.Space(3) }
func (d *Doc) Bullet(s string) {
	lh := 9.5 * 1.38
	lines := wrap(Regular, 9.5, s, d.width()-14)
	for i, ln := range lines {
		d.Ensure(lh)
		d.y += lh
		if i == 0 {
			d.Text(d.ML+3, d.y-2.7, Regular, 9.5, Teal, "-")
		}
		d.Text(d.ML+14, d.y-2.7, Regular, 9.5, Black, ln)
	}
}

// Code draws a monospaced block on a light background.
func (d *Doc) Code(s string) {
	size := 7.6
	maxChars := int((d.width() - 12) / (0.6 * size))
	var lines []string
	for _, ln := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		ln = strings.ReplaceAll(ln, "\t", "    ")
		for len(ln) > maxChars {
			lines = append(lines, ln[:maxChars])
			ln = "  " + ln[maxChars:]
		}
		lines = append(lines, ln)
	}
	lh := size * 1.35
	d.Space(2)
	for _, ln := range lines {
		d.Ensure(lh)
		d.Rect(d.ML, d.y, d.width(), lh, &Light, nil)
		d.Text(d.ML+6, d.y+lh-2.2, Mono, size, Black, ln)
		d.y += lh
	}
	d.Space(4)
}

// Banner draws a coloured title block.
func (d *Doc) Banner(title, sub string) {
	d.Ensure(64)
	d.Rect(d.ML, d.y, d.width(), 56, &Navy, nil)
	d.Rect(d.ML, d.y, 6, 56, &Teal, nil)
	d.Text(d.ML+18, d.y+24, Bold, 17, White, title)
	d.Text(d.ML+18, d.y+42, Regular, 9.5, RGB{0.72, 0.86, 0.9}, sub)
	d.y += 64
}

func cellColor(s string) RGB {
	switch strings.TrimSpace(s) {
	case "PASS", "Done", "OK", "Complete", "Complete ✓", "[x]":
		return Green
	case "FAIL", "WRONG", "Not done", "BURNED":
		return Red
	case "Pending", "Incomplete", "In progress":
		return Amber
	}
	return Black
}

// Table draws a bordered table with a header row; weights set relative column widths.
func (d *Doc) Table(head []string, rows [][]string, weights []float64) {
	n := len(head)
	if len(weights) != n {
		weights = make([]float64, n)
		for i := range weights {
			weights[i] = 1
		}
	}
	var sum float64
	for _, w := range weights {
		sum += w
	}
	cw := make([]float64, n)
	for i := range cw {
		cw[i] = d.width() * weights[i] / sum
	}
	size, pad := 8.3, 3.2
	lh := size * 1.3
	layout := func(cells []string, font int) ([][]string, float64) {
		out := make([][]string, n)
		maxL := 1
		for i := 0; i < n; i++ {
			t := ""
			if i < len(cells) {
				t = strings.NewReplacer("**", "", "`", "").Replace(cells[i])
			}
			out[i] = wrap(font, size, t, cw[i]-2*pad)
			if len(out[i]) > maxL {
				maxL = len(out[i])
			}
		}
		return out, float64(maxL)*lh + 2*pad
	}
	drawRow := func(cells [][]string, h float64, font int, fill *RGB, color bool) {
		x := d.ML
		for i := 0; i < n; i++ {
			d.Rect(x, d.y, cw[i], h, fill, &Line)
			for k, ln := range cells[i] {
				c := Black
				if font == Bold {
					c = Navy
				} else if color {
					c = cellColor(ln)
				}
				f := font
				if c != Black && c != Navy {
					f = Bold
				}
				d.Text(x+pad, d.y+pad+float64(k+1)*lh-2.2, f, size, c, ln)
			}
			x += cw[i]
		}
		d.y += h
	}
	hc, hh := layout(head, Bold)
	hfill := RGB{0.87, 0.93, 0.95}
	d.Ensure(hh + 14)
	drawRow(hc, hh, Bold, &hfill, false)
	for ri, r := range rows {
		rc, rh := layout(r, Regular)
		if d.y+rh > d.H-d.MB {
			d.AddPage()
			drawRow(hc, hh, Bold, &hfill, false)
		}
		var fill *RGB
		if ri%2 == 1 {
			fill = &Light
		}
		drawRow(rc, rh, Regular, fill, true)
	}
	d.Space(6)
}

// Series is one line in a chart; NaN breaks the line, Marks draws red dots.
type Series struct {
	Name  string
	Y     []float64
	Marks []bool
}

// LineChart draws a vector line chart.
func (d *Doc) LineChart(title, xlabel string, series []Series, h float64) {
	d.Ensure(h + 24)
	d.Space(2)
	d.Text(d.ML, d.y+9, Bold, 9, Navy, title)
	d.y += 14
	px, pw, ph := d.ML+38, d.width()-46, h-34
	top := d.y + 4
	lo, hi, n := math.Inf(1), math.Inf(-1), 0
	for _, s := range series {
		if len(s.Y) > n {
			n = len(s.Y)
		}
		for _, v := range s.Y {
			if !math.IsNaN(v) && !math.IsInf(v, 0) {
				lo, hi = math.Min(lo, v), math.Max(hi, v)
			}
		}
	}
	d.Rect(px, top, pw, ph, nil, &Line)
	if math.IsInf(lo, 1) || n < 2 {
		d.Text(px+8, top+ph/2, Italic, 9, Gray, "no data")
		d.y += h
		return
	}
	if hi == lo {
		hi, lo = hi+1, lo-1
	}
	pad := (hi - lo) * 0.05
	lo, hi = lo-pad, hi+pad
	X := func(i int) float64 { return px + float64(i)*pw/float64(n-1) }
	Y := func(v float64) float64 { return top + (hi-v)*ph/(hi-lo) }
	for k := 0; k <= 4; k++ {
		v := lo + (hi-lo)*float64(k)/4
		y := Y(v)
		if k > 0 && k < 4 {
			d.lineSeg(px, y, px+pw, y, RGB{0.9, 0.92, 0.94}, 0.4)
		}
		lab := fmt.Sprintf("%.1f", v)
		if math.Abs(v) >= 1000 {
			lab = fmt.Sprintf("%.0f", v)
		}
		d.Text(px-TextWidth(Regular, 6.5, lab)-3, y+2.3, Regular, 6.5, Gray, lab)
	}
	for si, s := range series {
		col := Palette[si%len(Palette)]
		pen := false
		for i, v := range s.Y {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				pen = false
				continue
			}
			if !pen {
				fmt.Fprintf(&d.cur.buf, "%.3f %.3f %.3f RG 0.9 w %.2f %.2f m\n", col[0], col[1], col[2], X(i), d.H-Y(v))
				pen = true
			} else {
				fmt.Fprintf(&d.cur.buf, "%.2f %.2f l\n", X(i), d.H-Y(v))
			}
			if i == len(s.Y)-1 || math.IsNaN(s.Y[i+1]) {
				d.cur.buf.WriteString("S\n")
				pen = false
			}
		}
		for i, m := range s.Marks {
			if m && i < len(s.Y) && !math.IsNaN(s.Y[i]) {
				d.Rect(X(i)-2, Y(s.Y[i])-2, 4, 4, &Red, nil)
			}
		}
	}
	lx := px
	ly := top + ph + 12
	for si, s := range series {
		col := Palette[si%len(Palette)]
		d.Rect(lx, ly-6, 8, 6, &col, nil)
		d.Text(lx+11, ly, Regular, 7, Black, s.Name)
		lx += 20 + TextWidth(Regular, 7, s.Name)
	}
	if xlabel != "" {
		d.Text(px+pw-TextWidth(Italic, 7, xlabel), ly, Italic, 7, Gray, xlabel)
	}
	d.y += h
}

// BarChart draws vertical bars with value labels.
func (d *Doc) BarChart(title string, labels []string, values []float64, h float64) {
	d.Ensure(h + 24)
	d.Space(2)
	d.Text(d.ML, d.y+9, Bold, 9, Navy, title)
	d.y += 14
	px, pw, ph := d.ML+8, d.width()-16, h-40
	top := d.y + 8
	mx := 1e-9
	for _, v := range values {
		mx = math.Max(mx, v)
	}
	d.Rect(px, top, pw, ph, nil, &Line)
	bw := pw / float64(len(values))
	for i, v := range values {
		bh := ph * v / mx
		col := Palette[i%len(Palette)]
		d.Rect(px+float64(i)*bw+bw*0.18, top+ph-bh, bw*0.64, bh, &col, nil)
		val := fmt.Sprintf("%.2f", v)
		d.Text(px+float64(i)*bw+bw/2-TextWidth(Regular, 7, val)/2, top+ph-bh-3, Regular, 7, Black, val)
		l := labels[i]
		d.Text(px+float64(i)*bw+bw/2-TextWidth(Regular, 7, l)/2, top+ph+11, Regular, 7, Gray, l)
	}
	d.y += h
}

// Ruled draws n writing lines (for handwritten answers).
func (d *Doc) Ruled(n int) {
	for i := 0; i < n; i++ {
		d.Ensure(18)
		d.y += 18
		d.lineSeg(d.ML, d.y, d.W-d.MR, d.y, Line, 0.5)
	}
	d.Space(4)
}

// Bytes finishes the document (adds headers/footers) and returns the PDF file.
func (d *Doc) Bytes() []byte {
	total := len(d.pages)
	for i, p := range d.pages {
		d.cur = p
		if d.HeaderL != "" || d.HeaderR != "" {
			d.drawText(p, d.ML, 32, Bold, 8.5, Teal, d.HeaderL)
			d.drawText(p, d.W-d.MR-TextWidth(Regular, 8, d.HeaderR), 32, Regular, 8, Gray, d.HeaderR)
			d.lineSeg(d.ML, 38, d.W-d.MR, 38, Line, 0.6)
		}
		d.lineSeg(d.ML, d.H-40, d.W-d.MR, d.H-40, Line, 0.6)
		d.drawText(p, d.ML, d.H-28, Regular, 7.5, Gray, d.FooterL)
		pg := fmt.Sprintf("Page %d of %d", i+1, total)
		d.drawText(p, d.W-d.MR-TextWidth(Regular, 7.5, pg), d.H-28, Regular, 7.5, Gray, pg)
	}
	var out bytes.Buffer
	offs := []int{}
	obj := func(body string) int {
		offs = append(offs, out.Len())
		n := len(offs)
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", n, body)
		return n
	}
	out.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	obj("<< /Type /Catalog /Pages 2 0 R >>") // 1
	kids := make([]string, total)
	for i := range kids {
		kids[i] = fmt.Sprintf("%d 0 R", 7+2*i)
	}
	obj(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), total)) // 2
	for _, f := range []string{"Helvetica", "Helvetica-Bold", "Courier", "Helvetica-Oblique"} { // 3..6
		obj(fmt.Sprintf("<< /Type /Font /Subtype /Type1 /BaseFont /%s /Encoding /WinAnsiEncoding >>", f))
	}
	for _, p := range d.pages {
		var zb bytes.Buffer
		zw := zlib.NewWriter(&zb)
		zw.Write(p.buf.Bytes())
		zw.Close()
		// page object number = next, content = next+1
		obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Resources << /Font << /F1 3 0 R /F2 4 0 R /F3 5 0 R /F4 6 0 R >> >> /Contents %d 0 R >>", d.W, d.H, len(offs)+2))
		offs = append(offs, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n<< /Length %d /Filter /FlateDecode >>\nstream\n", len(offs), zb.Len())
		out.Write(zb.Bytes())
		out.WriteString("\nendstream\nendobj\n")
	}
	now := time.Now().Format("20060102150405")
	info := obj(fmt.Sprintf("<< /Title (%s) /Author (%s) /Subject (%s) /Creator (AIoT Simulator) /Producer (AIoT Simulator PDF writer) /CreationDate (D:%s) >>",
		esc(encode(d.Title)), esc(encode(d.Author)), esc(encode(d.Subject)), now))
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offs)+1)
	for _, o := range offs {
		fmt.Fprintf(&out, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R /Info %d 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offs)+1, info, xref)
	return out.Bytes()
}
