"use strict";
// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.
// Workbenches for each lab. Each PANELS[n] returns {html, init}.
const card = (title, body, extra = "") => `<div class="card ${extra}"><h3>${title}</h3>${body}</div>`;
const code = t => `<pre>${esc(t)}</pre>`;
const PINS = ["", "D2", "D3", "D4", "D5", "D6", "D7", "D8", "A0", "A1", "A2"];
const cm = c => `<table><tr><th></th><th class="num">pred 0</th><th class="num">pred 1</th></tr><tr><th>true 0</th><td class="num">${c[0][0]}</td><td class="num">${c[0][1]}</td></tr><tr><th>true 1</th><td class="num">${c[1][0]}</td><td class="num">${c[1][1]}</td></tr></table>`;
const repTable = r => table(["class", "precision", "recall", "f1", "support"], r.class.map((c, i) => [i, fx(c.precision), fx(c.recall), fx(c.f1), c.support]), [1, 2, 3, 4]);
const sleep = ms => new Promise(r => setTimeout(r, ms));
const store = { get: k => { try { return localStorage.getItem("aiot." + k) || ""; } catch { return ""; } }, set: (k, v) => { try { localStorage.setItem("aiot." + k, v); } catch { } } };

// ------------------------------------------------------------ Lab 1
PANELS[1] = () => {
  const items = ["Raspberry Pi 4/5 board + power supply", "microSD card (16 GB+)", "Arduino Uno or Nano", "ESP32 development board", "Breadboard", "Jumper wires", "USB data cables", "DHT11 sensor", "PIR motion sensor", "LDR + 10 kΩ resistor"];
  const asset = ["Raspberry Pi", "Arduino", "ESP32"];
  return {
    html: card("Kit inventory", `<table>${items.map((t, i) => `<tr><td style="width:24px"><input type="checkbox" class="inv" ${store.get("inv" + i) ? "checked" : ""} data-i="${i}"></td><td>${esc(t)}</td></tr>`).join("")}</table>
      <div class="grid3" style="margin-top:8px">${asset.map((a, i) => fld("asset" + i, a + " serial / asset tag", store.get("asset" + i), "text")).join("")}</div>
      <div class="row" style="margin-top:8px"><button class="btn" data-act="l1inv">Confirm inventory</button><span class="muted tiny">Tags are stored only in your browser.</span></div>`) +
      card("Verify the workstation (simulated)", `<p class="muted small">Runs the manual's verification commands against the virtual workstation and virtual gateway <span class="mono">raspberrypi.local</span>.</p>
      <button class="btn" data-act="l1verify">Run verification</button><pre id="l1out">$ _</pre>
      <details><summary>Hint for the SSH-key viva question</summary>${code("ssh-keygen -t ed25519\nssh-copy-id user@raspberrypi.local\nssh user@raspberrypi.local   # no password prompt\n# then in /etc/ssh/sshd_config: PasswordAuthentication no")}</details>`),
    init() { $$(".inv").forEach(c => c.onchange = () => store.set("inv" + c.dataset.i, c.checked ? "1" : "")); asset.forEach((_, i) => $("#asset" + i).oninput = e => store.set("asset" + i, e.target.value)); }
  };
};
Object.assign(A, {
  l1inv: () => { const all = $$(".inv").every(c => c.checked), tags = [0, 1, 2].every(i => val("asset" + i).trim()); if (!all || !tags) return toast("Tick every item and enter the three serial numbers / asset tags.", true); mark("inventory"); toast("Inventory confirmed"); },
  l1verify: async () => {
    const o = $("#l1out"); o.textContent = "";
    const lines = ["$ python3 --version", "Python 3.x (simulated)", "$ source ~/aiot-env/bin/activate && pip --version", "pip (simulated) from ~/aiot-env", "$ git --version", "git version 2.x (simulated)", "$ ping -c 4 raspberrypi.local", "PING raspberrypi.local (192.168.1.50): 56 data bytes", "64 bytes from 192.168.1.50: icmp_seq=0 time=2.1 ms", "64 bytes from 192.168.1.50: icmp_seq=1 time=1.9 ms", "64 bytes from 192.168.1.50: icmp_seq=2 time=2.3 ms", "64 bytes from 192.168.1.50: icmp_seq=3 time=2.0 ms", "--- 4 packets transmitted, 4 received, 0% packet loss ---", "", "[simulated output – real versions come from your own machine]"];
    for (const l of lines) { o.textContent += l + "\n"; await sleep(90); }
    mark("verify");
  }
});

// ------------------------------------------------------------ Lab 2
const SKETCH_CODE = {
  blink: "void setup(){ pinMode(LED_BUILTIN, OUTPUT); }\nvoid loop(){ digitalWrite(LED_BUILTIN,HIGH); delay(500); digitalWrite(LED_BUILTIN,LOW); delay(500); }",
  dht: "#include <DHT.h>\n#define DHTPIN 2\n#define DHTTYPE DHT11\nDHT dht(DHTPIN, DHTTYPE);\nvoid setup() { Serial.begin(9600); dht.begin(); }\nvoid loop() {\n  float h = dht.readHumidity();\n  float t = dht.readTemperature();\n  Serial.print(\"Temp: \"); Serial.print(t);\n  Serial.print(\" C, Humidity: \"); Serial.println(h);\n  delay(1000);\n}",
  dht_pir: "// DHT11 + PIR\n#define PIRPIN 3\nvoid setup(){ Serial.begin(9600); dht.begin(); pinMode(PIRPIN, INPUT); }\nvoid loop(){\n  ... read t, h ...\n  int motion = digitalRead(PIRPIN);\n  Serial.println(motion ? \"Motion detected\" : \"No motion\");\n  delay(1000);\n}",
  combined: "void loop() {\n  float h = dht.readHumidity();\n  float t = dht.readTemperature();\n  int motion = digitalRead(PIRPIN);\n  int lightRaw = analogRead(A0);\n  Serial.print(t); Serial.print(\",\");\n  Serial.print(h); Serial.print(\",\");\n  Serial.print(motion); Serial.print(\",\");\n  Serial.println(lightRaw);\n  delay(1000);\n}",
  combined_extra: "// combined + HC-SR04 distance as a fifth field\n  ...\n  Serial.print(lightRaw); Serial.print(\",\");\n  Serial.println(distanceCm);"
};
PANELS[2] = () => ({
  html: card("Build it visually", `<p class="small">Prefer to see the breadboard? The <b>Circuit Lab</b> lets you drag the DHT11, PIR and LDR, draw the wires yourself and run the real sketch.</p><button class="btn" data-act="circ2">⚡ Open the Lab 2 wiring exercise</button>`) + card("Wiring table", `<p class="muted small">Pick the microcontroller pin for each sensor, as you would on the breadboard.</p><div id="wiring"></div>
    <div class="row" style="margin-top:8px"><button class="btn" data-act="l2wire">Apply wiring &amp; check</button></div><div id="wcheck"></div>`) +
    card("Sketch &amp; live reading", `<div class="row">${sel("l2sketch", "Sketch to upload", (S.sketches || []).map(s => [s.ID, s.Name]), (S.board || {}).sketch)}<button class="btn" data-act="l2up">Upload &amp; run</button></div><div id="l2code"></div>
      <div id="l2ldr" class="grid3"></div><p class="muted small">Tip: if the Serial Monitor baud (right panel) differs from <span class="mono">Serial.begin()</span>, you will see garbage – exactly as on real hardware.</p>`),
  init() {
    const w = S.wiring || {}, rows = [["dht", "DHT11 DATA"], ["pir", "PIR OUT"], ["ldr", "LDR divider mid-point (to analog pin)"], ["extra", "Extra sensor (student task)"]];
    setHTML("wiring", `<div class="grid2">${rows.map(([k, l]) => sel("w_" + k, l, PINS.map(p => [p, p || "(not connected)"]), w[k] || "")).join("")}</div>`);
    const show = () => { const sk = val("l2sketch"); setHTML("l2code", code(SKETCH_CODE[sk] || "")); }; $("#l2sketch").onchange = show; show();
    $("#l2sketch").value = S.board.sketch; show();
    const draw = () => { const v = DASH.last; setHTML("l2ldr", v ? kpi(v.l, "LDR raw ADC (0–1023)") + kpi(v.l < 300 ? "DARK" : v.l < 700 ? "DIM" : "BRIGHT", "light level") + kpi((v.l * 5 / 1023).toFixed(2) + " V", "A0 voltage") : `<span class="muted small">Start the board with a combined sketch to see LDR values.</span>`); };
    HOOK.serial = draw; draw();
    if (S.wiringCheck) A.l2render(S.wiringCheck);
  }
});
Object.assign(A, {
  l2render: chk => setHTML("wcheck", table(["Sensor pin", "Expected", "Actual", "Status"], chk.map(c => [esc(c.sensor), esc(c.expected), esc(c.actual || "–"), badge(c.status, c.status === "OK" ? "ok" : c.status === "NOT USED" ? "info" : "bad")]))),
  l2wire: async () => { const r = await api("/api/wiring", { dht: val("w_dht"), pir: val("w_pir"), ldr: val("w_ldr"), extra: val("w_extra") }); A.l2render(r.check); await refresh(); toast(r.ok ? "Wiring matches the manual" : "Wiring differs from the manual – readings will fail", !r.ok); },
  l2up: async () => { await api("/api/board", { sketch: val("l2sketch"), running: true }); await refresh(); }
});

// ------------------------------------------------------------ Lab 3
PANELS[3] = () => ({
  html: card("Logger (log_sensors.py)", `<div class="row"><button class="btn" data-act="l3start">● Start logging</button><button class="btn ghost" data-act="l3stop">■ Stop &amp; save</button><button class="btn ghost" data-act="l3clear">Clear</button><button class="btn ghost" data-act="l3sample">Use instructor sample dataset (600 rows)</button><button class="btn ghost" data-act="l3dl">Download CSV</button></div>
    <div id="l3stat" class="note"></div>${code('if len(parts) == 4:\n    writer.writerow([time.time()] + parts)')}
    <p class="muted small">Only lines with the expected number of fields are logged. Set <b>Logger fields</b> to 5 if your sketch prints an extra sensor (Lab 2 task).</p>`) +
    card("Inspect the dataset (df.head / info / describe)", `<button class="btn" data-act="l3show">Show dataset</button><div id="l3ds"></div>`) +
    card("Clean &amp; smooth", `<div class="grid3">${sel("l3mode", "Missing / bad rows", [["dropna", "dropna()"], ["interpolate", "interpolate()"]], "dropna")}${fld("l3win", "Rolling window", 5)}<label class="inline" style="margin-top:18px"><input type="checkbox" id="l3med" checked> Compare median filter (task)</label></div>
    <button class="btn" data-act="l3pre" style="margin-top:8px">Run preprocessing</button><div id="l3out"></div>`),
  init() { const f = () => setHTML("l3stat", `Rows in sensor_log.csv: <b>${S.rows || 0}</b> ${S.dataSource ? "· source: " + esc(S.dataSource) : ""} ${S.logging ? badge("recording", "ok") : ""}`); HOOK.logger = HOOK.state = f; f(); }
});
Object.assign(A, {
  l3start: async () => { if (!(S.board || {}).running) await api("/api/board", { running: true }); await api("/api/logger", { action: "start" }); await refresh(); },
  l3stop: async () => { await api("/api/logger", { action: "stop" }); await refresh(); },
  l3clear: async () => { await api("/api/logger", { action: "clear" }); await refresh(); },
  l3sample: async () => { const r = await api("/api/logger/sample", { n: 600 }); await refresh(); toast(r.rows + " sample rows generated"); },
  l3dl: () => { window.location.href = "/api/dataset.csv"; },
  l3show: async () => {
    const d = await api("/api/dataset"); if (!d.rows) return setHTML("l3ds", `<div class="note warn">No rows yet.</div>`);
    const names = ["timestamp", "temperature", "humidity", "motion", "light", "distance"].slice(0, d.fields + 1);
    setHTML("l3ds", `<p class="small">${d.rows} rows × ${d.fields + 1} columns · <span class="mono">${esc(d.path)}</span></p><h4>df.head()</h4>${table(names, d.head.map(r => r.map(esc)))}<h4>df.tail()</h4>${table(names, d.tail.map(r => r.map(esc)))}
      <h4>df.info() / df.describe()</h4>${table(["column", "dtype", "non-null", "mean", "std", "min", "25%", "50%", "75%", "max"], d.columns.map(c => [c.name, c.dtype, c.nonNull + " / " + d.rows, fx(c.mean, 2), fx(c.std, 2), fx(c.min, 2), fx(c.q25, 2), fx(c.q50, 2), fx(c.q75, 2), fx(c.max, 2)]), [2, 3, 4, 5, 6, 7, 8, 9])}
      ${d.columns.some(c => c.dtype === "object") ? `<div class="note warn">A column has dtype <b>object</b>: at least one line contained non-numeric bytes. It must be cleaned before modelling.</div>` : ""}`);
  },
  l3pre: async () => {
    const r = await api("/api/preprocess", { mode: val("l3mode"), window: num("l3win"), median: val("l3med") });
    setHTML("l3out", `<div class="grid3" style="margin:8px 0">${kpi(r.total, "rows logged")}${kpi(r.kept, "rows kept")}${kpi(r.nanRows + " / " + r.nonNumeric + " / " + r.spikes, "NaN / non-numeric / spikes")}</div>
      <canvas id="c3a"></canvas><canvas id="c3b" style="margin-top:8px"></canvas>
      <h4>Rows that needed attention (for your reflection)</h4>${r.problems.length ? table(["#", "raw line", "issue", "likely cause"], r.problems.map(p => [p.index, `<span class="mono">${esc(p.raw)}</span>`, esc(p.issue), esc(p.cause)])) : "<p class='muted small'>No problem rows.</p>"}
      <p class="muted small">Saved: <span class="mono">${esc(r.file)}</span></p>`);
    chart($("#c3a"), [{ name: "Raw", data: r.temp }, { name: "Smoothed (window " + r.window + ")", data: r.smooth }], { xlabel: "sample number (temperature °C)" });
    chart($("#c3b"), [{ name: "Light raw", data: r.light, color: "#8fa7b8" }, { name: "Median(3)", data: r.median }, { name: "Rolling mean(3)", data: r.lightRoll }], { xlabel: "sample number (light, ADC)" });
  }
});

// ------------------------------------------------------------ Lab 4
PANELS[4] = () => ({
  html: card("Live dashboard (Node-RED style)", `<div class="row"><button class="btn small" data-act="l4start">▶ Start board with combined sketch</button><label class="inline">Alert when temperature &gt; <input id="l4thr" type="number" value="30" style="width:70px"></label></div>
    <div id="l4banner" class="banner" style="margin:8px 0">NORMAL</div>
    <div class="gauges"><canvas id="g4t"></canvas><canvas id="g4l"></canvas></div>
    <canvas id="c4h" class="short" style="margin-top:8px"></canvas>
    <div class="row" style="margin-top:8px"><span class="led" id="l4led"></span><b id="l4mot">Motion: none</b></div>`) +
    card("function node (parse the serial line)", code('var parts = msg.payload.split(",");\nreturn {\n  payload: {\n    temperature: parseFloat(parts[0]),\n    humidity: parseFloat(parts[1]),\n    motion: parseInt(parts[2]),\n    light: parseInt(parts[3])\n  }\n};') + `<p class="muted small">Lines that fail to parse (nan, corrupted) are ignored by the dashboard, as your function node should do.</p>`),
  init() {
    let alerted = false;
    const draw = () => {
      const v = DASH.last, thr = num("l4thr");
      gauge($("#g4t"), v ? v.t : null, 0, 50, "Temperature", " °C", thr); gauge($("#g4l"), v ? v.l : null, 0, 1023, "Light (ADC)", "");
      chart($("#c4h"), [{ name: "Humidity %", data: DASH.h }]);
      const al = v && v.t > thr; const b = $("#l4banner"); if (b) { b.textContent = al ? "ALERT — temperature above " + thr + " °C" : "NORMAL"; b.className = "banner" + (al ? " alert" : ""); }
      const led = $("#l4led"); if (led) led.className = "led" + (v && v.m ? " on" : ""); const mo = $("#l4mot"); if (mo) mo.textContent = "Motion: " + (v && v.m ? "DETECTED" : "none");
      if (al && !alerted) { alerted = true; mark("alert_set"); } if (DASH.n === 10) mark("dash_live");
    };
    HOOK.serial = () => requestAnimationFrame(draw); HOOK.state = draw; draw();
  }
});
A.l4start = async () => { await api("/api/board", { sketch: "combined", running: true }); await refresh(); };

// ------------------------------------------------------------ Lab 5
PANELS[5] = () => ({
  html: card("Broker (Mosquitto-compatible)", `<div id="l5b"></div>`) +
    card("mosquitto_sub / mosquitto_pub test client", `<div class="grid3">${fld("l5u", "User", "", "text")}${fld("l5p", "Password", "", "password")}<label class="inline" style="margin-top:18px"><input type="checkbox" id="l5auth"> Send credentials</label></div>
      <div class="row" style="margin-top:8px"><button class="btn" data-act="l5rt">Pub/Sub round trip</button><button class="btn ghost" data-act="l5conn">Connect only (test authentication)</button></div><pre id="l5tr">Run a test to see the transcript.</pre>`) +
    card("publisher.py (serial → MQTT every N s)", `<div id="l5pub"></div>`) +
    card("MQTT Explorer — topic tree", `<div id="l5tree" style="max-height:260px;overflow:auto"></div>`),
  async init() {
    const drawB = () => {
      const m = S.mqtt || {};
      setHTML("l5b", `<div class="grid3">${kpi(m.port, "port")}${kpi(m.clients, "network clients")}${kpi((m.users || []).length, "users")}</div>${m.note ? `<div class="note warn">${esc(m.note)}</div>` : ""}
        <div class="row" style="margin:8px 0"><label class="inline"><input type="checkbox" id="l5lan" ${m.lan ? "checked" : ""}> Listen on the local network (0.0.0.0)</label><label class="inline"><input type="checkbox" id="l5anon" ${m.allowAnonymous ? "checked" : ""}> allow_anonymous</label></div>
        <div class="grid3">${fld("l5nu", "New user (mosquitto_passwd)", "", "text")}${fld("l5np", "Password", "", "password")}<button class="btn small" style="margin-top:18px" data-act="l5adduser">Add user</button></div>
        <div class="muted small">Users: ${(m.users || []).map(u => `<span class="badge b-info">${esc(u)}</span> <a href="#" data-act="l5rmuser" data-u="${esc(u)}">remove</a>`).join(" ") || "none"}</div>${code(m.conf || "")}`);
      ["l5lan", "l5anon"].forEach(id => $("#" + id).onchange = () => run(async () => { await api("/api/mqtt/config", { lan: val("l5lan"), allowAnonymous: val("l5anon") }); await refresh(); drawB(); }));
      const b = m.bridge || {};
      setHTML("l5pub", `<div class="grid3">${sel("l5int", "Interval (s of sim time)", [1, 2, 5, 10], b.interval || 2)}${sel("l5qos", "QoS", [0, 1], b.qos || 0)}<label class="inline" style="margin-top:18px"><input type="checkbox" id="l5pauth" ${b.useAuth ? "checked" : ""}> Authenticate</label></div>
        <div class="grid2">${fld("l5pu", "User", b.user || "", "text")}${fld("l5pp", "Password", "", "password")}</div>
        <div class="row" style="margin-top:8px"><button class="btn" data-act="l5pubon">${b.enabled ? "Restart publisher" : "▶ Start publisher"}</button><button class="btn ghost" data-act="l5puboff">■ Stop</button><span class="small">${b.enabled ? badge("running", "ok") : badge("stopped", "info")} sent ${b.sent || 0}</span></div>
        ${b.error ? `<div class="note bad">${esc(b.error)}</div>` : ""}${code('client.publish("aiot/lab5/temperature", parts[0])\nclient.publish("aiot/lab5/humidity", parts[1])\nclient.publish("aiot/lab5/motion", parts[2])\nclient.publish("aiot/lab5/light", parts[3])')}`);
    };
    const drawT = () => { const ts = Object.values(TOPICS).sort((a, b) => a.topic.localeCompare(b.topic)); setHTML("l5tree", ts.length ? table(["topic", "last value", "msgs"], ts.map(t => [`<span class="mono">${esc(t.topic)}</span>`, `<span class="mono">${esc(t.payload)}</span>`, t.count]), [2]) : `<span class="muted small">No messages yet – start the board and the publisher.</span>`); };
    HOOK.mqttstate = drawB; HOOK.mqtt = (() => { let q = 0; return () => { if (!q) q = setTimeout(() => { q = 0; drawT(); }, 400); }; })(); HOOK.state = drawB;
    (await api("/api/mqtt/topics")).forEach(t => TOPICS[t.topic] = t); drawB(); drawT();
  }
});
Object.assign(A, {
  l5adduser: async () => { await api("/api/mqtt/config", { addUser: { U: val("l5nu"), P: val("l5np") } }); await refresh(); toast("User added. Now untick allow_anonymous."); },
  l5rmuser: async el => { await api("/api/mqtt/config", { removeUser: el.dataset.u }); await refresh(); },
  l5rt: async () => { const r = await api("/api/mqtt/test", { action: "roundtrip", useAuth: val("l5auth"), user: val("l5u"), pass: val("l5p") }); setHTML("l5tr", esc(r.transcript.join("\n"))); },
  l5conn: async () => { const r = await api("/api/mqtt/test", { action: "connect", useAuth: val("l5auth"), user: val("l5u"), pass: val("l5p") }); setHTML("l5tr", esc(r.transcript.join("\n"))); await refresh(); },
  l5pubon: async () => { if (!(S.board || {}).running) await api("/api/board", { running: true }); S.mqtt = await api("/api/mqtt/bridge", { enabled: true, interval: num("l5int"), qos: num("l5qos"), useAuth: val("l5pauth"), user: val("l5pu"), pass: val("l5pp") }); HOOK.mqttstate(S.mqtt); },
  l5puboff: async () => { S.mqtt = await api("/api/mqtt/bridge", { enabled: false }); HOOK.mqttstate(S.mqtt); }
});

// ------------------------------------------------------------ Lab 6
PANELS[6] = () => ({
  html: card("Train &amp; evaluate (comfort classifier)", `<p class="small">Label: <span class="mono">comfortable = 20 ≤ temperature ≤ 28 and humidity ≤ 60</span>. Split 75 % / 25 %, features: temperature, humidity, light.</p>
    <div class="grid3">${sel("l6algo", "Algorithm", [["dt", "Decision tree"], ["rf", "Random forest"], ["knn", "k-nearest neighbours"]], "dt")}${fld("l6depth", "max depth (0 = none)", 0)}${fld("l6k", "k (k-NN)", 5)}</div>
    <div class="row" style="margin-top:6px"><label class="inline"><input type="checkbox" id="l6std"> standardise features (k-NN)</label><button class="btn" data-act="l6train">Train &amp; evaluate</button></div><div id="l6out"></div>`)
});
A.l6train = async () => {
  const r = await api("/api/ml/train", { algo: val("l6algo"), maxDepth: num("l6depth"), k: num("l6k"), standardise: val("l6std") });
  const rp = r.report;
  setHTML("l6out", `<div class="grid3" style="margin:8px 0">${kpi(pct(rp.accuracy), "accuracy")}${kpi(r.trainN + " / " + r.testN, "train / test rows")}${kpi((r.labelCounts["1"]) + " / " + r.labelCounts["0"], "comfortable / not")}</div>
    <div class="grid2"><div><h4>Confusion matrix</h4>${cm(rp.confusion)}</div><div><h4>Classification report</h4>${repTable(rp)}</div></div>
    ${r.importances ? `<h4>Feature importances</h4><canvas id="c6" class="short"></canvas>` : ""}
    ${r.rules ? `<h4>Learned rules (depth ${r.depth}, ${r.leaves} leaves)</h4>${code(r.rules)}` : ""}
    ${r.comparison ? `<div class="note">Decision tree ${pct(r.comparison.dtAccuracy)} vs ${esc(r.algo)} ${pct(r.comparison.thisAccuracy)} → better: <b>${esc(r.comparison.better)}</b>. Write your hypothesis for the difference (e.g. feature scale for k-NN, rule-based labels suit axis-aligned splits).</div>` : ""}
    <p class="muted small">Saved: <span class="mono">${esc(r.file)}</span></p>`);
  if (r.importances) bars($("#c6"), r.features, r.importances, { fmt: v => fx(v, 3), max: 1 });
};

// ------------------------------------------------------------ Lab 7
PANELS[7] = () => ({
  html: card("Keras-style network: Input(3) → Dense(8, relu) → Dense(1, sigmoid)", `<div class="grid3">${fld("l7h", "Hidden units", 8)}${fld("l7e", "Epochs", 30)}${fld("l7lr", "Learning rate", 0.01, "number", 'step="0.001"')}</div>
    <div class="row" style="margin-top:6px"><label class="inline"><input type="checkbox" id="l7n" checked> normalise inputs</label><button class="btn" data-act="l7train">Train</button></div><div id="l7out"></div>`) +
    card("Convert &amp; verify", `<div class="row"><button class="btn" data-act="l7conv">Convert to TFLite</button><button class="btn" data-act="l7q">Apply quantisation &amp; convert</button><label class="inline">Test sample # <input id="l7i" type="number" value="0" style="width:70px"></label><button class="btn ghost" data-act="l7ver">Run interpreter &amp; compare</button></div><div id="l7conv"></div>`)
});
const L7 = { sizes: {} };
Object.assign(A, {
  l7train: async () => {
    const r = await api("/api/nn/train", { hidden: num("l7h"), epochs: num("l7e"), lr: num("l7lr"), normalise: val("l7n") });
    setHTML("l7out", `<p class="mono small">${esc(r.summary)}</p><div class="grid3" style="margin:8px 0">${kpi(pct(r.testAcc), "test accuracy")}${kpi(r.params, "parameters")}${kpi(r.kerasBytes + " B", "Keras file (est.)")}</div><canvas id="c7"></canvas>`);
    chart($("#c7"), [{ name: "train acc", data: r.history.map(h => h.accuracy) }, { name: "val acc", data: r.history.map(h => h.valAccuracy) }, { name: "val loss", data: r.history.map(h => h.valLoss), color: "#f87171" }], { xlabel: "epoch" });
  },
  l7conv: async () => { const r = await api("/api/nn/convert", { quantise: false }); L7.sizes = r; A.l7sizes(); },
  l7q: async () => { const r = await api("/api/nn/convert", { quantise: true }); L7.sizes = r; L7.q = r; A.l7sizes(); },
  l7sizes: () => { const r = L7.sizes; setHTML("l7conv", `<h4>Size table (estimated)</h4>${table(["Artefact", "Bytes"], [["Keras model (.keras)", r.keras], ["TFLite float32", r.tflite], ["TFLite int8-quantised", r.quant]], [1])}${L7.q ? `<p class="small">Quantised accuracy ${pct(L7.q.accuracy)}, max weight error ${fx(L7.q.maxErr, 5)}.</p><div class="note">${esc(L7.q.note)}</div>` : ""}`); },
  l7ver: async () => { const r = await api("/api/nn/verify", { index: num("l7i") }); setHTML("l7conv", (document.getElementById("l7conv").innerHTML) + `<h4>Interpreter vs Keras on sample ${num("l7i")}</h4>${table(["input (T, H, light)", "TFLite output", "Keras output", "difference", "equivalent", "µs (TFLite / Keras, this PC)"], [[r.input.map(v => v.toFixed(1)).join(", "), fx(r.tflite, 4), fx(r.keras, 4), fx(r.diff, 5), badge(r.equivalent ? "yes" : "no", r.equivalent ? "ok" : "bad"), r.tfliteUs + " / " + r.kerasUs]])}`); }
});

// ------------------------------------------------------------ Lab 8
PANELS[8] = () => ({
  html: card("Practice assessment", `<div class="row"><button class="btn" data-act="l8draw">🎲 Draw a task</button><button class="btn ghost" data-act="l8submit">I have submitted my files</button></div>
    <div id="l8task" class="note">Draw a task to begin. Time limit: 45–60 minutes, official documentation only.</div><div class="kpi"><b id="l8t">45:00</b><span>time remaining</span></div>`) +
    card("Viva questions (5 random)", `<button class="btn ghost" data-act="l8viva">New questions</button><ol id="l8q"></ol>`),
  init() { A.l8viva(); }
});
let T8 = null;
Object.assign(A, {
  l8draw: () => { const t = MIDTERM_TASKS[Math.floor(Math.random() * MIDTERM_TASKS.length)]; setHTML("l8task", `<b>Your task:</b> ${esc(t)}`); clearInterval(T8); let s = 45 * 60; T8 = setInterval(() => { s--; const e = $("#l8t"); if (!e || s < 0) return clearInterval(T8); e.textContent = String(Math.floor(s / 60)).padStart(2, "0") + ":" + String(s % 60).padStart(2, "0"); }, 1000); },
  l8submit: () => { clearInterval(T8); mark("midterm"); toast("Marked as submitted"); },
  l8viva: () => { const p = [...VIVA].sort(() => Math.random() - .5).slice(0, 5); setHTML("l8q", p.map(q => `<li>${esc(q)}</li>`).join("")); }
});

// ------------------------------------------------------------ Lab 9
PANELS[9] = () => ({
  html: card("Step 3 — convert the model to a C array", `<button class="btn" data-act="l9hdr">xxd -i comfort_model_quant.tflite &gt; model_data.h</button><div id="l9h"></div>`) +
    card("Steps 5–7 — compile, upload and observe", `<div class="grid3">${sel("l9b", "Board", [["nano33", "Arduino Nano 33 BLE Sense"], ["esp32", "ESP32 DevKit"]], "nano33")}${fld("l9a", "kTensorArenaSize (bytes)", 4096)}<button class="btn" style="margin-top:18px" data-act="l9run">Compile, upload &amp; run</button></div>
      <div class="row" style="margin-top:6px">Try:${[8192, 4096, 2048, 1800, 1700, 1600].map(v => `<button class="btn tiny ghost" data-act="l9p" data-v="${v}">${v}</button>`).join("")}</div><pre id="l9o">Serial Monitor output appears here.</pre><div id="l9s" class="muted small"></div>`)
});
Object.assign(A, {
  l9hdr: async () => { const r = await api("/api/mcu/header", {}); setHTML("l9h", `<div class="note">Array name <b class="mono">${esc(r.name)}</b>, length <b class="mono">${r.len}</b> bytes. ${esc(r.note)}</div>${code(r.header.split("\n").slice(0, 12).join("\n") + "\n  ...\n};\nunsigned int " + r.name + "_len = " + r.len + ";")}<span class="muted small">Saved: ${esc(r.file)}</span>`); },
  l9p: el => { $("#l9a").value = el.dataset.v; },
  l9run: async () => { const r = await api("/api/mcu/run", { board: val("l9b"), arena: num("l9a") }); setHTML("l9o", esc(r.lines.join("\n"))); setHTML("l9s", r.ok ? `Estimated inference ≈ ${r.us} µs · smallest working arena so far: ${r.smallestWorking} B · output ${r.match ? "matches" : "differs from"} the workstation model.` : "Run failed – adjust and try again."); }
});

// ------------------------------------------------------------ Lab 10
PANELS[10] = () => ({
  html: card("ThingsBoard-style platform — devices", `<div class="row">${fld("l10n", "Device name", "sensor-node-1", "text")}<button class="btn" style="margin-top:18px" data-act="l10dev">Create device</button></div><div id="l10d"></div>
    <p class="muted small">Login and default-password change are part of real ThingsBoard CE; here the platform is built in.</p>`) +
    card("Publisher (token as MQTT user name)", `<div id="l10p"></div>${code('client.username_pw_set(ACCESS_TOKEN)\nclient.connect("localhost", 1883, 60)\nclient.publish("v1/devices/me/telemetry", json.dumps(payload))')}`) +
    card("Dashboard", `<canvas id="c10"></canvas><div class="grid2" style="margin-top:8px"><div class="gauges" style="grid-template-columns:1fr"><canvas id="g10"></canvas></div><div><h4>Alarm rule</h4><div class="grid3">${sel("l10k", "Key", ["temperature", "humidity", "light"], "temperature")}${sel("l10o", "Op", [">", "<"], ">")}${fld("l10t", "Threshold", 28)}</div><button class="btn small" style="margin-top:6px" data-act="l10rule">Add alarm rule</button><div id="l10r" class="small muted"></div></div></div><h4>Alarms</h4><div id="l10a"></div>`),
  async init() {
    const T = { t: [], h: [], dev: "" };
    const drawTb = () => {
      const tb = S.tb || {}; const dv = tb.devices || [];
      setHTML("l10d", dv.length ? table(["name", "access token", "messages"], dv.map(d => [esc(d.name), `<span class="mono">${esc(d.token)}</span>`, d.count]), [2]) : `<span class="muted small">No devices yet.</span>`);
      const p = tb.publisher || {};
      setHTML("l10p", `<div class="grid3">${sel("l10dev", "Device", dv.map(d => [d.token, d.name]), p.token)}${sel("l10pi", "Interval (s)", [1, 2, 5], p.interval || 2)}<button class="btn" style="margin-top:18px" data-act="l10pub">${p.enabled ? "Restart" : "▶ Start"} publisher</button></div>${p.enabled ? badge("running – sent " + (p.sent || 0), "ok") : badge("stopped", "info")}${p.error ? `<div class="note warn">${esc(p.error)}</div>` : ""}<button class="btn small ghost" data-act="l10stop">■ Stop</button>`);
      setHTML("l10r", (tb.rules || []).map(r => `<div>${esc(r.device)}: ${esc(r.key)} ${esc(r.op)} ${r.threshold} ${r.active ? badge("ACTIVE", "bad") : ""}</div>`).join(""));
      setHTML("l10a", (tb.alarms || []).length ? table(["time", "device", "rule", "value", "status"], tb.alarms.slice().reverse().map(a => [new Date(a.ts).toLocaleTimeString(), esc(a.device), esc(a.rule), a.value, badge(a.status, a.status.startsWith("ACTIVE") ? "bad" : "info")])) : `<span class="muted small">No alarms.</span>`);
    };
    const drawC = () => { chart($("#c10"), [{ name: "temperature", data: T.t }, { name: "humidity", data: T.h }], { xlabel: "telemetry messages" }); gauge($("#g10"), T.t.length ? T.t[T.t.length - 1] : null, 0, 50, "temperature", " °C"); };
    HOOK.state = () => { drawTb(); drawC(); };
    HOOK.tb = d => { T.t.push(d.values.temperature); T.h.push(d.values.humidity); if (T.t.length > 120) { T.t.shift(); T.h.shift(); } drawC(); };
    HOOK.tbalarm = async () => { await refresh(); };
    drawTb(); drawC();
  }
});
Object.assign(A, {
  l10dev: async () => { S.tb = await api("/api/tb/device", { name: val("l10n") }); HOOK.state(); },
  l10pub: async () => { if (!(S.board || {}).running) await api("/api/board", { running: true }); S.tb = await api("/api/tb/publisher", { enabled: true, token: val("l10dev"), interval: num("l10pi") }); await refresh(); },
  l10stop: async () => { await api("/api/tb/publisher", { enabled: false }); await refresh(); },
  l10rule: async () => { const dv = (S.tb.devices || [])[0]; if (!dv) return toast("Create a device first", true); S.tb = await api("/api/tb/rule", { device: dv.name, key: val("l10k"), op: val("l10o"), threshold: num("l10t") }); HOOK.state(); }
});

// ------------------------------------------------------------ Lab 11
PANELS[11] = () => ({
  html: card("Steps 1–3 — train on the logged data", `<div class="grid3">${fld("l11c", "contamination", 0.05, "number", 'step="0.01"')}<label class="inline" style="margin-top:18px"><input type="checkbox" id="l11s" checked> compare 0.01 / 0.05 / 0.10 / 0.20</label><button class="btn" style="margin-top:18px" data-act="l11train">Train Isolation Forest</button></div><div id="l11o"></div>`) +
    card("Steps 4–6 — live detection on MQTT", `<div class="note small">Needs the Lab 5 publisher running. <button class="btn tiny" data-act="l11pre">Start board + publisher</button></div>
      <div class="row"><button class="btn" data-act="l11live">Enable live detector</button><button class="btn ghost" data-act="l11off">Disable</button><button class="btn ghost" data-act="l11test">Create unusual condition (warm + cover LDR)</button></div>
      <div id="l11b" class="banner" style="margin:8px 0">No anomaly</div><div id="l11s2" class="small muted"></div>`),
  init() {
    let t = 0; const st = () => { const a = S.anomaly || {}; setHTML("l11s2", `Detector ${a.enabled ? badge("on", "ok") : badge("off", "info")} · trained: ${a.trained ? "yes" : "no"} · readings checked ${a.checked || 0} · flagged ${a.flagged || 0} ${a.last ? "· last: " + esc(a.last) : ""}`); };
    HOOK.anomaly = () => st(); HOOK.state = st; st();
    HOOK.mqtt = d => { if (d.topic === "aiot/lab11/anomaly") { const b = $("#l11b"); if (b) { b.textContent = "⚠ ANOMALY DETECTED"; b.className = "banner alert"; clearTimeout(t); t = setTimeout(() => { b.textContent = "No anomaly"; b.className = "banner"; }, 5000); } } };
  }
});
Object.assign(A, {
  l11train: async () => {
    const r = await api("/api/anomaly/train", { contamination: num("l11c"), sweep: val("l11s") ? [0.01, 0.05, 0.1, 0.2] : [] }); await refresh();
    setHTML("l11o", `<div class="grid3" style="margin:8px 0">${kpi(r.normal, "normal (1)")}${kpi(r.anomalies, "anomaly (−1)")}${kpi(pct(r.detectRate), "injected faults caught")}</div><canvas id="c11" class="short"></canvas>
      ${r.sweep ? `<h4>Contamination trade-off</h4>${table(["contamination", "rows flagged", "injected faults detected"], r.sweep.map(s => [s.contamination, s.flagged, pct(s.detectRate)]), [0, 1, 2])}<p class="muted small">More flagged rows = more false alarms; fewer = more missed faults. Justify your choice in the report.</p>` : ""}`);
    chart($("#c11"), [{ name: "temperature", data: r.temp }, { name: "anomaly", data: r.temp.map((v, i) => r.flags[i] ? v : null), dots: true, color: "#f87171" }]);
  },
  l11pre: async () => { await api("/api/board", { sketch: "combined", running: true }); S.mqtt = await api("/api/mqtt/bridge", { enabled: true, interval: 2, qos: 0 }); await refresh(); toast("Board and publisher running"); },
  l11live: async () => { S.anomaly = await api("/api/anomaly/live", { enabled: true }); HOOK.anomaly(); },
  l11off: async () => { S.anomaly = await api("/api/anomaly/live", { enabled: false }); HOOK.anomaly(); },
  l11test: async () => { await api("/api/event", { name: "heat" }); await api("/api/event", { name: "cover" }); }
});

// ------------------------------------------------------------ Lab 12
PANELS[12] = () => ({
  html: card("Vibration case study (simulated MPU6050)", `<div class="grid3">${fld("l12w", "Window size", 20)}${sel("l12x", "Extra feature (student task)", [["none", "none (baseline)"], ["p2p", "peak-to-peak"], ["rms", "RMS"], ["crest", "crest factor"], ["kurtosis", "kurtosis"]], "p2p")}<button class="btn" style="margin-top:18px" data-act="l12run">Collect, engineer &amp; train</button></div><div id="l12o"></div>`) +
    card("Edge deployment sketch (Step 8)", `<ol class="small"><li>Export the trained classifier (a small decision tree / MLP) and convert to TFLite (Lab 7).</li><li>Quantise to int8 and emit a C array (Lab 9).</li><li>On the MCU keep a circular buffer of the last N samples; compute mean, std, max each hop.</li><li>Run inference per window and raise a GPIO / MQTT alert on class 1.</li></ol>`)
});
A.l12run = async () => {
  const r = await api("/api/pdm/run", { window: num("l12w"), extra: val("l12x") });
  const row = (n, m) => [n, pct(m.accuracy), pct(m.precision), pct(m.recall), m.confusion[1][0] + " missed faults / " + m.confusion[0][1] + " false alarms"];
  setHTML("l12o", `<canvas id="c12" class="short" style="margin:8px 0"></canvas>${table(["features", "accuracy", "precision", "recall", "errors"], [row("mean, std, max", r.base)].concat(r.extra ? [row("+ " + r.extraName, r.extra)] : []), [1, 2, 3])}
    ${r.extra ? `<div class="note">Recall change from the extra feature: <b>${r.recallDelta >= 0 ? "+" : ""}${(r.recallDelta * 100).toFixed(1)} points</b> (results vary between runs; repeat a few times).</div>` : ""}
    <h4>Feature importances (baseline)</h4><canvas id="c12b" class="short"></canvas><div class="note warn">${esc(r.note)} A missed fault (false negative) costs far more than a false alarm, so prioritise recall.</div>`);
  chart($("#c12"), [{ name: "healthy", data: r.healthy }, { name: "degraded", data: r.degraded }], { xlabel: "sample (vibration magnitude, g)" });
  bars($("#c12b"), r.features, r.importances, { fmt: v => fx(v, 3), max: 1 });
};

// ------------------------------------------------------------ Lab 13
PANELS[13] = () => ({
  html: card("Choose a project theme", `<div class="grid2"><div class="kpi"><b>Smart irrigation</b><span>soil moisture + DHT11 → relay / pump (decision tree)</span><br><button class="btn small" data-act="l13p" data-t="irrigation" style="margin-top:6px">Select</button></div>
    <div class="kpi"><b>Health monitoring</b><span>pulse + skin temperature + motion → alert (Isolation Forest). Education demo only.</span><br><button class="btn small" data-act="l13p" data-t="health" style="margin-top:6px">Select</button></div></div><div id="l13a"></div>`) +
    card("Technical risk paragraph (kept in your browser)", `<textarea id="l13r" rows="5" placeholder="Risk: sensor noise ... Mitigation: median filter ...">${esc(store.get("risk"))}</textarea><p class="muted small">Tick the checkbox in the guide when you have written it.</p>`),
  init() { $("#l13r").oninput = e => store.set("risk", e.target.value); }
});
A.l13p = async el => { const r = await api("/api/project/run", { template: el.dataset.t, validate: true }); setHTML("l13a", `<h4>${esc(r.name)}</h4>${table(["Layer", "Components"], r.architecture.map(a => [`<b>${esc(a.layer)}</b>`, esc(a.items)]))}<p class="small">Inputs: ${r.features.map(esc).join(", ")}. Draw this as your architecture diagram, adapt it, and write the one-page proposal.</p>`); };

// ------------------------------------------------------------ Lab 14
PANELS[14] = () => ({
  html: card("Functional test scenarios", `<div class="row">${sel("l14t", "Project", [["irrigation", "Smart irrigation"], ["health", "Health monitoring"]], "irrigation")}<label class="inline" style="margin-top:18px"><input type="checkbox" id="l14v" checked> input-validation stage</label><button class="btn" style="margin-top:18px" data-act="l14run">Run test pass</button><button class="btn ghost" style="margin-top:18px" data-act="l14exp">Export test_log.csv</button></div><div id="l14o"></div>`)
});
Object.assign(A, {
  l14run: async () => {
    const r = await api("/api/project/run", { template: val("l14t"), validate: val("l14v") });
    setHTML("l14o", `<div class="grid3" style="margin:8px 0">${kpi(r.passed + " / " + r.total, "scenarios passed")}${kpi(r.validate ? "on" : "off", "validation layer")}</div>${table(["#", "scenario", "type", "input", "expected", "observed", "result"], r.rows.map(x => [x.n, esc(x.name), badge(x.kind, x.kind === "fault" ? "warn" : "info"), `<span class="mono">${x.input.map(v => v === null ? "NaN" : v).join(", ")}</span>`, esc(x.expected), esc(x.observed), badge(x.pass ? "PASS" : "FAIL", x.pass ? "ok" : "bad")]))}
      ${r.passed < r.total ? `<div class="note warn">Some scenarios failed. Find the cause, fix the design (hint: what protects the model from a disconnected or shorted sensor?) and repeat the test pass (Step 6).</div>` : `<div class="note">All scenarios pass. Update your architecture diagram and export the test log.</div>`}`);
  },
  l14exp: async () => { const r = await api("/api/project/export", {}); toast("Saved " + r.file); }
});

// ------------------------------------------------------------ Lab 15
PANELS[15] = () => ({
  html: card("Benchmark the pipeline", `<div class="grid3">${sel("l15t", "Project", [["irrigation", "Smart irrigation"], ["health", "Health monitoring"]], "irrigation")}${sel("l15w", "Inference runs on", [["edge", "Edge MCU"], ["gateway", "Gateway / workstation"], ["cloud", "Cloud"]], "gateway")}${sel("l15l", "Network link", [["loopback", "Loopback"], ["wifi", "Wi-Fi LAN"], ["cellular", "4G / cellular"], ["internet", "Internet (cloud)"]], "wifi")}</div>
    <div class="row" style="margin-top:8px">${fld("l15n", "Trials (≥ 20)", 30)}<button class="btn" style="margin-top:18px" data-act="l15run">Run benchmark</button></div><div id="l15o"></div>`)
});
A.l15run = async () => {
  const r = await api("/api/bench/run", { trials: num("l15n"), where: val("l15w"), link: val("l15l"), template: val("l15t") }); const t = r.total;
  setHTML("l15o", `<div class="grid3" style="margin:8px 0">${kpi(t.mean + " ms", "mean response time")}${kpi(t.std + " ms", "std deviation")}${kpi(t.p95 + " ms", "95th percentile")}</div>
    <div class="grid2"><div><h4>Latency per stage (ms)</h4><canvas id="c15a" class="short"></canvas></div><div><h4>Response-time histogram</h4><canvas id="c15b" class="short"></canvas></div></div>
    ${table(["metric", "value"], [["Accuracy (held-out)", pct(r.accuracy.accuracy)], ["Precision / recall", fx(r.accuracy.precision, 3) + " / " + fx(r.accuracy.recall, 3)], ["Model size (est.)", r.modelBytes + " bytes"], ["Inference (mean)", r.stages[2].mean + " ms"], ["Largest contributor", `<b>${esc(r.bottleneck)}</b>`]])}
    <div class="note"><b>Design change to consider:</b> ${esc(r.advice)}</div><div class="note warn">${esc(r.note)}</div>`);
  bars($("#c15a"), r.stages.map(s => s.name), r.stages.map(s => s.mean), { fmt: v => fx(v, 1) });
  bars($("#c15b"), r.histogram.map((_, i) => (r.histLo + i * r.histW).toFixed(0)), r.histogram, { fmt: v => v, sub: r.histogram.map(() => "ms") });
};

// ------------------------------------------------------------ Lab 16
PANELS[16] = () => ({
  html: card("Viva practice", `<button class="btn" data-act="l16draw">Ask me a question</button><div id="l16q" class="note">Press the button. Answer aloud, then reveal the next one.</div>`) +
    card("Submission checklist", `<ul class="small"><li>Slides + 8–10 minute talk (problem, architecture, implementation, results, lessons).</li><li>Live demo and a fallback (screenshots / recording).</li><li>Final report, source code and slides.</li><li>One-page personal reflection.</li></ul><button class="btn" data-act="report">Download evidence report (Markdown)</button><p class="muted small">The report lists completed labs and the measured results (tables for Labs 3, 6, 7, 9, 11, 12, 14, 15). Paste it into your Word report.</p>`)
});
A.l16draw = () => setHTML("l16q", `<b>${esc(VIVA[Math.floor(Math.random() * VIVA.length)])}</b>`);
