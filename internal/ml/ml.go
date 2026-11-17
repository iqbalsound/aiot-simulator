// Package ml contains the small machine-learning toolkit behind Labs 6, 7, 11 and 12:
// decision tree, random forest, k-NN, a tiny neural network with int8 quantisation,
// Isolation Forest, evaluation metrics and rolling-window feature helpers.
// Everything is pure Go so the simulator ships as a single binary.
package ml

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

// ---------------------------------------------------------------- split & metrics

// Split shuffles (seeded) and splits into train and test sets, like train_test_split.
func Split(X [][]float64, y []int, testFrac float64, seed int64) (Xtr, Xte [][]float64, ytr, yte []int) {
	idx := rand.New(rand.NewSource(seed)).Perm(len(X))
	nTest := int(math.Ceil(float64(len(X)) * testFrac))
	for i, j := range idx {
		if i < nTest {
			Xte, yte = append(Xte, X[j]), append(yte, y[j])
		} else {
			Xtr, ytr = append(Xtr, X[j]), append(ytr, y[j])
		}
	}
	return
}

type ClassStat struct {
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	F1        float64 `json:"f1"`
	Support   int     `json:"support"`
}

// Report mirrors sklearn's accuracy / confusion_matrix / classification_report.
type Report struct {
	Accuracy  float64      `json:"accuracy"`
	Confusion [2][2]int    `json:"confusion"` // [true][pred]
	Class     [2]ClassStat `json:"class"`
	Precision float64      `json:"precision"` // positive class (1)
	Recall    float64      `json:"recall"`
	F1        float64      `json:"f1"`
}

func div(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func Evaluate(yTrue, yPred []int) Report {
	var r Report
	for i := range yTrue {
		r.Confusion[yTrue[i]][yPred[i]]++
	}
	n := float64(len(yTrue))
	r.Accuracy = div(float64(r.Confusion[0][0]+r.Confusion[1][1]), n)
	for c := 0; c < 2; c++ {
		tp := float64(r.Confusion[c][c])
		fp := float64(r.Confusion[1-c][c])
		fn := float64(r.Confusion[c][1-c])
		p, rc := div(tp, tp+fp), div(tp, tp+fn)
		r.Class[c] = ClassStat{p, rc, div(2*p*rc, p+rc), r.Confusion[c][0] + r.Confusion[c][1]}
	}
	r.Precision, r.Recall, r.F1 = r.Class[1].Precision, r.Class[1].Recall, r.Class[1].F1
	return r
}

// ---------------------------------------------------------------- decision tree

type node struct {
	feat        int
	thr         float64
	left, right *node
	p1          float64 // probability of class 1 at a leaf
	n           int
}

type Tree struct {
	root       *node
	NFeat      int
	importance []float64
}

type TreeOpts struct {
	MaxDepth int // 0 = unlimited
	MinSplit int
	Mtry     int // features tried per split (0 = all)
	Rng      *rand.Rand
}

func gini(n0, n1 float64) float64 {
	t := n0 + n1
	if t == 0 {
		return 0
	}
	p := n1 / t
	return 2 * p * (1 - p)
}

// FitTree trains a CART classifier on the given sample indices.
func FitTree(X [][]float64, y []int, idx []int, o TreeOpts) *Tree {
	if o.MinSplit < 2 {
		o.MinSplit = 2
	}
	nf := len(X[0])
	t := &Tree{NFeat: nf, importance: make([]float64, nf)}
	var build func(ix []int, depth int) *node
	build = func(ix []int, depth int) *node {
		var n1 float64
		for _, i := range ix {
			n1 += float64(y[i])
		}
		n := float64(len(ix))
		nd := &node{p1: n1 / n, n: len(ix), feat: -1}
		if n1 == 0 || n1 == n || len(ix) < o.MinSplit || (o.MaxDepth > 0 && depth >= o.MaxDepth) {
			return nd
		}
		feats := make([]int, nf)
		for i := range feats {
			feats[i] = i
		}
		if o.Mtry > 0 && o.Mtry < nf && o.Rng != nil {
			o.Rng.Shuffle(nf, func(a, b int) { feats[a], feats[b] = feats[b], feats[a] })
			feats = feats[:o.Mtry]
		}
		parent := gini(n-n1, n1)
		bestGain, bestF, bestT := 0.0, -1, 0.0
		for _, f := range feats {
			s := append([]int(nil), ix...)
			sort.Slice(s, func(a, b int) bool { return X[s[a]][f] < X[s[b]][f] })
			var l0, l1 float64
			for k := 0; k < len(s)-1; k++ {
				if y[s[k]] == 1 {
					l1++
				} else {
					l0++
				}
				if X[s[k]][f] == X[s[k+1]][f] {
					continue
				}
				r0, r1 := (n-n1)-l0, n1-l1
				g := parent - ((l0+l1)/n)*gini(l0, l1) - ((r0+r1)/n)*gini(r0, r1)
				if g > bestGain+1e-12 {
					bestGain, bestF, bestT = g, f, (X[s[k]][f]+X[s[k+1]][f])/2
				}
			}
		}
		if bestF < 0 {
			return nd
		}
		var li, ri []int
		for _, i := range ix {
			if X[i][bestF] <= bestT {
				li = append(li, i)
			} else {
				ri = append(ri, i)
			}
		}
		t.importance[bestF] += bestGain * n
		nd.feat, nd.thr = bestF, bestT
		nd.left, nd.right = build(li, depth+1), build(ri, depth+1)
		return nd
	}
	t.root = build(idx, 0)
	return t
}

func (t *Tree) Proba(x []float64) float64 {
	n := t.root
	for n.feat >= 0 {
		if x[n.feat] <= n.thr {
			n = n.left
		} else {
			n = n.right
		}
	}
	return n.p1
}

func (t *Tree) Predict(x []float64) int {
	if t.Proba(x) > 0.5 {
		return 1
	}
	return 0
}

// Importances returns normalised impurity-decrease importances (sum = 1).
func (t *Tree) Importances() []float64 { return normalise(t.importance) }

// Depth and Leaves describe the fitted tree.
func (t *Tree) Depth() int  { return depth(t.root) }
func (t *Tree) Leaves() int { return leaves(t.root) }
func depth(n *node) int {
	if n == nil || n.feat < 0 {
		return 0
	}
	return 1 + int(math.Max(float64(depth(n.left)), float64(depth(n.right))))
}
func leaves(n *node) int {
	if n == nil {
		return 0
	}
	if n.feat < 0 {
		return 1
	}
	return leaves(n.left) + leaves(n.right)
}

func normalise(v []float64) []float64 {
	out := make([]float64, len(v))
	var s float64
	for _, x := range v {
		s += x
	}
	for i, x := range v {
		out[i] = div(x, s)
	}
	return out
}

// ---------------------------------------------------------------- random forest

type Forest struct {
	Trees []*Tree
	nfeat int
}

func FitForest(X [][]float64, y []int, nTrees, maxDepth int, seed int64) *Forest {
	rng := rand.New(rand.NewSource(seed))
	nf := len(X[0])
	mtry := int(math.Max(1, math.Round(math.Sqrt(float64(nf)))))
	f := &Forest{nfeat: nf}
	for i := 0; i < nTrees; i++ {
		idx := make([]int, len(X))
		for j := range idx {
			idx[j] = rng.Intn(len(X))
		}
		f.Trees = append(f.Trees, FitTree(X, y, idx, TreeOpts{MaxDepth: maxDepth, Mtry: mtry, Rng: rng}))
	}
	return f
}

func (f *Forest) Proba(x []float64) float64 {
	var s float64
	for _, t := range f.Trees {
		s += t.Proba(x)
	}
	return s / float64(len(f.Trees))
}

func (f *Forest) Predict(x []float64) int {
	if f.Proba(x) > 0.5 {
		return 1
	}
	return 0
}

func (f *Forest) Importances() []float64 {
	acc := make([]float64, f.nfeat)
	for _, t := range f.Trees {
		for i, v := range t.Importances() {
			acc[i] += v
		}
	}
	return normalise(acc)
}

// ---------------------------------------------------------------- k-NN

type KNN struct {
	X           [][]float64
	y           []int
	K           int
	mean, std   []float64
	Standardise bool
}

func FitKNN(X [][]float64, y []int, k int, standardise bool) *KNN {
	m := &KNN{K: k, y: y, Standardise: standardise}
	m.mean, m.std = meanStd(X)
	m.X = X
	if standardise {
		m.X = scale(X, m.mean, m.std)
	}
	return m
}

func (m *KNN) Predict(x []float64) int {
	if m.Standardise {
		x = scale([][]float64{x}, m.mean, m.std)[0]
	}
	type nb struct {
		d float64
		y int
	}
	ns := make([]nb, len(m.X))
	for i, r := range m.X {
		var d float64
		for j := range r {
			d += (r[j] - x[j]) * (r[j] - x[j])
		}
		ns[i] = nb{d, m.y[i]}
	}
	sort.Slice(ns, func(a, b int) bool { return ns[a].d < ns[b].d })
	k := m.K
	if k > len(ns) {
		k = len(ns)
	}
	var v1 int
	for i := 0; i < k; i++ {
		v1 += ns[i].y
	}
	if v1*2 > k {
		return 1
	}
	return 0
}

func meanStd(X [][]float64) (mean, std []float64) {
	nf := len(X[0])
	mean, std = make([]float64, nf), make([]float64, nf)
	for _, r := range X {
		for j, v := range r {
			mean[j] += v
		}
	}
	for j := range mean {
		mean[j] /= float64(len(X))
	}
	for _, r := range X {
		for j, v := range r {
			std[j] += (v - mean[j]) * (v - mean[j])
		}
	}
	for j := range std {
		std[j] = math.Sqrt(std[j] / float64(len(X)))
		if std[j] < 1e-9 {
			std[j] = 1
		}
	}
	return
}

func scale(X [][]float64, mean, std []float64) [][]float64 {
	out := make([][]float64, len(X))
	for i, r := range X {
		out[i] = make([]float64, len(r))
		for j, v := range r {
			out[i][j] = (v - mean[j]) / std[j]
		}
	}
	return out
}

// ---------------------------------------------------------------- neural network

// NN is the manual's Lab 7 model: Input(n) -> Dense(h, relu) -> Dense(1, sigmoid).
type NN struct {
	In, H      int
	W1         [][]float64 // In x H
	B1         []float64
	W2         []float64 // H
	B2         float64
	Mean, Std  []float64
	Normalise  bool
	QuantisedW bool
}

type EpochLog struct {
	Epoch int     `json:"epoch"`
	Loss  float64 `json:"loss"`
	Acc   float64 `json:"accuracy"`
	VLoss float64 `json:"valLoss"`
	VAcc  float64 `json:"valAccuracy"`
}

func sigmoid(z float64) float64 { return 1 / (1 + math.Exp(-z)) }

func (m *NN) Params() int { return m.In*m.H + m.H + m.H + 1 }
func (m *NN) MACs() int   { return m.In*m.H + m.H }

func (m *NN) prep(x []float64) []float64 {
	if !m.Normalise {
		return x
	}
	o := make([]float64, len(x))
	for j := range x {
		o[j] = (x[j] - m.Mean[j]) / m.Std[j]
	}
	return o
}

func (m *NN) forward(x []float64) (h []float64, out float64) {
	h = make([]float64, m.H)
	for k := 0; k < m.H; k++ {
		z := m.B1[k]
		for j := 0; j < m.In; j++ {
			z += x[j] * m.W1[j][k]
		}
		if z > 0 {
			h[k] = z
		}
	}
	z := m.B2
	for k := 0; k < m.H; k++ {
		z += h[k] * m.W2[k]
	}
	return h, sigmoid(z)
}

func (m *NN) Proba(x []float64) float64 { _, o := m.forward(m.prep(x)); return o }
func (m *NN) Predict(x []float64) int {
	if m.Proba(x) >= 0.5 {
		return 1
	}
	return 0
}

func (m *NN) lossAcc(X [][]float64, y []int) (float64, float64) {
	if len(X) == 0 {
		return 0, 0
	}
	var l, a float64
	for i, x := range X {
		p := math.Min(math.Max(m.Proba(x), 1e-7), 1-1e-7)
		if y[i] == 1 {
			l -= math.Log(p)
		} else {
			l -= math.Log(1 - p)
		}
		if (p >= 0.5) == (y[i] == 1) {
			a++
		}
	}
	return l / float64(len(X)), a / float64(len(X))
}

// TrainNN trains with Adam (batch 32), using the last 20% of the training data
// as validation (like Keras validation_split=0.2).
func TrainNN(X [][]float64, y []int, hidden, epochs int, lr float64, normalise bool, seed int64) (*NN, []EpochLog) {
	rng := rand.New(rand.NewSource(seed))
	in := len(X[0])
	m := &NN{In: in, H: hidden, Normalise: normalise}
	m.Mean, m.Std = meanStd(X)
	lim := math.Sqrt(6.0 / float64(in+hidden))
	m.W1 = make([][]float64, in)
	for j := range m.W1 {
		m.W1[j] = make([]float64, hidden)
		for k := range m.W1[j] {
			m.W1[j][k] = (rng.Float64()*2 - 1) * lim
		}
	}
	m.B1, m.W2 = make([]float64, hidden), make([]float64, hidden)
	lim2 := math.Sqrt(6.0 / float64(hidden+1))
	for k := range m.W2 {
		m.W2[k] = (rng.Float64()*2 - 1) * lim2
	}
	nVal := len(X) / 5
	Xt, yt := X[:len(X)-nVal], y[:len(y)-nVal]
	Xv, yv := X[len(X)-nVal:], y[len(y)-nVal:]
	// Adam state, flattened params: W1, B1, W2, B2
	np := m.Params()
	mo, ve := make([]float64, np), make([]float64, np)
	step := 0
	var logs []EpochLog
	order := rng.Perm(len(Xt))
	for ep := 1; ep <= epochs; ep++ {
		rng.Shuffle(len(order), func(a, b int) { order[a], order[b] = order[b], order[a] })
		for s := 0; s < len(order); s += 32 {
			e := s + 32
			if e > len(order) {
				e = len(order)
			}
			g := make([]float64, np)
			for _, oi := range order[s:e] {
				x := m.prep(Xt[oi])
				h, out := m.forward(x)
				d := out - float64(yt[oi]) // dL/dz2 for BCE+sigmoid
				off := m.In*m.H + m.H
				for k := 0; k < m.H; k++ {
					g[off+k] += d * h[k]
					if h[k] > 0 {
						dh := d * m.W2[k]
						g[m.In*m.H+k] += dh
						for j := 0; j < m.In; j++ {
							g[j*m.H+k] += dh * x[j]
						}
					}
				}
				g[np-1] += d
			}
			step++
			b1, b2 := 0.9, 0.999
			bs := float64(e - s)
			upd := func(i int, p *float64) {
				gi := g[i] / bs
				mo[i] = b1*mo[i] + (1-b1)*gi
				ve[i] = b2*ve[i] + (1-b2)*gi*gi
				mh := mo[i] / (1 - math.Pow(b1, float64(step)))
				vh := ve[i] / (1 - math.Pow(b2, float64(step)))
				*p -= lr * mh / (math.Sqrt(vh) + 1e-7)
			}
			for j := 0; j < m.In; j++ {
				for k := 0; k < m.H; k++ {
					upd(j*m.H+k, &m.W1[j][k])
				}
			}
			for k := 0; k < m.H; k++ {
				upd(m.In*m.H+k, &m.B1[k])
				upd(m.In*m.H+m.H+k, &m.W2[k])
			}
			upd(np-1, &m.B2)
		}
		l, a := m.lossAcc(Xt, yt)
		vl, va := m.lossAcc(Xv, yv)
		logs = append(logs, EpochLog{ep, l, a, vl, va})
	}
	return m, logs
}

func quant(w float64, scale float64) float64 {
	if scale == 0 {
		return w
	}
	q := math.Round(w / scale)
	q = math.Max(-127, math.Min(127, q))
	return q * scale
}

// Quantise returns a copy whose weight matrices are int8 (symmetric, per-tensor),
// like TFLite dynamic-range post-training quantisation; biases stay float.
func (m *NN) Quantise() (*NN, float64) {
	q := *m
	q.W1 = make([][]float64, m.In)
	q.W2 = make([]float64, m.H)
	var mx1, mx2 float64
	for _, r := range m.W1 {
		for _, v := range r {
			mx1 = math.Max(mx1, math.Abs(v))
		}
	}
	for _, v := range m.W2 {
		mx2 = math.Max(mx2, math.Abs(v))
	}
	s1, s2 := mx1/127, mx2/127
	var maxErr float64
	for j, r := range m.W1 {
		q.W1[j] = make([]float64, m.H)
		for k, v := range r {
			q.W1[j][k] = quant(v, s1)
			maxErr = math.Max(maxErr, math.Abs(q.W1[j][k]-v))
		}
	}
	for k, v := range m.W2 {
		q.W2[k] = quant(v, s2)
		maxErr = math.Max(maxErr, math.Abs(q.W2[k]-v))
	}
	q.QuantisedW = true
	return &q, maxErr
}

// ---------------------------------------------------------------- isolation forest

type iNode struct {
	feat        int
	thr         float64
	left, right *iNode
	size        int
}

type IForest struct {
	trees  []*iNode
	sample int
	Thr    float64 // score threshold learned from contamination
}

func cFactor(n int) float64 {
	if n <= 1 {
		return 0
	}
	if n == 2 {
		return 1
	}
	h := math.Log(float64(n-1)) + 0.5772156649
	return 2*h - 2*float64(n-1)/float64(n)
}

func buildI(X [][]float64, idx []int, depth, limit int, rng *rand.Rand) *iNode {
	if depth >= limit || len(idx) <= 1 {
		return &iNode{feat: -1, size: len(idx)}
	}
	nf := len(X[0])
	for try := 0; try < nf*2; try++ {
		f := rng.Intn(nf)
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, i := range idx {
			lo, hi = math.Min(lo, X[i][f]), math.Max(hi, X[i][f])
		}
		if hi-lo < 1e-12 {
			continue
		}
		thr := lo + rng.Float64()*(hi-lo)
		var l, r []int
		for _, i := range idx {
			if X[i][f] < thr {
				l = append(l, i)
			} else {
				r = append(r, i)
			}
		}
		return &iNode{feat: f, thr: thr, left: buildI(X, l, depth+1, limit, rng), right: buildI(X, r, depth+1, limit, rng)}
	}
	return &iNode{feat: -1, size: len(idx)}
}

func pathLen(n *iNode, x []float64, d int) float64 {
	if n.feat < 0 {
		return float64(d) + cFactor(n.size)
	}
	if x[n.feat] < n.thr {
		return pathLen(n.left, x, d+1)
	}
	return pathLen(n.right, x, d+1)
}

// FitIForest trains 100 trees and sets the decision threshold so that roughly
// `contamination` of the training data is flagged as anomalous.
func FitIForest(X [][]float64, contamination float64, seed int64) *IForest {
	rng := rand.New(rand.NewSource(seed))
	ss := 256
	if len(X) < ss {
		ss = len(X)
	}
	f := &IForest{sample: ss}
	limit := int(math.Ceil(math.Log2(float64(ss))))
	for t := 0; t < 100; t++ {
		perm := rng.Perm(len(X))[:ss]
		f.trees = append(f.trees, buildI(X, perm, 0, limit, rng))
	}
	sc := make([]float64, len(X))
	for i, x := range X {
		sc[i] = f.Score(x)
	}
	sorted := append([]float64(nil), sc...)
	sort.Float64s(sorted)
	k := int(math.Round(float64(len(sorted)) * (1 - contamination)))
	if k >= len(sorted) {
		k = len(sorted) - 1
	}
	if k < 0 {
		k = 0
	}
	f.Thr = sorted[k]
	if contamination <= 0 {
		f.Thr = math.Inf(1)
	}
	return f
}

// Score returns the anomaly score in (0,1); higher = more anomalous.
func (f *IForest) Score(x []float64) float64 {
	var s float64
	for _, t := range f.trees {
		s += pathLen(t, x, 0)
	}
	return math.Pow(2, -(s/float64(len(f.trees)))/cFactor(f.sample))
}

// Predict returns -1 for anomalies and 1 for normal points, like scikit-learn.
func (f *IForest) Predict(x []float64) int {
	if f.Score(x) > f.Thr {
		return -1
	}
	return 1
}

// ---------------------------------------------------------------- rolling helpers

var nan = math.NaN()

func window(x []float64, i, w int) []float64 {
	if i-w+1 < 0 {
		return nil
	}
	return x[i-w+1 : i+1]
}

func RollingMean(x []float64, w int) []float64 {
	out := make([]float64, len(x))
	for i := range x {
		ws := window(x, i, w)
		if ws == nil {
			out[i] = nan
			continue
		}
		var s float64
		for _, v := range ws {
			s += v
		}
		out[i] = s / float64(w) // NaN propagates, like pandas min_periods=window
	}
	return out
}

func RollingStd(x []float64, w int) []float64 {
	out := make([]float64, len(x))
	m := RollingMean(x, w)
	for i := range x {
		if math.IsNaN(m[i]) || w < 2 {
			out[i] = nan
			continue
		}
		var s float64
		for _, v := range window(x, i, w) {
			s += (v - m[i]) * (v - m[i])
		}
		out[i] = math.Sqrt(s / float64(w-1)) // sample std (ddof=1) as in pandas
	}
	return out
}

func RollingMax(x []float64, w int) []float64 {
	out := make([]float64, len(x))
	for i := range x {
		ws := window(x, i, w)
		if ws == nil {
			out[i] = nan
			continue
		}
		mx := math.Inf(-1)
		for _, v := range ws {
			mx = math.Max(mx, v)
		}
		out[i] = mx
	}
	return out
}

// RollingFeature computes the optional Lab 12 student-task features.
func RollingFeature(x []float64, w int, kind string) []float64 {
	out := make([]float64, len(x))
	for i := range x {
		ws := window(x, i, w)
		if ws == nil {
			out[i] = nan
			continue
		}
		var s, mn, mx = 0.0, math.Inf(1), math.Inf(-1)
		for _, v := range ws {
			s += v
			mn, mx = math.Min(mn, v), math.Max(mx, v)
		}
		mean := s / float64(w)
		var m2, m4, sq float64
		for _, v := range ws {
			d := v - mean
			m2 += d * d
			m4 += d * d * d * d
			sq += v * v
		}
		m2 /= float64(w)
		m4 /= float64(w)
		rms := math.Sqrt(sq / float64(w))
		switch kind {
		case "p2p":
			out[i] = mx - mn
		case "rms":
			out[i] = rms
		case "crest":
			out[i] = div(mx, rms)
		default: // kurtosis
			out[i] = div(m4, m2*m2)
		}
	}
	return out
}

// Median3 is a centred three-point median filter (edges replicate).
func Median3(x []float64) []float64 {
	out := make([]float64, len(x))
	for i := range x {
		a, b, c := x[maxi(i-1, 0)], x[i], x[mini(i+1, len(x)-1)]
		s := []float64{a, b, c}
		sort.Float64s(s)
		out[i] = s[1]
	}
	return out
}

func maxi(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func mini(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Interpolate fills NaNs by linear interpolation (like DataFrame.interpolate()).
func Interpolate(x []float64) []float64 {
	out := append([]float64(nil), x...)
	last := -1
	for i := 0; i < len(out); i++ {
		if math.IsNaN(out[i]) {
			continue
		}
		if last >= 0 && i-last > 1 {
			for k := last + 1; k < i; k++ {
				out[k] = out[last] + (out[i]-out[last])*float64(k-last)/float64(i-last)
			}
		}
		last = i
	}
	return out
}

// Stats helpers
func Mean(x []float64) float64 {
	var s float64
	for _, v := range x {
		s += v
	}
	return div(s, float64(len(x)))
}

func Std(x []float64) float64 { // sample std, like statistics.stdev
	if len(x) < 2 {
		return 0
	}
	m := Mean(x)
	var s float64
	for _, v := range x {
		s += (v - m) * (v - m)
	}
	return math.Sqrt(s / float64(len(x)-1))
}

func Percentile(x []float64, p float64) float64 {
	if len(x) == 0 {
		return 0
	}
	s := append([]float64(nil), x...)
	sort.Float64s(s)
	k := p / 100 * float64(len(s)-1)
	lo, hi := int(math.Floor(k)), int(math.Ceil(k))
	return s[lo] + (s[hi]-s[lo])*(k-float64(lo))
}

// Rules prints the tree like scikit-learn's export_text (useful for the viva).
func (t *Tree) Rules(names []string) string {
	var sb []byte
	var walk func(n *node, d int)
	pad := func(d int) string {
		s := ""
		for i := 0; i < d; i++ {
			s += "|   "
		}
		return s + "|--- "
	}
	walk = func(n *node, d int) {
		if n.feat < 0 {
			c := 0
			if n.p1 > 0.5 {
				c = 1
			}
			sb = append(sb, []byte(pad(d)+"class: "+string(rune('0'+c))+"\n")...)
			return
		}
		sb = append(sb, []byte(pad(d)+names[n.feat]+" <= "+ftoa(n.thr)+"\n")...)
		walk(n.left, d+1)
		sb = append(sb, []byte(pad(d)+names[n.feat]+" >  "+ftoa(n.thr)+"\n")...)
		walk(n.right, d+1)
	}
	if t.root != nil {
		walk(t.root, 0)
	}
	return string(sb)
}

func ftoa(v float64) string { return trimF(v) }

func trimF(v float64) string {
	s := fmtFloat(v)
	return s
}

func fmtFloat(v float64) string { return fmt.Sprintf("%.2f", v) }
