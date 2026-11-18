"use strict";
const test = require("node:test"), assert = require("node:assert");
const { Machine, compile, CErr } = require("../internal/web/static/cint.js");

function mkhw(o = {}) {
  const log = []; let now = () => 0;
  return { log, setClock(f) { now = f; }, pinMode(p, m) { log.push(["mode", p, m]); }, digitalWrite(p, v) { log.push(["dw", p, v, Math.round(now() / 1000)]); },
    digitalRead: p => (o.dr ? o.dr(p) : 0), analogRead: p => (o.ar ? o.ar(p) : 512), analogWrite(p, d) { log.push(["aw", p, d]); },
    tone(p, f, d) { log.push(["tone", p, f, d]); }, noTone(p) { log.push(["notone", p]); }, pulseIn: () => ({ us: 0, advance: 0 }), dht: p => (o.dht ? o.dht(p) : { t: 24, h: 50 }) };
}
function run(src, ms = 100, hw = mkhw()) {
  let out = ""; const vm = new Machine(src, hw, { out: s => { out += s; } });
  vm.run(ms * 1000); return { out, vm, hw };
}
const lines = s => s.trim().split("\n");

test("blink toggles with correct timing", () => {
  const { hw } = run(`void setup(){ pinMode(13, OUTPUT); } void loop(){ digitalWrite(13,HIGH); delay(500); digitalWrite(13,LOW); delay(500); }`, 2100);
  const w = hw.log.filter(e => e[0] === "dw");
  assert.deepStrictEqual(w.slice(0, 5).map(e => [e[2], e[3]]), [[1, 0], [0, 500], [1, 1000], [0, 1500], [1, 2000]]);
});

test("Serial formatting follows Arduino rules", () => {
  const { out } = run(`void setup(){ Serial.begin(9600);
    float f = 3.14159; int i = 42; char c = 'A'; String s = "hi"; byte b = 255;
    Serial.println(f); Serial.println(f, 4); Serial.println(i); Serial.println(c); Serial.println(s);
    Serial.println(i, HEX); Serial.println(i, BIN); Serial.println(b); Serial.print("x="); Serial.print(1); Serial.println();
    Serial.println(true); Serial.println(2.0/3); Serial.println(10/4); Serial.println(10/4.0); Serial.println(-7/2); Serial.println(-7%3); }
    void loop(){ delay(1000); }`, 10);
  assert.deepStrictEqual(lines(out), ["3.14", "3.1416", "42", "A", "hi", "2A", "101010", "255", "x=1", "1", "0.67", "2", "2.50", "-3", "-1"]);
});

test("integer widths wrap like an Uno", () => {
  const { out } = run(`void setup(){ int a = 32767; a = a + 1; Serial.println(a);
    unsigned int u = 0; u = u - 1; Serial.println(u); byte b = 250; b += 10; Serial.println(b);
    long big = 100000; Serial.println(big); unsigned long m = 4294967295; m++; Serial.println(m);
    int d = 7.9; Serial.println(d); float x = 7; Serial.println(x / 2); int q = 7; Serial.println(q / 2); char ch = 200; Serial.println((int)ch); }
    void loop(){ delay(10); }`, 5);
  assert.deepStrictEqual(lines(out), ["-32768", "65535", "4", "100000", "0", "7", "3.50", "3", "-56"]);
});

test("functions, recursion and delay inside functions and expressions", () => {
  const { out, hw } = run(`int twice(int x){ delay(10); return x * 2; }
    int fact(int n){ if (n <= 1) return 1; return n * fact(n - 1); }
    void pulse(int n){ for (int i = 0; i < n; i++){ digitalWrite(5, HIGH); delay(100); digitalWrite(5, LOW); delay(100); } }
    void setup(){ Serial.println(twice(4) + twice(5)); Serial.println(fact(5)); pulse(3); Serial.println(millis()); }
    void loop(){ delay(1000); }`, 2000);
  assert.deepStrictEqual(lines(out), ["18", "120", "620"].slice(0, 2).concat([String(20 + 600 + 0)]));
  assert.strictEqual(hw.log.filter(e => e[0] === "dw").length, 6);
});

test("arrays, loops, break/continue, switch, ternary, #define, compound ops", () => {
  const { out } = run(`#define N 5
    #define LIMIT 3
    const int K = 2;
    int sq[N];
    int primes[] = {2, 3, 5, 7};
    void setup(){
      for (int i = 0; i < N; i++) sq[i] = i * i;
      int s = 0; for (int i = 0; i < N; i++) s += sq[i]; Serial.println(s);
      int w = 0; while (true){ w++; if (w < LIMIT) continue; if (w > 6) break; }
      Serial.println(w);
      int t = 0; for (int i = 0; i < 4; i++) { switch (primes[i]) { case 2: t += 1; break; case 3: t += 10; case 5: t += 100; break; default: t += 1000; } }
      Serial.println(t);
      Serial.println(K > 1 ? "big" : "small");
      int x = 5; x <<= 2; x |= 1; x ^= 3; x %= 7; Serial.println(x);
      int y = 3; Serial.println(y++ + ++y); Serial.println(y);
      do { y--; } while (y > 0); Serial.println(y);
    } void loop(){ delay(10); }`, 5);
  assert.deepStrictEqual(lines(out), ["30", "7", "1211", "big", "1", "8", "5", "0"]);
});

test("timing builtins, map, constrain, random, min/max/abs", () => {
  const { out } = run(`void setup(){ delay(250); Serial.println(millis()); Serial.println(map(512, 0, 1023, 0, 255)); Serial.println(map(-5, 0, 10, 100, 200));
    Serial.println(constrain(15, 0, 10)); Serial.println(min(3, 9)); Serial.println(max(2.5, 1)); Serial.println(abs(-4)); Serial.println(pow(2, 10)); Serial.println(sqrt(16));
    randomSeed(5); long r1 = random(100); randomSeed(5); long r2 = random(100); Serial.println(r1 == r2); Serial.println(random(10, 20) >= 10); }
    void loop(){ delay(10); }`, 500);
  assert.deepStrictEqual(lines(out), ["250", "127", "50", "10", "3", "2.50", "4", "1024.00", "4.00", "1", "1"]);
});

test("String helpers and concatenation", () => {
  const { out } = run(`void setup(){ String a = "Temp: "; int t = 23; String b = a + t + "C"; Serial.println(b); Serial.println(b.length()); String n = "42"; Serial.println(n.toInt() + 1); float f = 2.5; Serial.println(String("v=") + f); }
    void loop(){ delay(10); }`, 5);
  assert.deepStrictEqual(lines(out), ["Temp: 23C", "9", "43", "v=2.50"]);
});

test("DHT object reads through the hardware layer", () => {
  let asked = [];
  const hw = mkhw({ dht: p => { asked.push(p); return { t: 21.5, h: 40 }; } });
  const { out } = run(`#include <DHT.h>
    #define DHTPIN 2
    #define DHTTYPE DHT11
    DHT dht(DHTPIN, DHTTYPE);
    void setup(){ Serial.begin(9600); dht.begin(); }
    void loop(){ float t = dht.readTemperature(); float h = dht.readHumidity(); Serial.print(t); Serial.print(","); Serial.println(h); delay(1000); }`, 2500, hw);
  assert.deepStrictEqual(lines(out).slice(0, 2), ["21.50,40.00", "21.50,40.00"]); assert.strictEqual(asked[0], 2);
  const r = run(`DHT dht(2, DHT11); void setup(){} void loop(){ Serial.println(dht.readTemperature()); Serial.println(isnan(dht.readHumidity())); delay(1000); }`, 10, mkhw({ dht: () => null }));
  assert.deepStrictEqual(lines(r.out), ["nan", "1"]);
});

test("pins: analogRead maps A0..A5, digitalRead, tone, analogWrite", () => {
  const hw = mkhw({ ar: p => p * 100, dr: p => (p === 7 ? 1 : 0) });
  const { out } = run(`void setup(){ pinMode(7, INPUT_PULLUP); Serial.println(analogRead(A2)); Serial.println(analogRead(1)); Serial.println(digitalRead(7)); tone(8, 440, 100); analogWrite(9, 128); noTone(8); } void loop(){ delay(1000); }`, 5, hw);
  assert.deepStrictEqual(lines(out), ["200", "100", "1"]);
  assert.deepStrictEqual(hw.log.filter(e => e[0] === "tone" || e[0] === "aw" || e[0] === "notone").map(e => e[0]), ["tone", "aw", "notone"]);
});

test("compile errors carry Arduino-style messages and line numbers", () => {
  const bad = (src, re, line) => { try { compile(src); assert.fail("should not compile: " + src); } catch (e) { assert.ok(e instanceof CErr, String(e)); assert.match(e.message, re); if (line) assert.strictEqual(e.line, line); } };
  bad("void setup(){\n int x = y;\n}\nvoid loop(){}", /'y' was not declared/, 2);
  bad("void setup(){\n int x = 3\n}\nvoid loop(){}", /expected ';'/, 2);
  bad("void setup(){}", /loop\(\)/);
  bad("void loop(){}", /setup\(\)/);
  bad("void setup(){ digitalWrite(13); } void loop(){}", /too few arguments to function 'digitalWrite'/);
  bad("void setup(){ foo(); } void loop(){}", /'foo' was not declared/);
  bad("void setup(){ int a; int a; } void loop(){}", /redeclaration of 'a'/);
  bad("#define SQ(x) ((x)*(x))\nvoid setup(){} void loop(){}", /function-like macros/);
  bad("void setup(){ int *p; } void loop(){}", /pointers/);
  bad("Servo s; void setup(){} void loop(){}", /Servo is not supported/);
  bad("void setup(){ HIGH = 3; } void loop(){}", /cannot assign to constant/);
});

test("runtime safety: busy loops do not hang, division by zero is reported", () => {
  const t0 = Date.now();
  const { vm } = run(`void setup(){} void loop(){ int x = 0; while (true) { x++; } }`, 1000);
  assert.ok(Date.now() - t0 < 2000, "busy loop must yield to the scheduler");
  assert.ok(vm.micros > 0);
  assert.throws(() => run(`void setup(){ int z = 0; Serial.println(5 / z); } void loop(){}`, 10), /division by zero/);
  assert.throws(() => run(`int f(int n){ return f(n + 1); } void setup(){ f(0); } void loop(){}`, 10), /stack overflow/);
});

test("virtual time advances only as much as requested", () => {
  const hw = mkhw(); let out = "";
  const vm = new Machine(`void setup(){} void loop(){ Serial.println(millis()); delay(100); }`, hw, { out: s => { out += s; } });
  vm.run(350 * 1000); const a = lines(out).length; vm.run(300 * 1000); const b = lines(out).length;
  assert.ok(a >= 3 && a <= 5, "after 350ms about 4 prints, got " + a); assert.ok(b > a);
  assert.ok(Math.abs(vm.micros - 650000) < 1000);
});
