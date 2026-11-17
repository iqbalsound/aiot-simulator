package sim

import (
	"math"
	"strings"
	"testing"
)

func TestLDRAndFormat(t *testing.T) {
	if LDRRaw(100) < 850 || LDRRaw(0) > 100 {
		t.Errorf("LDR range %d..%d", LDRRaw(0), LDRRaw(100))
	}
	if LightLevel(LDRRaw(3)) != "DARK" || LightLevel(LDRRaw(100)) != "BRIGHT" {
		t.Error("light classification")
	}
	s := NewSensors(1)
	e := s.GetEnv()
	e.Glitch = 0
	s.SetEnv(e)
	smp := s.Read(1)
	line := s.Format("combined", smp)
	if len(strings.Split(line, ",")) != 4 {
		t.Errorf("combined line %q", line)
	}
	if !strings.HasPrefix(s.Format("dht", smp), "Temp: ") {
		t.Error("dht format")
	}
}

func TestWiringFaults(t *testing.T) {
	s := NewSensors(2)
	e := s.GetEnv()
	e.Glitch, e.MotionProb = 0, 1
	s.SetEnv(e)
	if smp := s.Read(1); math.IsNaN(smp.Temp) || smp.Motion != 1 {
		t.Errorf("correct wiring should read: %+v", smp)
	}
	s.SetWiring(Wiring{DHT: "D4", PIR: "D5", LDR: "A1"})
	smp := s.Read(2)
	if !math.IsNaN(smp.Temp) || smp.Motion != 0 {
		t.Errorf("miswired sensors must fail: %+v", smp)
	}
	if got := s.Format("combined", smp); !strings.HasPrefix(got, "nan,nan,0,") {
		t.Errorf("expected nan line, got %q", got)
	}
}

func TestEventsAndGlitches(t *testing.T) {
	s := NewSensors(3)
	e := s.GetEnv()
	e.Glitch, e.Noise = 0, 0
	s.SetEnv(e)
	base := s.Read(1).Temp
	s.Event("heat")
	peak := base
	for i := 0; i < 12; i++ {
		peak = math.Max(peak, s.Read(float64(i)).Temp)
	}
	if peak-base < 6 {
		t.Errorf("heat event raised temp by only %.1f", peak-base)
	}
	s.Event("cover")
	if s.Read(1).Light > 150 {
		t.Error("cover should darken LDR")
	}
	e.Glitch = 1
	s.SetEnv(e)
	bad := 0
	for i := 0; i < 60; i++ {
		l := s.Format("combined", s.Read(1))
		if len(strings.Split(l, ",")) != 4 || strings.Contains(l, "#") {
			bad++
		}
	}
	if bad == 0 {
		t.Error("glitches should corrupt some lines")
	}
}

func TestBoardStep(t *testing.T) {
	b := NewBoard(NewSensors(4))
	n := 0
	b.OnTick = func(Sample, string) { n++ }
	c0 := b.State().Clock
	b.Step()
	b.Step()
	if n != 2 || b.State().Clock != c0+2 {
		t.Errorf("ticks %d clock %v", n, b.State().Clock)
	}
}
