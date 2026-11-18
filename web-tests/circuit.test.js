"use strict";
const test = require("node:test"), assert = require("node:assert");
const { Machine } = require("../internal/web/static/cint.js");
const { Sim } = require("../internal/web/static/circuit-sim.js");

const part = (id, type, props = {}) => ({ id, type, props, label: id });
const W = (a, b) => ({ a, b });
function mk(parts, wires) { return { parts: [part("uno", "uno"), ...parts], wires }; }
function go(circ, src, ms = 50, setup) {
  const sim = new Sim(circ); let out = "";
  const vm = new Machine(src, sim.hw, { out: s => { out += s; } });
  if (setup) setup(sim, vm);
  vm.run(ms * 1000); return { sim, vm, out, lines: out.trim().split("\n") };
}
const ledCirc = (ohms) => mk([part("r1", "res", { ohms }), part("l1", "led", { color: "red" })],
  [W("uno.D13", "r1.1"), W("r1.2", "l1.A"), W("l1.K", "uno.GND1")]);

test("LED with 220 ohm resistor lights at about 14 mA and turns off", () => {
  const { sim } = go(ledCirc(220), `void setup(){ pinMode(13, OUTPUT); digitalWrite(13, HIGH); } void loop(){ delay(10); }`);
  const s = sim.leds().l1; assert.ok(s.on && s.b > 0.9, JSON.stringify(s)); assert.ok(Math.abs(s.i - 0.0139) < 0.001, "current " + s.i);
  const off = go(ledCirc(220), `void setup(){ pinMode(13, OUTPUT); digitalWrite(13, LOW); } void loop(){ delay(10); }`).sim.leds().l1; assert.ok(!off.on);
});
test("LED wired backwards does not light; direct drive burns it", () => {
  const rev = mk([part("r1", "res", { ohms: 220 }), part("l1", "led", { color: "red" })], [W("uno.D13", "r1.1"), W("r1.2", "l1.K"), W("l1.A", "uno.GND1")]);
  assert.ok(!go(rev, `void setup(){ pinMode(13, OUTPUT); digitalWrite(13, HIGH); } void loop(){ delay(10); }`).sim.leds().l1.on);
  const direct = mk([part("l1", "led", { color: "red" })], [W("uno.D13", "l1.A"), W("l1.K", "uno.GND1")]);
  const { sim } = go(direct, `void setup(){ pinMode(13, OUTPUT); digitalWrite(13, HIGH); } void loop(){ delay(10); }`);
  assert.ok(sim.leds().l1.burned); assert.ok(sim.diagnostics().some(d => d.level === "error" && /burned/.test(d.msg)));
});
test("PWM dims the LED proportionally", () => {
  const c = mk([part("r1", "res", { ohms: 220 }), part("l1", "led", { color: "green" })], [W("uno.D9", "r1.1"), W("r1.2", "l1.A"), W("l1.K", "uno.GND1")]);
  const b = go(c, `void setup(){ analogWrite(9, 64); } void loop(){ delay(10); }`).sim.leds().l1.b; assert.ok(b > 0.2 && b < 0.3, "brightness " + b);
});
test("button with INPUT_PULLUP reads HIGH/LOW; without pull resistor it floats and warns", () => {
  const c = mk([part("b1", "btn", { pressed: false })], [W("uno.D2", "b1.1"), W("b1.2", "uno.GND1")]);
  const src = `void setup(){ pinMode(2, INPUT_PULLUP); Serial.println(digitalRead(2)); } void loop(){ delay(10); }`;
  assert.strictEqual(go(c, src).lines[0], "1");
  c.parts[1].props.pressed = true; assert.strictEqual(go(c, src).lines[0], "0");
  const fl = go(mk([part("b1", "btn")], [W("uno.D2", "b1.1"), W("b1.2", "uno.5V")]), `void setup(){ pinMode(2, INPUT); digitalRead(2); } void loop(){ delay(10); }`);
  assert.ok(fl.sim.diagnostics().some(d => /floating/.test(d.msg)), "floating warning expected");
  const pd = mk([part("b1", "btn", { pressed: true }), part("r1", "res", { ohms: 10000 })], [W("uno.D2", "b1.1"), W("b1.2", "uno.5V"), W("uno.D2", "r1.1"), W("r1.2", "uno.GND1")]);
  assert.strictEqual(go(pd, `void setup(){ pinMode(2, INPUT); Serial.println(digitalRead(2)); } void loop(){ delay(10); }`).lines[0], "1");
  pd.parts[1].props.pressed = false; assert.strictEqual(go(pd, `void setup(){ pinMode(2, INPUT); Serial.println(digitalRead(2)); } void loop(){ delay(10); }`).lines[0], "0");
});
test("LDR voltage divider gives the ADC values used in Lab 2", () => {
  const c = mk([part("ldr", "ldr", { light: 100 }), part("r", "res", { ohms: 10000 })], [W("uno.5V", "ldr.1"), W("ldr.2", "uno.A0"), W("uno.A0", "r.1"), W("r.2", "uno.GND1")]);
  const src = `void setup(){ Serial.println(analogRead(A0)); } void loop(){ delay(10); }`;
  const bright = Number(go(c, src).lines[0]); assert.ok(bright > 900 && bright < 960, "bright " + bright);
  c.parts[1].props.light = 0; const dark = Number(go(c, src).lines[0]); assert.ok(dark > 30 && dark < 70, "dark " + dark);
  const un = mk([part("ldr", "ldr")], [W("ldr.1", "uno.A0")]); // A0 connected to a single LDR leg only: floating
  assert.ok(go(un, src).sim.diagnostics().some(d => /floating/.test(d.msg)));
});
test("potentiometer wiper voltage", () => {
  const c = mk([part("p", "pot", { pos: 0.5 })], [W("uno.5V", "p.VCC"), W("uno.GND1", "p.GND"), W("p.SIG", "uno.A1")]);
  const v = Number(go(c, `void setup(){ Serial.println(analogRead(A1)); } void loop(){ delay(10); }`).lines[0]); assert.ok(Math.abs(v - 511) < 4, "pot " + v);
});
test("DHT11 works only when powered and on the pin used in the sketch", () => {
  const src = `DHT dht(2, DHT11); void setup(){ Serial.println(dht.readTemperature()); Serial.println(dht.readHumidity()); } void loop(){ delay(10); }`;
  const good = mk([part("d", "dht11", { temp: 27, hum: 61 })], [W("d.VCC", "uno.5V"), W("d.GND", "uno.GND2"), W("d.DATA", "uno.D2")]);
  assert.deepStrictEqual(go(good, src).lines, ["27.00", "61.00"]);
  const nop = mk([part("d", "dht11")], [W("d.GND", "uno.GND2"), W("d.DATA", "uno.D2")]);
  const r = go(nop, src); assert.deepStrictEqual(r.lines, ["nan", "nan"]); assert.ok(r.sim.diagnostics().some(d => /not powered/.test(d.msg)));
  const wrong = mk([part("d", "dht11")], [W("d.VCC", "uno.5V"), W("d.GND", "uno.GND2"), W("d.DATA", "uno.D4")]);
  assert.ok(go(wrong, src).sim.diagnostics().some(d => /no DHT11 DATA wire/.test(d.msg)));
});
test("PIR output follows motion and requires power", () => {
  const c = mk([part("p", "pir", { motion: false })], [W("p.VCC", "uno.5V"), W("p.GND", "uno.GND1"), W("p.OUT", "uno.D3")]);
  const src = `void setup(){ pinMode(3, INPUT); Serial.println(digitalRead(3)); } void loop(){ delay(10); }`;
  assert.strictEqual(go(c, src).lines[0], "0"); c.parts[1].props.motion = true; assert.strictEqual(go(c, src).lines[0], "1");
  const unpowered = mk([part("p", "pir", { motion: true })], [W("p.OUT", "uno.D3")]); assert.ok(go(unpowered, src).sim.diagnostics().length > 0);
});
test("HC-SR04 echo time matches distance after a trigger pulse", () => {
  const c = mk([part("s", "hcsr04", { dist: 80 })], [W("s.VCC", "uno.5V"), W("s.GND", "uno.GND1"), W("s.TRIG", "uno.D9"), W("s.ECHO", "uno.D10")]);
  const src = `void setup(){ pinMode(9, OUTPUT); pinMode(10, INPUT); digitalWrite(9, LOW); delayMicroseconds(2); digitalWrite(9, HIGH); delayMicroseconds(10); digitalWrite(9, LOW);
    long us = pulseIn(10, HIGH); Serial.println(us); Serial.println(us / 58); } void loop(){ delay(10); }`;
  assert.deepStrictEqual(go(c, src).lines, ["4656", "80"]);
  const notrig = go(c, `void setup(){ pinMode(10, INPUT); Serial.println(pulseIn(10, HIGH)); } void loop(){ delay(10); }`); assert.strictEqual(notrig.lines[0], "0");
});
test("short circuits and buzzer tones are detected", () => {
  const sh = mk([], [W("uno.D13", "uno.GND1")]);
  assert.ok(go(sh, `void setup(){ pinMode(13, OUTPUT); digitalWrite(13, HIGH); } void loop(){ delay(10); }`).sim.diagnostics().some(d => /Short circuit/.test(d.msg)));
  const bz = mk([part("z", "buzzer")], [W("uno.D8", "z.+"), W("z.-", "uno.GND1")]);
  const { sim } = go(bz, `void setup(){ tone(8, 440); } void loop(){ delay(10); }`); assert.strictEqual(sim.sounds().z, 440);
  const t = go(bz, `void setup(){ tone(8, 880, 20); } void loop(){ delay(10); }`, 5); assert.strictEqual(t.sim.sounds().z, 880);
  const later = go(bz, `void setup(){ tone(8, 880, 20); delay(100); } void loop(){ delay(10); }`, 200); assert.strictEqual(later.sim.sounds().z, 0);
});
test("full Lab 2 style sketch prints a valid CSV line from DHT + PIR + LDR", () => {
  const c = mk([part("d", "dht11", { temp: 24, hum: 52 }), part("p", "pir", { motion: true }), part("ldr", "ldr", { light: 60 }), part("r", "res", { ohms: 10000 })],
    [W("d.VCC", "uno.5V"), W("d.GND", "uno.GND2"), W("d.DATA", "uno.D2"), W("p.VCC", "uno.5V"), W("p.GND", "uno.GND3"), W("p.OUT", "uno.D3"), W("uno.5V", "ldr.1"), W("ldr.2", "uno.A0"), W("uno.A0", "r.1"), W("r.2", "uno.GND1")]);
  const { lines } = go(c, `#include <DHT.h>
    #define DHTPIN 2
    DHT dht(DHTPIN, DHT11);
    void setup(){ Serial.begin(9600); dht.begin(); pinMode(3, INPUT); }
    void loop(){ float t = dht.readTemperature(); float h = dht.readHumidity(); int m = digitalRead(3); int l = analogRead(A0);
      Serial.print(t); Serial.print(","); Serial.print(h); Serial.print(","); Serial.print(m); Serial.print(","); Serial.println(l); delay(1000); }`, 3500);
  assert.ok(lines.length >= 3); const p = lines[0].split(","); assert.strictEqual(p.length, 4);
  assert.deepStrictEqual(p.slice(0, 3), ["24.00", "52.00", "1"]); assert.ok(Number(p[3]) > 150 && Number(p[3]) < 350, "ldr " + p[3]);
});
