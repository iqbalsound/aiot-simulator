"use strict";
// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.
// Circuit Lab: visual Arduino Uno wiring editor + sketch runner (uses cint.js and circuit-sim.js).

const CL = (() => {
  const W = 1000, H = 640;
  const UNO = { x: 320, y: 400, w: 360, h: 210 };
  const unoPins = {};
  { const top = ["D13", "D12", "D11", "D10", "D9", "D8"]; top.forEach((n, i) => unoPins[n] = [60 + i * 20, 0, 0, -1]);
    ["D7", "D6", "D5", "D4", "D3", "D2", "D1", "D0"].forEach((n, i) => unoPins[n] = [190 + i * 20, 0, 0, -1]); unoPins.GND3 = [38, 0, 0, -1];
    ["3V3", "5V", "GND1", "GND2", "VIN"].forEach((n, i) => unoPins[n] = [60 + i * 24, 210, 0, 1]);
    ["A0", "A1", "A2", "A3", "A4", "A5"].forEach((n, i) => unoPins[n] = [200 + i * 24, 210, 0, 1]); }
  const DEF = {
    uno: { w: UNO.w, h: UNO.h, pins: unoPins, name: "Arduino Uno" },
    led: { w: 36, h: 70, pins: { A: [10, 66, 0, 1], K: [26, 66, 0, 1] }, name: "LED", tag: "LED" },
    res: { w: 90, h: 22, pins: { "1": [0, 11, -1, 0], "2": [90, 11, 1, 0] }, name: "Resistor", tag: "R" },
    btn: { w: 56, h: 56, pins: { "1": [0, 28, -1, 0], "2": [56, 28, 1, 0] }, name: "Push button", tag: "BTN" },
    pot: { w: 64, h: 70, pins: { VCC: [12, 70, 0, 1], SIG: [32, 70, 0, 1], GND: [52, 70, 0, 1] }, name: "Potentiometer", tag: "POT" },
    ldr: { w: 44, h: 64, pins: { "1": [12, 64, 0, 1], "2": [32, 64, 0, 1] }, name: "LDR (light sensor)", tag: "LDR" },
    dht11: { w: 66, h: 78, pins: { VCC: [11, 78, 0, 1], DATA: [33, 78, 0, 1], GND: [55, 78, 0, 1] }, name: "DHT11", tag: "DHT" },
    pir: { w: 80, h: 78, pins: { VCC: [14, 78, 0, 1], OUT: [40, 78, 0, 1], GND: [66, 78, 0, 1] }, name: "PIR motion sensor", tag: "PIR" },
    hcsr04: { w: 120, h: 64, pins: { VCC: [20, 64, 0, 1], TRIG: [48, 64, 0, 1], ECHO: [76, 64, 0, 1], GND: [102, 64, 0, 1] }, name: "HC-SR04 distance", tag: "SONAR" },
    buzzer: { w: 50, h: 58, pins: { "+": [14, 58, 0, 1], "-": [36, 58, 0, 1] }, name: "Buzzer", tag: "BZ" },
  };
  const ADD = ["led", "res", "btn", "pot", "ldr", "dht11", "pir", "hcsr04", "buzzer"];
  const DEFAULT_PROPS = { led: { color: "red" }, res: { ohms: 220 }, btn: { pressed: false, latch: false }, pot: { pos: 0.5 }, ldr: { light: 55 }, dht11: { temp: 24, hum: 50 }, pir: { motion: false, auto: false, prob: 0.3 }, hcsr04: { dist: 80 }, buzzer: {} };
  const LEDCOL = { red: "#ef4444", green: "#22c55e", yellow: "#facc15", blue: "#3b82f6", white: "#f1f5f9" };
  const OHMS = [100, 220, 330, 470, 1000, 2200, 4700, 10000, 47000, 100000];
  const fmtOhm = o => o >= 1000 ? (o / 1000) + " kΩ" : o + " Ω";

  let C = { parts: [{ id: "uno", type: "uno", x: UNO.x, y: UNO.y, props: {}, label: "Uno" }], wires: [], code: "", example: "" };
  let sim = null, vm = null, timer = null, speed = 1, sel = null, drag = null, wiring = null, serial = [], partial = "", wireSeq = 0, saveT = 0, ok2 = 0;
  const audio = { on: true, ctx: null, osc: null, gain: null };
  const $c = id => document.getElementById(id);

  // ---------------------------------------------------------- model helpers
  const byId = id => C.parts.find(p => p.id === id);
  function pinPos(ref) {
    const [id, pin] = ref.split("."); const p = byId(id); if (!p) return null; const d = DEF[p.type].pins[pin]; if (!d) return null;
    return { x: p.x + d[0], y: p.y + d[1], fx: d[2], fy: d[3] };
  }
  function newLabel(type) { const n = C.parts.filter(p => p.type === type).length + 1; return DEF[type].tag + n; }
  function addPart(type, x, y, props, label, id) {
    const n = C.parts.length - 1;
    const p = { id: id || type + (++wireSeq), type, x: x ?? 40 + (n % 6) * 140, y: y ?? 30 + Math.floor(n / 6) * 125, props: { ...DEFAULT_PROPS[type], ...(props || {}) }, label: label || newLabel(type) };
    C.parts.push(p); changed(); return p;
  }
  function changed() { if (sim) sim.touch(); clearTimeout(saveT); saveT = setTimeout(save, 400); draw(); }
  function save() { try { localStorage.setItem("aiot.circuit", JSON.stringify(C)); } catch { } }
  function loadLocal() {
    try { const s = JSON.parse(localStorage.getItem("aiot.circuit") || "null"); if (s && Array.isArray(s.parts) && s.parts.some(p => p.id === "uno")) { C = s; C.wires = C.wires || []; wireSeq = C.parts.length + 50; } } catch { }
  }
  function ensureSim() { if (!sim) sim = new CircuitSim.Sim(C); sim.c = C; return sim; }

  // ---------------------------------------------------------------- examples
  const R = (id, ohms, x, y) => ["res", id, x, y, { ohms }], L = (id, color, x, y) => ["led", id, x, y, { color }];
  const EXAMPLES = [
    { id: "blank", name: "Blank canvas", parts: [], wires: [], code: `void setup() {\n  Serial.begin(9600);\n}\n\nvoid loop() {\n  Serial.println("Hello from the Circuit Lab");\n  delay(1000);\n}\n` },
    { id: "blink", name: "1. Blink an LED (D13 + 220 Ω)", parts: [R("r1", 220, 150, 250), L("led1", "red", 300, 190)], wires: [["uno.D13", "r1.1"], ["r1.2", "led1.A"], ["led1.K", "uno.GND3"]],
      code: `// Blink an LED on pin 13 through a 220 ohm resistor\nvoid setup() {\n  pinMode(13, OUTPUT);\n}\n\nvoid loop() {\n  digitalWrite(13, HIGH);\n  delay(500);\n  digitalWrite(13, LOW);\n  delay(500);\n}\n` },
    { id: "button", name: "2. Button controls an LED (INPUT_PULLUP)", parts: [["btn", "btn1", 90, 200, {}], R("r1", 220, 420, 250), L("led1", "green", 560, 190)], wires: [["uno.D2", "btn1.1"], ["btn1.2", "uno.GND1"], ["uno.D8", "r1.1"], ["r1.2", "led1.A"], ["led1.K", "uno.GND3"]],
      code: `const int BUTTON = 2;\nconst int LED = 8;\n\nvoid setup() {\n  pinMode(BUTTON, INPUT_PULLUP);  // pressed = LOW\n  pinMode(LED, OUTPUT);\n  Serial.begin(9600);\n}\n\nvoid loop() {\n  int pressed = (digitalRead(BUTTON) == LOW);\n  digitalWrite(LED, pressed);\n  if (pressed) {\n    Serial.println("Pressed");\n  }\n  delay(50);\n}\n` },
    { id: "night", name: "3. Night light (LDR voltage divider)", parts: [["ldr", "ldr1", 90, 220, { light: 55 }], R("r2", 10000, 200, 330), R("r1", 220, 620, 250), L("led1", "yellow", 780, 190)], wires: [["uno.5V", "ldr1.1"], ["ldr1.2", "uno.A0"], ["uno.A0", "r2.1"], ["r2.2", "uno.GND1"], ["uno.D6", "r1.1"], ["r1.2", "led1.A"], ["led1.K", "uno.GND3"]],
      code: `const int LDR_PIN = A0;\nconst int LED = 6;\n\nvoid setup() {\n  Serial.begin(9600);\n  pinMode(LED, OUTPUT);\n}\n\nvoid loop() {\n  int raw = analogRead(LDR_PIN);\n  Serial.print("light=");\n  Serial.print(raw);\n  if (raw < 300) {\n    Serial.println("  DARK -> LED ON");\n    digitalWrite(LED, HIGH);\n  } else if (raw < 700) {\n    Serial.println("  DIM");\n    digitalWrite(LED, LOW);\n  } else {\n    Serial.println("  BRIGHT");\n    digitalWrite(LED, LOW);\n  }\n  delay(500);\n}\n` },
    { id: "pot", name: "4. Potentiometer dimmer (PWM)", parts: [["pot", "pot1", 120, 220, { pos: 0.4 }], R("r1", 220, 500, 250), L("led1", "red", 660, 190)], wires: [["uno.5V", "pot1.VCC"], ["pot1.SIG", "uno.A1"], ["pot1.GND", "uno.GND1"], ["uno.D9", "r1.1"], ["r1.2", "led1.A"], ["led1.K", "uno.GND3"]],
      code: `void setup() {\n  pinMode(9, OUTPUT);   // D9 supports PWM\n  Serial.begin(9600);\n}\n\nvoid loop() {\n  int v = analogRead(A1);            // 0..1023\n  int duty = map(v, 0, 1023, 0, 255);\n  analogWrite(9, duty);\n  Serial.print("pot=");\n  Serial.print(v);\n  Serial.print("  duty=");\n  Serial.println(duty);\n  delay(200);\n}\n` },
    { id: "dht", name: "5. DHT11 temperature and humidity", parts: [["dht11", "dht1", 150, 200, { temp: 26, hum: 55 }]], wires: [["uno.5V", "dht1.VCC"], ["dht1.DATA", "uno.D2"], ["dht1.GND", "uno.GND2"]],
      code: `#include <DHT.h>\n#define DHTPIN 2\n#define DHTTYPE DHT11\nDHT dht(DHTPIN, DHTTYPE);\n\nvoid setup() {\n  Serial.begin(9600);\n  dht.begin();\n}\n\nvoid loop() {\n  float h = dht.readHumidity();\n  float t = dht.readTemperature();\n  if (isnan(t) || isnan(h)) {\n    Serial.println("DHT read failed - check wiring");\n  } else {\n    Serial.print("Temp: ");\n    Serial.print(t);\n    Serial.print(" C, Humidity: ");\n    Serial.println(h);\n  }\n  delay(1000);\n}\n` },
    { id: "pir", name: "6. PIR motion alarm (LED + buzzer)", parts: [["pir", "pir1", 90, 190, { motion: false }], R("r1", 220, 330, 290), L("led1", "red", 470, 190), ["buzzer", "bz1", 650, 200, {}]], wires: [["uno.5V", "pir1.VCC"], ["pir1.OUT", "uno.D3"], ["pir1.GND", "uno.GND2"], ["uno.D7", "r1.1"], ["r1.2", "led1.A"], ["led1.K", "uno.GND3"], ["uno.D8", "bz1.+"], ["bz1.-", "uno.GND1"]],
      code: `const int PIR = 3;\nconst int LED = 7;\nconst int BUZZ = 8;\n\nvoid setup() {\n  pinMode(PIR, INPUT);\n  pinMode(LED, OUTPUT);\n  Serial.begin(9600);\n}\n\nvoid loop() {\n  if (digitalRead(PIR) == HIGH) {\n    digitalWrite(LED, HIGH);\n    tone(BUZZ, 1000, 200);\n    Serial.println("MOTION!");\n  } else {\n    digitalWrite(LED, LOW);\n  }\n  delay(250);\n}\n` },
    { id: "sonar", name: "7. HC-SR04 distance meter", parts: [["hcsr04", "s1", 160, 220, { dist: 120 }]], wires: [["uno.5V", "s1.VCC"], ["s1.TRIG", "uno.D9"], ["s1.ECHO", "uno.D10"], ["s1.GND", "uno.GND2"]],
      code: `const int TRIG = 9;\nconst int ECHO = 10;\n\nvoid setup() {\n  pinMode(TRIG, OUTPUT);\n  pinMode(ECHO, INPUT);\n  Serial.begin(9600);\n}\n\nvoid loop() {\n  digitalWrite(TRIG, LOW);\n  delayMicroseconds(2);\n  digitalWrite(TRIG, HIGH);\n  delayMicroseconds(10);\n  digitalWrite(TRIG, LOW);\n  long us = pulseIn(ECHO, HIGH, 30000);\n  float cm = us / 58.2;\n  Serial.print("distance: ");\n  Serial.print(cm);\n  Serial.println(" cm");\n  delay(300);\n}\n` },
    { id: "traffic", name: "8. Traffic light (3 LEDs, functions)", parts: [L("lr", "red", 80, 160), L("ly", "yellow", 200, 160), L("lg", "green", 320, 160), R("r1", 220, 60, 300), R("r2", 220, 200, 300), R("r3", 220, 340, 300)], wires: [["uno.D11", "r1.1"], ["r1.2", "lr.A"], ["lr.K", "uno.GND3"], ["uno.D12", "r2.1"], ["r2.2", "ly.A"], ["ly.K", "uno.GND3"], ["uno.D13", "r3.1"], ["r3.2", "lg.A"], ["lg.K", "uno.GND3"]],
      code: `const int RED = 11;\nconst int YEL = 12;\nconst int GRN = 13;\n\nvoid setLights(int r, int y, int g) {\n  digitalWrite(RED, r);\n  digitalWrite(YEL, y);\n  digitalWrite(GRN, g);\n}\n\nvoid setup() {\n  pinMode(RED, OUTPUT);\n  pinMode(YEL, OUTPUT);\n  pinMode(GRN, OUTPUT);\n}\n\nvoid loop() {\n  setLights(1, 0, 0); delay(3000);   // stop\n  setLights(1, 1, 0); delay(1000);   // get ready\n  setLights(0, 0, 1); delay(3000);   // go\n  setLights(0, 1, 0); delay(1000);   // slow down\n}\n` },
    { id: "lab2", name: "Lab 2 exercise: wire DHT11 + PIR + LDR (unwired)", parts: [["dht11", "dht1", 60, 200, { temp: 24, hum: 52 }], ["pir", "pir1", 200, 190, { motion: false, auto: true, prob: 0.3 }], ["ldr", "ldr1", 360, 210, { light: 60 }], R("r1", 10000, 480, 300)], wires: [],
      sol: [["uno.5V", "dht1.VCC"], ["dht1.GND", "uno.GND2"], ["dht1.DATA", "uno.D2"], ["uno.5V", "pir1.VCC"], ["pir1.GND", "uno.GND2"], ["pir1.OUT", "uno.D3"], ["uno.5V", "ldr1.1"], ["ldr1.2", "uno.A0"], ["uno.A0", "r1.1"], ["r1.2", "uno.GND1"]],
      code: `// Lab 2: combined sensor line  temperature,humidity,motion,light\n// Wire DHT11 DATA->D2, PIR OUT->D3, LDR divider middle->A0 (10k to GND), power sensors from 5V/GND.\n#include <DHT.h>\n#define DHTPIN 2\n#define DHTTYPE DHT11\n#define PIRPIN 3\nDHT dht(DHTPIN, DHTTYPE);\n\nvoid setup() {\n  Serial.begin(9600);\n  dht.begin();\n  pinMode(PIRPIN, INPUT);\n}\n\nvoid loop() {\n  float h = dht.readHumidity();\n  float t = dht.readTemperature();\n  int motion = digitalRead(PIRPIN);\n  int lightRaw = analogRead(A0);\n  Serial.print(t);        Serial.print(",");\n  Serial.print(h);        Serial.print(",");\n  Serial.print(motion);   Serial.print(",");\n  Serial.println(lightRaw);\n  delay(1000);\n}\n` },
  ];

  function loadExample(id, silent) {
    const ex = EXAMPLES.find(e => e.id === id); if (!ex) return;
    if (!silent && C.parts.length > 1 && !confirm("Replace the current circuit and sketch with this example?")) return;
    stop(); C = { parts: [C.parts[0]], wires: [], code: ex.code, example: id, sol: ex.sol || null }; wireSeq = 0;
    for (const [type, pid, x, y, props] of ex.parts) addPart(type, x, y, props, null, pid);
    for (const [a, b] of ex.wires) C.wires.push({ a, b });
    sim = null; ensureSim(); sel = null; serial = []; partial = ""; changed(); refreshAll();
  }

  // ---------------------------------------------------------------- drawing
  const st = s => `style="${s}"`;
  function partSVG(p, r, ls) {
    const d = DEF[p.type]; let b = "";
    switch (p.type) {
      case "uno": {
        const on13 = sim && sim.pins[13].mode === 1 && sim.pins[13].out;
        b = `<rect width="360" height="210" rx="14" ${st("fill:#0b6672;stroke:#06424b;stroke-width:2")}/><rect x="-22" y="70" width="46" height="56" rx="4" ${st("fill:#9ca3af;stroke:#4b5563")}/>
        <text x="180" y="108" text-anchor="middle" ${st("fill:#e6f7f9;font:bold 22px system-ui")}>ARDUINO UNO</text><text x="180" y="130" text-anchor="middle" ${st("fill:#a5d8de;font:11px system-ui")}>simulated · ATmega328P 16 MHz · 5 V logic</text>
        <rect x="46" y="14" width="300" height="12" rx="2" ${st("fill:#111827")}/><rect x="52" y="184" width="130" height="12" rx="2" ${st("fill:#111827")}/><rect x="192" y="184" width="152" height="12" rx="2" ${st("fill:#111827")}/>
        <circle cx="232" cy="52" r="5" ${st(`fill:${on13 ? "#fbbf24" : "#5b4a14"}`)}/><text x="242" y="56" ${st("fill:#e6f7f9;font:9px system-ui")}>L (D13)</text><circle cx="40" cy="52" r="4" ${st("fill:#4ade80")}/><text x="48" y="56" ${st("fill:#e6f7f9;font:9px system-ui")}>ON</text>
        ${[...Object.keys(unoPins)].map(n => { const q = unoPins[n]; const top = q[1] === 0; return `<text x="${q[0]}" y="${top ? 38 : 176}" text-anchor="middle" ${st("fill:#e6f7f9;font:8.5px system-ui")}>${n.replace("GND3", "GND").replace(/^GND[12]$/, "GND")}${[3, 5, 6, 9, 10, 11].includes(Number(n.slice(1))) && n[0] === "D" ? "~" : ""}</text>`; }).join("")}`;
        break;
      }
      case "led": {
        const s = ls[p.id] || {}, col = LEDCOL[p.props.color] || LEDCOL.red, bright = s.on ? 0.35 + 0.65 * s.b : 0.28;
        b = `${s.on ? `<circle cx="18" cy="22" r="${16 + 8 * s.b}" fill="${col}" opacity="${0.35 * s.b}" filter="url(#blur)"/>` : ""}
        <line x1="10" y1="38" x2="10" y2="66" ${st("stroke:#cbd5e1;stroke-width:2.5")}/><line x1="26" y1="38" x2="26" y2="66" ${st("stroke:#cbd5e1;stroke-width:2.5")}/>
        <path d="M6 40 V22 a12 12 0 0 1 24 0 V40 z" fill="${col}" opacity="${bright}" ${st("stroke:#0008;stroke-width:1")}/>${s.burned ? `<text x="18" y="34" text-anchor="middle" ${st("fill:#000;font:bold 20px system-ui")}>✕</text>` : ""}`;
        break;
      }
      case "res": b = `<line x1="0" y1="11" x2="90" y2="11" ${st("stroke:#cbd5e1;stroke-width:2.5")}/><rect x="16" y="1" width="58" height="20" rx="8" ${st("fill:#d9b98a;stroke:#7c5e2f")}/><text x="45" y="15" text-anchor="middle" ${st("fill:#3b2a0b;font:bold 11px system-ui")}>${fmtOhm(p.props.ohms)}</text>`; break;
      case "btn": b = `<rect x="6" y="6" width="44" height="44" rx="6" ${st("fill:#374151;stroke:#111827")}/><line x1="0" y1="28" x2="6" y2="28" ${st("stroke:#cbd5e1;stroke-width:2.5")}/><line x1="50" y1="28" x2="56" y2="28" ${st("stroke:#cbd5e1;stroke-width:2.5")}/><circle class="btnface" data-part="${p.id}" cx="28" cy="28" r="${p.props.pressed ? 11 : 14}" ${st(`fill:${p.props.pressed ? "#991b1b" : "#ef4444"};stroke:#7f1d1d;cursor:pointer`)}/>`; break;
      case "pot": { const a = (-135 + 270 * (p.props.pos ?? 0.5)) * Math.PI / 180; b = `<rect x="2" y="2" width="60" height="50" rx="6" ${st("fill:#2563eb;stroke:#1e3a8a")}/><circle cx="32" cy="28" r="18" ${st("fill:#e5e7eb;stroke:#6b7280")}/><line x1="32" y1="28" x2="${32 + 15 * Math.sin(a)}" y2="${28 - 15 * Math.cos(a)}" ${st("stroke:#111827;stroke-width:3;stroke-linecap:round")}/>`; break; }
      case "ldr": b = `<circle cx="22" cy="22" r="18" ${st("fill:#f5e6b8;stroke:#8a6d1d")}/><path d="M9 22 q4 -8 8 0 t8 0 t8 0" fill="none" ${st("stroke:#9a3412;stroke-width:2")}/><line x1="12" y1="40" x2="12" y2="64" ${st("stroke:#cbd5e1;stroke-width:2.5")}/><line x1="32" y1="40" x2="32" y2="64" ${st("stroke:#cbd5e1;stroke-width:2.5")}/><text x="22" y="52" text-anchor="middle" ${st("fill:var(--muted);font:9px system-ui")}>${Math.round(p.props.light)}%</text>`; break;
      case "dht11": b = `<rect x="2" y="2" width="62" height="62" rx="4" ${st("fill:#3b82f6;stroke:#1e3a8a")}/>${[0, 1, 2, 3, 4].map(i => `<line x1="10" y1="${10 + i * 7}" x2="56" y2="${10 + i * 7}" ${st("stroke:#1e3a8a;stroke-width:2")}/>`).join("")}<text x="33" y="56" text-anchor="middle" ${st("fill:#fff;font:bold 9px system-ui")}>DHT11</text>`; break;
      case "pir": { const m = sim && sim.pirMotion(p); b = `<rect x="2" y="2" width="76" height="66" rx="4" ${st("fill:#15803d;stroke:#14532d")}/><circle cx="40" cy="34" r="22" class="pirdome" data-part="${p.id}" ${st(`fill:${m ? "#fde68a" : "#f8fafc"};stroke:${m ? "#f59e0b" : "#94a3b8"};stroke-width:${m ? 4 : 2};cursor:pointer`)}/><text x="40" y="38" text-anchor="middle" ${st("fill:#475569;font:bold 9px system-ui")}>PIR</text>`; break; }
      case "hcsr04": b = `<rect x="2" y="2" width="116" height="50" rx="4" ${st("fill:#2563eb;stroke:#1e3a8a")}/><circle cx="30" cy="27" r="18" ${st("fill:#d1d5db;stroke:#4b5563;stroke-width:3")}/><circle cx="90" cy="27" r="18" ${st("fill:#d1d5db;stroke:#4b5563;stroke-width:3")}/><text x="60" y="24" text-anchor="middle" ${st("fill:#fff;font:bold 9px system-ui")}>HC-SR04</text><text x="60" y="38" text-anchor="middle" ${st("fill:#bfdbfe;font:9px system-ui")}>${p.props.dist} cm</text>`; break;
      case "buzzer": { const f = (r && r.sounds[p.id]) || 0; b = `<circle cx="25" cy="25" r="22" ${st("fill:#111827;stroke:#374151;stroke-width:2")}/><circle cx="25" cy="25" r="5" ${st("fill:#374151")}/><text x="12" y="20" ${st("fill:#9ca3af;font:bold 12px system-ui")}>+</text>${f ? `<path d="M50 14 q8 11 0 22 M56 8 q14 17 0 34" fill="none" ${st("stroke:#fbbf24;stroke-width:2.5")}/>` : ""}`; break; }
    }
    const lbl = p.type !== "uno" ? `<text x="${d.w / 2}" y="-6" text-anchor="middle" ${st("fill:var(--muted);font:10px system-ui")}>${esc(p.label)}</text>` : "";
    const selb = sel && sel.kind === "part" && sel.id === p.id ? `<rect x="-6" y="-14" width="${d.w + 12}" height="${d.h + 22}" rx="8" fill="none" ${st("stroke:var(--accent);stroke-width:1.6;stroke-dasharray:5 4")}/>` : "";
    return `<g transform="translate(${p.x},${p.y})" data-part="${p.id}" class="part" ${st(p.type === "uno" ? "" : "cursor:grab")}>${selb}${b}${lbl}</g>`;
  }
  const WCOL = ["#38bdf8", "#4ade80", "#fbbf24", "#a78bfa", "#fb7185", "#2dd4bf"];
  function wirePath(a, b) {
    const k = 46; return `M${a.x},${a.y} C${a.x + a.fx * k},${a.y + a.fy * k} ${b.x + b.fx * k},${b.y + b.fy * k} ${b.x},${b.y}`;
  }
  function draw() {
    const svg = $c("cv"); if (!svg) return;
    const s = ensureSim(); let r = null, leds = {};
    try { r = s.ensure(); leds = r.ledRes; r.sounds = s.sounds(); } catch (e) { r = null; }
    const parts = C.parts.map(p => partSVG(p, r, leds)).join("");
    const wires = C.wires.map((w, i) => {
      const a = pinPos(w.a), b = pinPos(w.b); if (!a || !b) return "";
      let col = WCOL[i % WCOL.length];
      if (r) { const f = r.fixed.get(r.net(w.a)); if (f) col = f.v === 5 ? "#ef4444" : f.v === 0 ? "#94a3b8" : f.v === 3.3 ? "#f59e0b" : col; }
      const on = sel && sel.kind === "wire" && sel.i === i; const d = wirePath(a, b);
      return `<path d="${d}" fill="none" ${st(`stroke:${col};stroke-width:${on ? 5 : 3.2};stroke-linecap:round;opacity:.95`)}/><path class="wire" data-wire="${i}" d="${d}" fill="none" ${st("stroke:transparent;stroke-width:12;cursor:pointer")}/>`;
    }).join("");
    let tmp = ""; if (wiring) { const a = pinPos(wiring.from); if (a) tmp = `<path d="M${a.x},${a.y} L${wiring.x},${wiring.y}" fill="none" ${st("stroke:var(--accent);stroke-width:3;stroke-dasharray:6 4")}/>`; }
    const pins = C.parts.map(p => Object.entries(DEF[p.type].pins).map(([n, d]) => {
      const v = r ? s.volts(p.id, n) : null; const tip = `${p.label || p.type} · ${n}${v === null ? " · floating" : " · " + v.toFixed(2) + " V"}`;
      return `<g class="pin" data-pin="${p.id}.${n}"><circle cx="${p.x + d[0]}" cy="${p.y + d[1]}" r="9" fill="transparent"/><circle cx="${p.x + d[0]}" cy="${p.y + d[1]}" r="4" ${st("fill:#fbbf24;stroke:#92400e;pointer-events:none")}/><title>${esc(tip)}</title></g>`;
    }).join("")).join("");
    const pl = C.parts.filter(p => p.type !== "uno").map(p => Object.entries(DEF[p.type].pins).map(([n, d]) => `<text class="pinlbl" x="${p.x + d[0]}" y="${p.y + d[1] + (d[3] > 0 ? 13 : d[2] < 0 ? -8 : -8)}" text-anchor="middle">${n}</text>`).join("")).join("");
    svg.innerHTML = `<defs><filter id="blur"><feGaussianBlur stdDeviation="6"/></filter></defs>${parts}${wires}${pl}${pins}${tmp}`;
    if (C.sol) { }
  }
  function svgPt(e) { const svg = $c("cv"), m = svg.getScreenCTM && svg.getScreenCTM(); if (!m) return { x: 0, y: 0 }; const p = new DOMPoint(e.clientX, e.clientY).matrixTransform(m.inverse()); return { x: p.x, y: p.y }; }

  // ---------------------------------------------------------------- interaction
  function bind() {
    const svg = $c("cv");
    svg.onpointerdown = e => {
      const pin = e.target.closest && e.target.closest("[data-pin]");
      if (pin) { const p = svgPt(e); wiring = { from: pin.dataset.pin, x: p.x, y: p.y }; svg.setPointerCapture(e.pointerId); sel = null; draw(); return; }
      const face = e.target.closest && e.target.closest(".btnface,.pirdome");
      if (face) { const part = byId(face.dataset.part); if (part.type === "btn") { part.props.latch ? part.props.pressed = !part.props.pressed : part.props.pressed = true; drag = { btn: part }; svg.setPointerCapture(e.pointerId); } else if (part.type === "pir" && !part.props.auto) part.props.motion = !part.props.motion; changed(); renderProps(); return; }
      const wr = e.target.closest && e.target.closest("[data-wire]"); if (wr) { sel = { kind: "wire", i: Number(wr.dataset.wire) }; draw(); renderProps(); return; }
      const pt = e.target.closest && e.target.closest("[data-part]");
      if (pt) { const part = byId(pt.dataset.part); sel = { kind: "part", id: part.id }; if (part.type !== "uno") { const p = svgPt(e); drag = { id: part.id, dx: p.x - part.x, dy: p.y - part.y }; svg.setPointerCapture(e.pointerId); } draw(); renderProps(); return; }
      sel = null; draw(); renderProps();
    };
    svg.onpointermove = e => {
      if (wiring) { const p = svgPt(e); wiring.x = p.x; wiring.y = p.y; draw(); return; }
      if (drag && drag.id) { const part = byId(drag.id), p = svgPt(e), d = DEF[part.type]; part.x = Math.max(0, Math.min(W - d.w, p.x - drag.dx)); part.y = Math.max(14, Math.min(H - d.h, p.y - drag.dy)); draw(); }
    };
    svg.onpointerup = e => {
      if (wiring) {
        const el = document.elementFromPoint(e.clientX, e.clientY), pin = el && el.closest && el.closest("[data-pin]");
        if (pin && pin.dataset.pin !== wiring.from) { const a = wiring.from, b = pin.dataset.pin; if (!C.wires.some(w => (w.a === a && w.b === b) || (w.a === b && w.b === a))) { C.wires.push({ a, b }); sim && sim.reset && 0; } }
        wiring = null; changed(); refreshDiag(); return;
      }
      if (drag && drag.btn) { if (!drag.btn.props.latch) drag.btn.props.pressed = false; changed(); }
      if (drag && drag.id) save();
      drag = null;
    };
  }
  function del() {
    if (!sel) return;
    if (sel.kind === "wire") C.wires.splice(sel.i, 1);
    else if (sel.id !== "uno") { C.parts = C.parts.filter(p => p.id !== sel.id); C.wires = C.wires.filter(w => !w.a.startsWith(sel.id + ".") && !w.b.startsWith(sel.id + ".")); }
    sel = null; changed(); renderProps(); refreshDiag();
  }
  function renderProps() {
    const el = $c("cprops"); if (!el) return;
    if (!sel) { el.innerHTML = `<h4>Wiring tips</h4><ul class="small"><li>Press on a <b>pin</b> (gold dot) and drag to another pin to make a wire.</li><li>Drag parts to move them. Select a wire or part and press <b>Delete</b>.</li><li>Hover a pin to read its voltage.</li><li>Red wire = 5 V, grey = GND.</li><li>Click the red cap of a button, or the PIR dome, to operate them while running.</li></ul>`; return; }
    if (sel.kind === "wire") { const w = C.wires[sel.i]; el.innerHTML = `<h4>Wire</h4><p class="small mono">${esc(w.a)} ↔ ${esc(w.b)}</p><button class="btn small bad" data-act="cdel">Delete wire</button>`; return; }
    const p = byId(sel.id); const d = DEF[p.type]; const sl = (k, lo, hi, step, unit) => `<label>${k}: <b id="pv_${k}">${p.props[k]}</b> ${unit || ""}<input type="range" data-prop="${k}" min="${lo}" max="${hi}" step="${step}" value="${p.props[k]}"></label>`;
    let body = "";
    switch (p.type) {
      case "led": body = `<label>Colour<select data-prop="color">${Object.keys(LEDCOL).map(c => `<option ${p.props.color === c ? "selected" : ""}>${c}</option>`).join("")}</select></label>`; break;
      case "res": body = `<label>Resistance<select data-prop="ohms" data-num="1">${OHMS.map(o => `<option value="${o}" ${p.props.ohms === o ? "selected" : ""}>${fmtOhm(o)}</option>`).join("")}</select></label>`; break;
      case "btn": body = `<label class="inline"><input type="checkbox" data-prop="latch" ${p.props.latch ? "checked" : ""}> latching (toggle) switch</label>`; break;
      case "pot": body = sl("pos", 0, 1, 0.01, ""); break;
      case "ldr": body = sl("light", 0, 100, 1, "% light") + `<p class="small muted">≈ ${fmtOhm(Math.round(CircuitSim.ldrOhms(p.props.light) / 10) * 10)}</p>`; break;
      case "dht11": body = sl("temp", 0, 50, 1, "°C") + sl("hum", 20, 90, 1, "%RH"); break;
      case "pir": body = `<label class="inline"><input type="checkbox" data-prop="motion" ${p.props.motion ? "checked" : ""}> motion detected</label><label class="inline"><input type="checkbox" data-prop="auto" ${p.props.auto ? "checked" : ""}> automatic random motion</label>` + sl("prob", 0, 1, 0.05, ""); break;
      case "hcsr04": body = sl("dist", 2, 400, 1, "cm"); break;
      case "buzzer": body = `<p class="small muted">Sounds when driven by tone() or a steady ≥ 2 V.</p>`; break;
      case "uno": body = `<p class="small muted">Digital pins D0–D13, analog A0–A5, 5 V, 3.3 V, GND. PWM pins are marked ~.</p>`; break;
    }
    el.innerHTML = `<h4>${esc(d.name)} <span class="muted small">${esc(p.label)}</span></h4>${body}${p.type !== "uno" ? `<div class="row" style="margin-top:8px"><button class="btn small bad" data-act="cdel">Delete part</button></div>` : ""}`;
    el.querySelectorAll("[data-prop]").forEach(inp => { inp.oninput = inp.onchange = () => { const k = inp.dataset.prop; p.props[k] = inp.type === "checkbox" ? inp.checked : (inp.type === "range" || inp.dataset.num) ? Number(inp.value) : inp.value; const lb = $c("pv_" + k); if (lb) lb.textContent = p.props[k]; changed(); if (inp.type === "checkbox" || inp.tagName === "SELECT") renderProps(); }; });
  }
  function refreshDiag() {
    const el = $c("cdiag"); if (!el) return; const s = ensureSim();
    let list = []; try { list = s.diagnostics(); } catch { }
    el.innerHTML = list.map(d => `<li class="${d.level === "error" ? "e" : d.level === "warn" ? "w" : d.level === "ok" ? "g" : ""}">${d.level === "error" ? "✖" : d.level === "warn" ? "⚠" : d.level === "ok" ? "✔" : "•"} ${esc(d.msg)}</li>`).join("");
  }

  // ---------------------------------------------------------------- running
  function status(msg, bad) { const e = $c("cstatus"); if (e) { e.textContent = msg; e.className = "pill " + (bad ? "warn" : ""); } }
  function out(s) {
    partial += s; const parts = partial.split("\n"); partial = parts.pop();
    for (const l of parts) serial.push(l.replace(/\r$/, ""));
    if (serial.length > 300) serial = serial.slice(-300);
  }
  function showSerial() { const b = $c("cserial"); if (!b) return; b.textContent = serial.join("\n") + (partial ? "\n" + partial : ""); b.scrollTop = b.scrollHeight; }
  function lineOf(src, n) { let i = 0, k = 1; while (k < n && i >= 0) { i = src.indexOf("\n", i) + 1; k++; } const e = src.indexOf("\n", i); return [i, e < 0 ? src.length : e]; }
  function showError(e) {
    const msg = e.name === "CErr" ? `sketch.ino:${e.line}: error: ${e.message}` : "runtime error: " + e.message;
    status(msg, true); serial.push("⚠ " + msg); showSerial();
    if (e.name === "CErr" && e.line) { const ta = $c("csrc"); if (ta) { const [a, b] = lineOf(ta.value, e.line); ta.focus(); ta.setSelectionRange(a, b); } }
  }
  function run() {
    stop(); C.code = $c("csrc").value; const s = ensureSim(); s.reset(); serial = []; partial = ""; ok2 = 0;
    try { vm = new CInt.Machine(C.code, s.hw, { out, warn: m => { serial.push("⚠ " + m); } }); } catch (e) { vm = null; showError(e); return; }
    try { if (audio.on && !audio.ctx && (window.AudioContext || window.webkitAudioContext)) audio.ctx = new (window.AudioContext || window.webkitAudioContext)(); if (audio.ctx && audio.ctx.resume) audio.ctx.resume(); } catch { }
    status("running…"); const last = { t: performance.now() };
    timer = setInterval(() => {
      const now = performance.now(), dt = Math.min(100, now - last.t); last.t = now;
      try { vm.run(dt * 1000 * speed); } catch (e) { stop(); showError(e); return; }
      if (sim.notes.length) { sim.notes.splice(0).forEach(n => serial.push((n.level === "error" ? "✖ " : "⚠ ") + n.msg)); }
      draw(); showSerial(); refreshDiag(); soundOut(); status(`running · ${(vm.micros / 1e6).toFixed(1)} s virtual time`); checkLab2();
      if (vm.finished) { stop(); status("sketch ended"); }
    }, 40);
    $c("crun").disabled = true; $c("cstop").disabled = false; refreshDiag();
  }
  function stop() { if (timer) { clearInterval(timer); timer = null; } setTone(0); const r = $c("crun"), s = $c("cstop"); if (r) r.disabled = false; if (s) s.disabled = true; if (vm && !vm.finished && !vm.error) status("stopped · " + (vm.micros / 1e6).toFixed(1) + " s"); }
  function reset() { stop(); vm = null; if (sim) sim.reset(); serial = []; partial = ""; C.parts.forEach(p => { if (p.type === "btn" && !p.props.latch) p.props.pressed = false; }); status("reset"); draw(); showSerial(); refreshDiag(); }
  function setTone(f) {
    try {
      if (!audio.on || !audio.ctx) return;
      if (f > 0) { if (!audio.osc) { audio.osc = audio.ctx.createOscillator(); audio.gain = audio.ctx.createGain(); audio.gain.gain.value = 0.04; audio.osc.type = "square"; audio.osc.connect(audio.gain); audio.gain.connect(audio.ctx.destination); audio.osc.start(); } audio.osc.frequency.value = f; }
      else if (audio.osc) { audio.osc.stop(); audio.osc.disconnect(); audio.osc = null; }
    } catch { }
  }
  function soundOut() { try { const s = sim.sounds(); const f = Math.max(0, ...Object.values(s)); setTone(f); } catch { } }
  function checkLab2() { // proves Lab 2 wiring: five clean T,H,M,L lines from DHT11 + PIR + LDR
    const good = serial.filter(l => /^-?\d+(\.\d+)?,-?\d+(\.\d+)?,[01],\d+$/.test(l)).length;
    if (good >= 5 && good !== ok2 && C.parts.some(p => p.type === "dht11") && C.parts.some(p => p.type === "pir") && C.parts.some(p => p.type === "ldr")) { ok2 = good; mark("circuit_run", true, `Circuit Lab run produced ${good} valid temperature,humidity,motion,light lines`); }
  }

  // ---------------------------------------------------------------- panel
  function panel() {
    return {
      html: `<div class="card circ"><div class="row"><h3 style="margin:0">Circuit Lab — wire and run an Arduino Uno</h3><span class="spacer"></span><span id="cstatus" class="pill">ready</span></div>
        <div class="row"><label style="min-width:260px">Example circuit<select id="cex">${EXAMPLES.map(e => `<option value="${e.id}" ${C.example === e.id ? "selected" : ""}>${esc(e.name)}</option>`).join("")}</select></label><button class="btn small ghost" data-act="cload" style="margin-top:16px">Load example</button>
          <span class="spacer"></span><span class="muted small" style="margin-top:16px">Add part:</span><div class="palette" style="margin-top:14px">${ADD.map(t => `<button class="btn small ghost" data-act="cadd" data-t="${t}">${esc(DEF[t].name)}</button>`).join("")}</div></div>
        <div class="circ-main"><svg id="cv" viewBox="0 0 ${W} ${H}" preserveAspectRatio="xMidYMid meet"></svg><div class="card" id="cprops" style="overflow:auto;max-height:640px"></div></div>
        <div class="row"><button class="btn" id="crun" data-act="crun">▶ Run sketch</button><button class="btn ghost" id="cstop" data-act="cstop" disabled>■ Stop</button><button class="btn ghost" data-act="creset">↺ Reset circuit state</button>
          ${sel0()}<label class="inline">Speed <select id="cspeed"><option value="1">1×</option><option value="2">2×</option><option value="5">5×</option><option value="10">10×</option></select></label><label class="inline"><input type="checkbox" id="caud" checked> sound</label><span class="spacer"></span>
          <button class="btn small ghost" data-act="csol" ${C.sol ? "" : "hidden"}>Show solution wires</button><button class="btn small ghost" data-act="cclear">Clear wires</button>
          <input id="cname" placeholder="circuit name" style="width:140px" value="my-circuit"><button class="btn small ghost" data-act="csave">Save</button><select id="clist" style="width:150px"></select><button class="btn small ghost" data-act="cloadsaved">Open</button></div>
        <div class="grid2"><div><label>Sketch (Arduino C/C++ subset)<textarea id="csrc" class="src" spellcheck="false"></textarea></label></div>
          <div style="display:grid;gap:8px;align-content:start"><div><h4 style="margin-top:0">Serial Monitor</h4><pre id="cserial" style="height:150px;min-height:150px;margin:0"></pre></div><div><h4 style="margin-top:0">Wiring check</h4><ul id="cdiag" class="diag small" style="margin:0;padding-left:18px"></ul></div></div></div>
        <p class="muted small">The simulator runs a C/C++ subset: functions, int/long/byte/float/String, arrays, if/for/while/switch, Serial, delay/millis, pinMode/digitalRead/analogRead/analogWrite/tone/pulseIn, DHT. Pointers, classes and other libraries are not supported yet. Integer sizes follow the Uno (int = 16 bit).</p></div>`,
      init() {
        $c("csrc").value = C.code; $c("csrc").oninput = () => { C.code = $c("csrc").value; clearTimeout(saveT); saveT = setTimeout(save, 600); };
        $c("csrc").onkeydown = e => { if (e.key === "Tab") { e.preventDefault(); const t = e.target, s = t.selectionStart; t.value = t.value.slice(0, s) + "  " + t.value.slice(t.selectionEnd); t.selectionStart = t.selectionEnd = s + 2; } };
        $c("cspeed").value = String(speed); $c("cspeed").onchange = e => { speed = Number(e.target.value); };
        $c("caud").onchange = e => { audio.on = e.target.checked; if (!audio.on) setTone(0); };
        bind(); draw(); renderProps(); refreshDiag(); showSerial(); listSaved();
      }
    };
  }
  const sel0 = () => "";
  async function listSaved() { try { const l = await api("/api/circuit/list"); const el = $c("clist"); if (el) el.innerHTML = l.length ? l.map(n => `<option>${esc(n)}</option>`).join("") : "<option value=''>(none saved)</option>"; } catch { } }
  function refreshAll() { const ta = $c("csrc"); if (ta) ta.value = C.code; const ex = $c("cex"); if (ex) ex.value = C.example || "blank"; draw(); renderProps(); refreshDiag(); showSerial(); const sol = document.querySelector('[data-act="csol"]'); if (sol) sol.hidden = !C.sol; }

  Object.assign(A, {
    circuit: () => { CUR = 0; renderLab(); },
    circ2: () => { CUR = 0; renderLab(); loadExample("lab2", true); },
    cload: () => loadExample($c("cex").value),
    cadd: el => { addPart(el.dataset.t); renderProps(); refreshDiag(); },
    crun: () => run(), cstop: () => { stop(); },
    creset: () => reset(),
    cdel: () => del(),
    cclear: () => { if (!C.wires.length || confirm("Remove all wires?")) { C.wires = []; sel = null; changed(); renderProps(); refreshDiag(); } },
    csol: () => { if (!C.sol) return; C.wires = C.sol.map(([a, b]) => ({ a, b })); changed(); refreshDiag(); toast("Solution wires added – press Run"); },
    csave: async () => { const r = await api("/api/circuit/save", { name: $c("cname").value.trim(), data: { parts: C.parts, wires: C.wires, code: $c("csrc").value } }); toast("Saved " + r.file); listSaved(); },
    cloadsaved: async () => { const n = $c("clist").value; if (!n) return; const d = await api("/api/circuit/load?name=" + encodeURIComponent(n)); if (!d.parts) throw new Error("not a circuit file"); stop(); C = { parts: d.parts, wires: d.wires || [], code: d.code || "", example: "", sol: null }; sim = null; ensureSim(); changed(); refreshAll(); toast("Loaded " + n); },
  });
  document.addEventListener("keydown", e => { if (CUR === 0 && (e.key === "Delete" || e.key === "Backspace") && document.activeElement && document.activeElement.tagName !== "TEXTAREA" && document.activeElement.tagName !== "INPUT" && document.activeElement.tagName !== "SELECT") { e.preventDefault(); del(); } });
  window.addEventListener("beforeunload", () => { stop(); });

  loadLocal(); if (!C.code) { const ex = EXAMPLES.find(e => e.id === "blink"); C.code = ex.code; }
  return { panel, loadExample, stop, get state() { return C; }, EXAMPLES, DEF, setCode: c => { C.code = c; } };
})();
PANELS[0] = () => CL.panel();
