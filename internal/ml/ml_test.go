package ml

import (
	"math"
	"math/rand"
	"testing"
)

func comfortData(n int, seed int64) ([][]float64, []int) {
	r := rand.New(rand.NewSource(seed))
	var X [][]float64
	var y []int
	for i := 0; i < n; i++ {
		t := 16 + r.Float64()*20
		h := 30 + r.Float64()*50
		l := r.Float64() * 1023
		c := 0
		if t >= 20 && t <= 28 && h <= 60 {
			c = 1
		}
		X = append(X, []float64{t, h, l})
		y = append(y, c)
	}
	return X, y
}

func TestClassifiers(t *testing.T) {
	X, y := comfortData(600, 1)
	Xtr, Xte, ytr, yte := Split(X, y, 0.25, 42)
	if len(Xte) != 150 || len(Xtr) != 450 {
		t.Fatalf("split sizes %d/%d", len(Xtr), len(Xte))
	}
	idx := make([]int, len(Xtr))
	for i := range idx {
		idx[i] = i
	}
	dt := FitTree(Xtr, ytr, idx, TreeOpts{})
	pred := make([]int, len(Xte))
	for i, x := range Xte {
		pred[i] = dt.Predict(x)
	}
	if a := Evaluate(yte, pred).Accuracy; a < 0.93 {
		t.Errorf("decision tree accuracy %.3f", a)
	}
	imp := dt.Importances()
	if imp[2] > imp[0] || math.Abs(imp[0]+imp[1]+imp[2]-1) > 1e-9 {
		t.Errorf("importances %v (light should matter least)", imp)
	}
	rf := FitForest(Xtr, ytr, 50, 0, 7)
	for i, x := range Xte {
		pred[i] = rf.Predict(x)
	}
	if a := Evaluate(yte, pred).Accuracy; a < 0.95 {
		t.Errorf("forest accuracy %.3f", a)
	}
	kn := FitKNN(Xtr, ytr, 5, true)
	for i, x := range Xte {
		pred[i] = kn.Predict(x)
	}
	if a := Evaluate(yte, pred).Accuracy; a < 0.9 {
		t.Errorf("knn(std) accuracy %.3f", a)
	}
}

func TestNNAndQuantisation(t *testing.T) {
	X, y := comfortData(600, 2)
	Xtr, Xte, ytr, yte := Split(X, y, 0.25, 42)
	nn, logs := TrainNN(Xtr, ytr, 8, 30, 0.01, true, 3)
	if len(logs) != 30 || logs[29].Loss >= logs[0].Loss {
		t.Fatalf("loss did not fall: %v -> %v", logs[0].Loss, logs[29].Loss)
	}
	acc := func(m *NN) float64 {
		p := make([]int, len(Xte))
		for i, x := range Xte {
			p[i] = m.Predict(x)
		}
		return Evaluate(yte, p).Accuracy
	}
	a := acc(nn)
	if a < 0.85 {
		t.Errorf("nn accuracy %.3f", a)
	}
	q, e := nn.Quantise()
	if math.Abs(acc(q)-a) > 0.05 {
		t.Errorf("quantised accuracy %.3f vs %.3f", acc(q), a)
	}
	if e <= 0 {
		t.Error("expected non-zero quantisation error")
	}
	if nn.Params() != 3*8+8+8+1 || nn.MACs() != 3*8+8 {
		t.Errorf("params %d macs %d", nn.Params(), nn.MACs())
	}
}

func TestIsolationForest(t *testing.T) {
	r := rand.New(rand.NewSource(5))
	var X [][]float64
	for i := 0; i < 400; i++ {
		X = append(X, []float64{25 + r.NormFloat64(), 50 + r.NormFloat64()*2, 500 + r.NormFloat64()*20})
	}
	f := FitIForest(X, 0.05, 42)
	n := 0
	for _, x := range X {
		if f.Predict(x) == -1 {
			n++
		}
	}
	if n < 12 || n > 30 {
		t.Errorf("flagged %d of 400 at contamination 0.05", n)
	}
	if f.Predict([]float64{45, 10, 20}) != -1 {
		t.Error("obvious outlier not flagged")
	}
	if f.Predict([]float64{25, 50, 500}) != 1 {
		t.Error("centre point flagged")
	}
}

func TestRolling(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5, 6}
	m := RollingMean(x, 5)
	if !math.IsNaN(m[3]) || m[4] != 3 || m[5] != 4 {
		t.Errorf("rolling mean %v", m)
	}
	if s := RollingStd(x, 5); math.Abs(s[4]-math.Sqrt(2.5)) > 1e-9 {
		t.Errorf("rolling std %v", s)
	}
	if mx := RollingMax(x, 3); mx[2] != 3 || mx[5] != 6 {
		t.Errorf("rolling max %v", mx)
	}
	med := Median3([]float64{1, 100, 3, 4})
	if med[1] != 3 {
		t.Errorf("median %v", med)
	}
	ip := Interpolate([]float64{1, math.NaN(), math.NaN(), 4})
	if ip[1] != 2 || ip[2] != 3 {
		t.Errorf("interpolate %v", ip)
	}
	if p := Percentile([]float64{1, 2, 3, 4, 5}, 50); p != 3 {
		t.Errorf("percentile %v", p)
	}
}
