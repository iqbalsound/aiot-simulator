// Package sim models the AI-4301 lab kit: DHT11/DHT22, PIR, LDR (voltage divider),
// an optional HC-SR04 distance sensor, an MPU6050 vibration channel, and a virtual
// Arduino that prints structured serial lines exactly like the manual's sketches.
package sim

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// Env is the physical environment the sensors observe (controlled from the UI).
type Env struct {
	Temp       float64 `json:"temp"`       // deg C
	Hum        float64 `json:"hum"`        // %RH
	Light      float64 `json:"light"`      // 0..100 %
	MotionProb float64 `json:"motionProb"` // 0..1 per sample
	Distance   float64 `json:"distance"`   // cm (HC-SR04)
	Noise      float64 `json:"noise"`      // 0..3 noise multiplier
	Glitch     float64 `json:"glitch"`     // 0..0.3 probability of a bad line
}

func DefaultEnv() Env {
	return Env{Temp: 25, Hum: 50, Light: 55, MotionProb: 0.15, Distance: 80, Noise: 1, Glitch: 0.03}
}

// Wiring maps each sensor to the pin it is connected to ("" = not connected).
type Wiring struct {
	DHT   string `json:"dht"`
	PIR   string `json:"pir"`
	LDR   string `json:"ldr"`
	Extra string `json:"extra"`
}

// CorrectWiring is the wiring required by the manual's Lab 2 sketch.
func CorrectWiring() Wiring { return Wiring{DHT: "D2", PIR: "D3", LDR: "A0"} }

// Sample is one sensing cycle (before text formatting).
type Sample struct {
	TS       float64 `json:"ts"`
	Temp     float64 `json:"temp"` // NaN when the DHT read failed
	Hum      float64 `json:"hum"`
	Motion   int     `json:"motion"`
	Light    int     `json:"light"` // raw ADC 0..1023
	Distance float64 `json:"distance"`
	Vib      float64 `json:"vib"`
}

// Sensors generates samples from an Env.
type Sensors struct {
	mu     sync.Mutex
	rng    *rand.Rand
	Env    Env
	Wiring Wiring
	DHT22  bool
	heat   int
	cover  int
	shake  int
	faulty bool // degraded machine mode for predictive maintenance
}

func NewSensors(seed int64) *Sensors {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	return &Sensors{rng: rand.New(rand.NewSource(seed)), Env: DefaultEnv(), Wiring: CorrectWiring()}
}

func (s *Sensors) SetEnv(e Env)       { s.mu.Lock(); s.Env = e; s.mu.Unlock() }
func (s *Sensors) GetEnv() Env        { s.mu.Lock(); defer s.mu.Unlock(); return s.Env }
func (s *Sensors) SetWiring(w Wiring) { s.mu.Lock(); s.Wiring = w; s.mu.Unlock() }
func (s *Sensors) GetWiring() Wiring  { s.mu.Lock(); defer s.mu.Unlock(); return s.Wiring }
func (s *Sensors) SetDHT22(v bool)    { s.mu.Lock(); s.DHT22 = v; s.mu.Unlock() }

// Event triggers a timed physical disturbance ("heat", "cover", "shake", "clear").
func (s *Sensors) Event(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch name {
	case "heat":
		s.heat = 12
	case "cover":
		s.cover = 10
	case "shake":
		s.shake = 10
	case "clear":
		s.heat, s.cover, s.shake = 0, 0, 0
	default:
		return false
	}
	return true
}

// SetFaulty switches the vibration channel between healthy and degraded.
func (s *Sensors) SetFaulty(f bool) { s.mu.Lock(); s.faulty = f; s.mu.Unlock() }

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// LDRRaw converts light percentage to the ADC reading of the manual's voltage
// divider: LDR from 5V to A0, fixed 10k resistor from A0 to GND.
func LDRRaw(lightPct float64) int {
	p := clamp(lightPct, 0, 100) / 100
	rLDR := 1.0 + 199.0*math.Pow(1-p, 2) // kilo-ohms: bright ~1k, dark ~200k
	return int(math.Round(1023 * 10.0 / (rLDR + 10.0)))
}

// LightLevel is the manual's classification thresholds (<300 DARK, <700 DIM).
func LightLevel(raw int) string {
	if raw < 300 {
		return "DARK"
	}
	if raw < 700 {
		return "DIM"
	}
	return "BRIGHT"
}

// Read produces the next sample at simulation time ts.
func (s *Sensors) Read(ts float64) Sample {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, w := s.Env, s.Wiring
	n := func(sd float64) float64 { return s.rng.NormFloat64() * sd * e.Noise }

	heatOff := 0.0
	if s.heat > 0 {
		heatOff = 9 * math.Sin(math.Pi*float64(12-s.heat)/12) // warm up then cool
		s.heat--
	}
	lightP := e.Light
	if s.cover > 0 {
		lightP = 3
		s.cover--
	}
	smp := Sample{TS: ts}
	// DHT: temperature + humidity (humidity falls slightly as the sensor warms)
	t := e.Temp + heatOff + n(0.5)
	h := e.Hum - 0.8*heatOff + n(1.0)
	if s.DHT22 {
		t, h = math.Round(t*10)/10, math.Round(h*10)/10
	} else {
		t, h = math.Round(clamp(t, 0, 50)), math.Round(clamp(h, 20, 90))
	}
	smp.Temp, smp.Hum = clamp(t, -40, 80), clamp(h, 0, 100)
	if w.DHT != "D2" {
		smp.Temp, smp.Hum = math.NaN(), math.NaN() // not wired to the pin in the sketch
	}
	// PIR
	if w.PIR == "D3" && s.rng.Float64() < e.MotionProb {
		smp.Motion = 1
	}
	// LDR
	if w.LDR == "A0" {
		smp.Light = int(clamp(float64(LDRRaw(lightP))+n(6), 0, 1023))
	} else {
		smp.Light = s.rng.Intn(1024) // floating analog input
	}
	if w.Extra != "" {
		smp.Distance = math.Round(clamp(e.Distance+n(1.5), 2, 400)*10) / 10
	}
	// Vibration magnitude (g): healthy ~1 g with small noise; degraded adds periodic impacts
	v := 1.0 + n(0.03)
	if s.faulty || s.shake > 0 {
		v += 0.01*math.Sin(ts*2.3) + n(0.022)
		if s.rng.Float64() < 0.025 {
			v += 0.1 + s.rng.Float64()*0.2 // occasional impacts from developing wear
		}
		if s.shake > 0 {
			s.shake--
		}
	}
	smp.Vib = math.Round(v*1000) / 1000
	return smp
}

// Sketches supported by the virtual Arduino.
var Sketches = []struct{ ID, Name string }{
	{"blink", "Blink (no serial output)"},
	{"dht", "DHT11 only (Lab 2, Step 5)"},
	{"dht_pir", "DHT11 + PIR (Lab 2, Step 7)"},
	{"combined", "Combined CSV line (Lab 2, Step 9)"},
	{"combined_extra", "Combined CSV + extra sensor (Lab 2, Student Task)"},
}

func f2(v float64) string {
	if math.IsNaN(v) {
		return "nan"
	}
	return fmt.Sprintf("%.2f", v) // Arduino Serial.print(float) prints 2 decimals
}

// Format renders what the sketch prints for this sample. glitch may corrupt the line.
func (s *Sensors) Format(sketch string, smp Sample) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	line := ""
	switch sketch {
	case "dht":
		line = fmt.Sprintf("Temp: %s C, Humidity: %s", f2(smp.Temp), f2(smp.Hum))
	case "dht_pir":
		m := "No motion"
		if smp.Motion == 1 {
			m = "Motion detected"
		}
		line = fmt.Sprintf("Temp: %s C, Humidity: %s, %s", f2(smp.Temp), f2(smp.Hum), m)
	case "combined", "combined_extra":
		line = fmt.Sprintf("%s,%s,%d,%d", f2(smp.Temp), f2(smp.Hum), smp.Motion, smp.Light)
		if sketch == "combined_extra" {
			line += "," + f2(smp.Distance)
		}
	default:
		return ""
	}
	// Line-level glitches (electrical noise, dropped bytes, outliers)
	if s.rng.Float64() < s.Env.Glitch {
		switch s.rng.Intn(3) {
		case 0: // dropped bytes -> truncated line
			if len(line) > 4 {
				line = line[:2+s.rng.Intn(len(line)-3)]
			}
		case 1: // corrupted byte
			b := []byte(line)
			b[s.rng.Intn(len(b))] = '#'
			line = string(b)
		case 2: // spike on temperature field for CSV sketches
			if strings.HasPrefix(sketch, "combined") && !math.IsNaN(smp.Temp) {
				parts := strings.Split(line, ",")
				parts[0] = f2(smp.Temp + 25)
				line = strings.Join(parts, ",")
			}
		}
	}
	return line
}

// WiringCheck returns per-sensor wiring status against the manual's pin table.
func WiringCheck(w Wiring) []map[string]any {
	ok := func(b bool) string {
		if b {
			return "OK"
		}
		return "WRONG"
	}
	return []map[string]any{
		{"sensor": "DHT11 DATA", "expected": "D2", "actual": w.DHT, "status": ok(w.DHT == "D2")},
		{"sensor": "PIR OUT", "expected": "D3", "actual": w.PIR, "status": ok(w.PIR == "D3")},
		{"sensor": "LDR divider mid-point", "expected": "A0", "actual": w.LDR, "status": ok(w.LDR == "A0")},
		{"sensor": "Extra sensor (optional)", "expected": "any free pin", "actual": w.Extra,
			"status": func() string {
				if w.Extra == "" {
					return "NOT USED"
				}
				if w.Extra == "D2" || w.Extra == "D3" || w.Extra == "A0" {
					return "CONFLICT"
				}
				return "OK"
			}()},
	}
}

// Board is the virtual microcontroller: it ticks at a (sped-up) rate and prints lines.
type Board struct {
	mu      sync.Mutex
	S       *Sensors
	sketch  string
	baud    int
	speed   float64
	running bool
	clock   float64
	stop    chan struct{}
	// OnTick is called with the sample and the line the sketch printed.
	OnTick func(smp Sample, line string)
}

func NewBoard(s *Sensors) *Board {
	return &Board{S: s, sketch: "combined", baud: 9600, speed: 1, clock: float64(time.Now().Unix())}
}

type BoardState struct {
	Sketch  string  `json:"sketch"`
	Baud    int     `json:"baud"`
	Speed   float64 `json:"speed"`
	Running bool    `json:"running"`
	Clock   float64 `json:"clock"`
}

func (b *Board) State() BoardState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return BoardState{b.sketch, b.baud, b.speed, b.running, b.clock}
}

func (b *Board) Configure(sketch string, baud int, speed float64) {
	b.mu.Lock()
	if sketch != "" {
		b.sketch = sketch
	}
	if baud > 0 {
		b.baud = baud
	}
	if speed > 0 {
		b.speed = speed
	}
	b.mu.Unlock()
}

// Step advances the board by exactly one sample period (1 s of simulated time).
func (b *Board) Step() (Sample, string) {
	b.mu.Lock()
	b.clock += 1
	ts, sk := b.clock, b.sketch
	b.mu.Unlock()
	smp := b.S.Read(ts)
	line := b.S.Format(sk, smp)
	if b.OnTick != nil {
		b.OnTick(smp, line)
	}
	return smp, line
}

func (b *Board) Start() {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return
	}
	b.running = true
	b.stop = make(chan struct{})
	stop := b.stop
	b.mu.Unlock()
	go func() {
		for {
			b.mu.Lock()
			d := time.Duration(float64(time.Second) / b.speed)
			b.mu.Unlock()
			select {
			case <-stop:
				return
			case <-time.After(d):
				b.Step()
			}
		}
	}()
}

func (b *Board) Stop() {
	b.mu.Lock()
	if b.running {
		b.running = false
		close(b.stop)
	}
	b.mu.Unlock()
}
