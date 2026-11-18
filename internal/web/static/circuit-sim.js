"use strict";
// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.
// CircuitSim: nets + simple nodal analysis for an Arduino Uno circuit, and the hardware layer used by CInt.
(function (root) {
  const UNO_PINS = ["D0", "D1", "D2", "D3", "D4", "D5", "D6", "D7", "D8", "D9", "D10", "D11", "D12", "D13", "A0", "A1", "A2", "A3", "A4", "A5", "5V", "3V3", "GND1", "GND2", "GND3", "VIN"];
  const PINS = { uno: UNO_PINS, led: ["A", "K"], res: ["1", "2"], btn: ["1", "2"], pot: ["VCC", "SIG", "GND"], ldr: ["1", "2"], dht11: ["VCC", "DATA", "GND"],
    pir: ["VCC", "OUT", "GND"], hcsr04: ["VCC", "TRIG", "ECHO", "GND"], buzzer: ["+", "-"] };
  const VF = { red: 1.8, yellow: 2.0, green: 2.1, blue: 3.0, white: 3.0 };
  const PWM = new Set([3, 5, 6, 9, 10, 11]);
  const pinName = n => (n >= 14 ? "A" + (n - 14) : "D" + n);
  const ldrOhms = light => 1000 + 199000 * Math.pow(1 - Math.max(0, Math.min(100, light)) / 100, 2);

  class Sim {
    constructor(circuit) { this.c = circuit; this.clock = () => 0; this.rev = 0; this.reset(); }
    setClock(f) { this.clock = f; }
    now() { return this.clock(); }
    reset() {
      this.pins = Array.from({ length: 20 }, () => ({ mode: 0, out: 0, duty: null, pullup: 0 }));
      this.tones = {}; this.burned = {}; this.trig = {}; this.warned = {}; this.floatPins = {}; this.notes = []; this.res = null; this.key = null; this.rev++;
    }
    touch() { this.rev++; }
    note(key, level, msg) { if (!this.warned[key]) { this.warned[key] = 1; this.notes.push({ level, msg }); } }

    // ---------------------------------------------------------- netlist + solve
    ensure() {
      this.expireTones();
      const auto = this.c.parts.some(p => p.type === "pir" && p.props.auto);
      const key = this.rev + ":" + (auto ? Math.floor(this.now() / 1e6) : 0);
      if (this.key !== key || !this.res) { this.key = key; this.res = this.solve(); }
      return this.res;
    }
    expireTones() {
      const t = this.now();
      for (const k of Object.keys(this.tones)) if (this.tones[k].until <= t) { const n = Number(k); delete this.tones[k]; this.pins[n].duty = null; this.pins[n].out = 0; this.rev++; }
    }
    pirMotion(p) {
      if (!p.props.auto) return !!p.props.motion;
      const b = Math.floor(this.now() / 6e6); let h = (b * 2654435761) >>> 0; h ^= h >>> 15; h = Math.imul(h, 2246822519) >>> 0;
      return (h % 1000) / 1000 < (p.props.prob ?? 0.3);
    }
    solve(second) {
      const c = this.c, parent = new Map();
      const find = k => { if (!parent.has(k)) parent.set(k, k); let r = k; while (parent.get(r) !== r) r = parent.get(r); let x = k; while (parent.get(x) !== r) { const n = parent.get(x); parent.set(x, r); x = n; } return r; };
      const union = (a, b) => { const ra = find(a), rb = find(b); if (ra !== rb) parent.set(ra, rb); };
      const byId = {}; for (const p of c.parts) { byId[p.id] = p; for (const pn of (PINS[p.type] || [])) find(p.id + "." + pn); }
      for (const w of c.wires) if (parent.has(w.a) && parent.has(w.b)) union(w.a, w.b);
      union("uno.GND1", "uno.GND2"); union("uno.GND1", "uno.GND3");
      const ids = new Map(); const net = k => { const r = find(k); if (!ids.has(r)) ids.set(r, ids.size); return ids.get(r); };
      for (const k of [...parent.keys()]) net(k);
      const N = ids.size, fixed = new Map(), shorts = [], duty = new Map();
      const addFixed = (n, v, src) => { if (fixed.has(n)) { const o = fixed.get(n); if (Math.abs(o.v - v) > 0.05) shorts.push(`${o.src} (${o.v} V) is connected to ${src} (${v} V)`); return; } fixed.set(n, { v, src }); };
      addFixed(net("uno.5V"), 5, "5V pin"); addFixed(net("uno.3V3"), 3.3, "3V3 pin"); addFixed(net("uno.GND1"), 0, "GND");
      const edges = [], leds = []; const n5 = net("uno.5V");
      for (let n = 0; n < 20; n++) {
        const pin = this.pins[n], nm = net("uno." + pinName(n));
        if (pin.mode === 1) { const hi = pin.duty !== null ? pin.duty > 0 : pin.out; addFixed(nm, hi ? 5 : 0, "pin " + pinName(n) + (hi ? " (HIGH)" : " (LOW)")); if (pin.duty !== null && pin.duty > 0 && pin.duty < 255) duty.set(nm, pin.duty / 255); }
        else if (pin.mode === 2 || pin.pullup) edges.push({ a: nm, b: n5, g: 1 / 30000 });
      }
      const powered = (p, vcc, gnd) => { const v = fixed.get(net(p.id + "." + vcc)), g = fixed.get(net(p.id + "." + gnd)); return !!(v && v.v >= 3 && g && g.v === 0); };
      const info = {};
      for (const p of c.parts) {
        const k = x => net(p.id + "." + x);
        switch (p.type) {
          case "res": edges.push({ a: k("1"), b: k("2"), g: 1 / Math.max(1, Number(p.props.ohms) || 220) }); break;
          case "ldr": edges.push({ a: k("1"), b: k("2"), g: 1 / ldrOhms(p.props.light ?? 50) }); break;
          case "pot": { const r = 10000, pos = Math.max(0, Math.min(1, p.props.pos ?? 0.5)); edges.push({ a: k("VCC"), b: k("SIG"), g: 1 / (r * (1 - pos) + 1) }, { a: k("SIG"), b: k("GND"), g: 1 / (r * pos + 1) }); break; }
          case "btn": if (p.props.pressed) edges.push({ a: k("1"), b: k("2"), g: 1 }); break;
          case "buzzer": edges.push({ a: k("+"), b: k("-"), g: 1 / 100 }); break;
          case "led": if (!this.burned[p.id]) leds.push({ id: p.id, a: k("A"), k: k("K"), vf: VF[p.props.color] || 1.8 }); break;
          case "pir": { const on = powered(p, "VCC", "GND"); info[p.id] = { powered: on }; if (on) addFixed(k("OUT"), this.pirMotion(p) ? 3.3 : 0, "PIR OUT"); break; }
        }
      }
      // Gauss-Seidel
      const V = new Array(N).fill(0); fixed.forEach((f, n) => { V[n] = f.v; });
      const adj = Array.from({ length: N }, () => []); for (const e of edges) { adj[e.a].push({ o: e.b, g: e.g }); adj[e.b].push({ o: e.a, g: e.g }); }
      const lbyNet = Array.from({ length: N }, () => []); for (const l of leds) { lbyNet[l.a].push(l); lbyNet[l.k].push(l); }
      const RD = 10;
      for (let it = 0; it < 400; it++) {
        let delta = 0;
        for (let n = 0; n < N; n++) {
          if (fixed.has(n)) continue;
          let num = 0, den = 0;
          for (const e of adj[n]) { num += e.g * V[e.o]; den += e.g; }
          for (const l of lbyNet[n]) if (V[l.a] - V[l.k] > l.vf) { const g = 1 / RD; if (n === l.a) { num += g * (V[l.k] + l.vf); den += g; } else { num += g * (V[l.a] - l.vf); den += g; } }
          if (den > 0) { const nv = num / den; delta = Math.max(delta, Math.abs(nv - V[n])); V[n] = nv; }
        }
        if (delta < 1e-7) break;
      }
      // reachability (floating nets)
      const reach = new Array(N).fill(false), q = []; fixed.forEach((_, n) => { reach[n] = true; q.push(n); });
      const lconn = leds.filter(l => V[l.a] - V[l.k] > l.vf);
      while (q.length) { const n = q.pop(); for (const e of adj[n]) if (!reach[e.o]) { reach[e.o] = true; q.push(e.o); } for (const l of lconn) { const o = l.a === n ? l.k : l.k === n ? l.a : -1; if (o >= 0 && !reach[o]) { reach[o] = true; q.push(o); } } }
      // LED states
      const ledRes = {}; let burnedNow = false;
      for (const l of leds) {
        const i = Math.max(0, (V[l.a] - V[l.k] - l.vf) / RD);
        if (i > 0.04) { this.burned[l.id] = true; burnedNow = true; this.note("burn" + l.id, "error", `${(byId[l.id].label || "LED")} burned out: it received ${(i * 1000).toFixed(0)} mA. Add a series resistor (220 Ω) and press Reset.`); continue; }
        let f = 1; const seen = new Set([l.a]); const st = [l.a, l.k]; seen.add(l.k);
        while (st.length) { const n = st.pop(); if (duty.has(n)) { f = duty.get(n); break; } for (const e of adj[n]) if (!seen.has(e.o)) { seen.add(e.o); st.push(e.o); } }
        ledRes[l.id] = { i, on: i > 0.0005, b: Math.min(1, i / 0.012) * f };
      }
      for (const id of Object.keys(this.burned)) ledRes[id] = { i: 0, on: false, b: 0, burned: true };
      if (burnedNow && !second) return this.solve(true);
      for (const s of shorts) this.note("short" + s, "error", "Short circuit: " + s + ". Remove the wire.");
      return { net, V, reach, fixed, ledRes, shorts, info, byId, duty };
    }

    volts(partId, pin) { const r = this.ensure(); const n = r.net(partId + "." + pin); return r.reach[n] ? r.V[n] : null; }
    pinNet(n) { return this.ensure().net("uno." + pinName(n)); }

    // ---------------------------------------------------------- hardware interface for CInt
    get hw() {
      const s = this;
      return {
        setClock: f => s.setClock(f),
        pinMode(p, m) { if (!s.pins[p]) return s.note("pm" + p, "warn", `pinMode(${p}): there is no pin ${p} on an Uno.`); s.pins[p].mode = m === 2 ? 2 : m === 1 ? 1 : 0; if (m === 1) s.pins[p].out = 0; s.pins[p].pullup = m === 2 ? 1 : 0; s.rev++; },
        digitalWrite(p, v) {
          const pin = s.pins[p]; if (!pin) return s.note("dw" + p, "warn", `digitalWrite(${p}): there is no pin ${p} on an Uno.`);
          if (pin.mode === 0) { pin.pullup = v ? 1 : 0; s.rev++; return; }
          const rising = v && !pin.out; pin.out = v; pin.duty = null; s.rev++;
          if (rising) { const r = s.ensure(), n = r.net("uno." + pinName(p)); for (const q of s.c.parts) if (q.type === "hcsr04" && r.net(q.id + ".TRIG") === n) s.trig[q.id] = s.now(); }
        },
        digitalRead(p) {
          const pin = s.pins[p]; if (!pin) return 0; if (pin.mode === 1) return pin.out ? 1 : 0;
          const r = s.ensure(), n = r.net("uno." + pinName(p));
          if (!r.reach[n]) { if (!s.floatPins[p]) { s.floatPins[p] = 1; s.note("fl" + p, "warn", `Pin ${pinName(p)} is floating: nothing drives it, so digitalRead() returns random values. Use INPUT_PULLUP or add a pull-up/pull-down resistor.`); } return Math.random() < 0.5 ? 1 : 0; }
          const v = r.V[n]; return v >= 3 ? 1 : v <= 1.5 ? 0 : (Math.random() < 0.5 ? 1 : 0);
        },
        analogRead(i) {
          const p = 14 + i; const pin = s.pins[p]; if (!pin) return 0; const r = s.ensure(), n = r.net("uno." + pinName(p));
          if (!r.reach[n]) { s.note("fa" + p, "warn", `Pin A${i} is floating: analogRead() returns noise. Connect the sensor and complete the circuit.`); return Math.floor(Math.random() * 1024); }
          return Math.max(0, Math.min(1023, Math.floor(r.V[n] / 5 * 1023)));
        },
        analogWrite(p, d) { const pin = s.pins[p]; if (!pin) return; pin.mode = 1; if (PWM.has(p)) { pin.duty = d; pin.out = d > 0 ? 1 : 0; } else { pin.duty = null; pin.out = d >= 128 ? 1 : 0; s.note("pwm" + p, "warn", `Pin D${p} has no PWM: analogWrite() acts as digitalWrite (PWM pins are 3, 5, 6, 9, 10, 11).`); } s.rev++; },
        tone(p, f, dur) { const pin = s.pins[p]; if (!pin) return; pin.mode = 1; pin.duty = 128; pin.out = 1; s.tones[p] = { f, until: dur > 0 ? s.now() + dur * 1000 : Infinity }; s.rev++; },
        noTone(p) { const pin = s.pins[p]; if (!pin) return; delete s.tones[p]; pin.duty = null; pin.out = 0; s.rev++; },
        dht(p) {
          const r = s.ensure(), n = r.net("uno." + pinName(p));
          for (const q of s.c.parts) if (q.type === "dht11" && r.net(q.id + ".DATA") === n) {
            const v = r.V[r.net(q.id + ".VCC")], g = r.V[r.net(q.id + ".GND")], pv = r.reach[r.net(q.id + ".VCC")], pg = r.reach[r.net(q.id + ".GND")];
            if (pv && pg && v >= 3 && g <= 0.3) return { t: Math.round(q.props.temp ?? 24), h: Math.round(q.props.hum ?? 50) };
            s.note("dhtpow" + q.id, "warn", `${q.label || "DHT11"}: not powered. Connect VCC to 5V and GND to GND.`); return null;
          }
          s.note("dhtpin" + p, "warn", `DHT read failed: no DHT11 DATA wire is connected to pin ${pinName(p)}. Check the wiring and the pin number in DHT dht(pin, type).`); return null;
        },
        pulseIn(p, level, timeout) {
          const r = s.ensure(), n = r.net("uno." + pinName(p));
          for (const q of s.c.parts) if (q.type === "hcsr04" && r.net(q.id + ".ECHO") === n) {
            const v = r.V[r.net(q.id + ".VCC")], g = r.V[r.net(q.id + ".GND")];
            if (!(r.reach[r.net(q.id + ".VCC")] && v >= 3 && g <= 0.3)) { s.note("hcpow" + q.id, "warn", `${q.label || "HC-SR04"}: not powered (VCC to 5V, GND to GND).`); break; }
            const t0 = s.trig[q.id]; const d = q.props.dist ?? 80;
            if (t0 === undefined || s.now() - t0 > 100000) { s.note("hctrig" + q.id, "warn", "HC-SR04: no trigger pulse seen. Wire TRIG to a pin you set HIGH for 10 µs."); break; }
            if (d < 2 || d > 400 || level !== 1) break;
            s.trig[q.id] = undefined; const us = Math.round(d * 58.2); return { us, advance: us + 400 };
          }
          return { us: 0, advance: Math.min(timeout || 1e6, 1e6) };
        },
      };
    }

    // ---------------------------------------------------------- view model for the UI
    leds() { return this.ensure().ledRes; }
    sounds() {
      const r = this.ensure(), out = {};
      for (const p of this.c.parts) if (p.type === "buzzer") {
        const np = r.net(p.id + "."+ "+"), nm = r.net(p.id + ".-"); let f = 0;
        for (const k of Object.keys(this.tones)) if (r.net("uno." + pinName(Number(k))) === np && r.V[nm] < 0.3) f = this.tones[k].f;
        if (!f && r.reach[np] && r.V[np] - r.V[nm] >= 2) f = 2000;
        out[p.id] = f;
      }
      return out;
    }
    diagnostics() {
      const r = this.ensure(), out = [];
      for (const n of this.notes) out.push(n);
      const used = new Set(); for (const w of this.c.wires) { used.add(w.a.split(".")[0]); used.add(w.b.split(".")[0]); }
      for (const p of this.c.parts) {
        if (p.type === "uno") continue;
        if (!used.has(p.id)) { out.push({ level: "info", msg: `${p.label || p.type} is not wired yet.` }); continue; }
        if (["dht11", "pir", "hcsr04"].includes(p.type)) {
          const v = r.reach[r.net(p.id + ".VCC")] ? r.V[r.net(p.id + ".VCC")] : null, g = r.reach[r.net(p.id + ".GND")] ? r.V[r.net(p.id + ".GND")] : null;
          if (v === null || g === null || v < 3 || g > 0.3) out.push({ level: "warn", msg: `${p.label || p.type}: VCC/GND not connected to 5V and GND – it will not work.` });
        }
      }
      if (!out.some(o => o.level === "error" || o.level === "warn")) out.push({ level: "ok", msg: "No wiring problems detected." });
      return out;
    }
  }
  const api = { Sim, PINS, UNO_PINS, pinName, ldrOhms, VF };
  if (typeof module !== "undefined" && module.exports) module.exports = api; else root.CircuitSim = api;
})(typeof window !== "undefined" ? window : globalThis);
