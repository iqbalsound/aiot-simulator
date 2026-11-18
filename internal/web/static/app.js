"use strict";
// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.
// ---------------------------------------------------------------- helpers
const $ = (s, r = document) => r.querySelector(s);
const $$ = (s, r = document) => [...r.querySelectorAll(s)];
const esc = s => String(s ?? "").replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
const fx = (v, d = 3) => (v === null || v === undefined || Number.isNaN(v)) ? "–" : Number(v).toFixed(d);
const pct = v => (v * 100).toFixed(1) + " %";
let S = { progress: [], mqtt: {}, board: {}, env: {}, wiring: {}, tb: {}, anomaly: {} };
let CUR = 1;
const HOOK = {};      // per-panel live hooks, reset on lab change
const A = {};         // click actions
const PANELS = {};    // lab workbench renderers
const DASH = { t: [], h: [], l: [], m: 0, last: null, n: 0 };
const TOPICS = {};
const SERIAL = [];

async function api(path, body) {
  const o = body === undefined ? {} : { method: "POST", headers: { "Content-Type": "application/json", "X-AIoT": "1" }, body: JSON.stringify(body) };
  const r = await fetch(path, o);
  const t = await r.text();
  let j; try { j = JSON.parse(t); } catch { j = t; }
  if (!r.ok) throw new Error((j && j.error) || r.statusText);
  return j;
}
function toast(msg, err) {
  const d = document.createElement("div");
  d.className = "toast" + (err ? " err" : ""); d.textContent = msg;
  $("#toast").appendChild(d);
  setTimeout(() => d.remove(), err ? 7000 : 3500);
}
async function run(fn) { try { return await fn(); } catch (e) { toast(e.message || String(e), true); } }
const mark = (key, value = true, evidence) => run(async () => { S.progress = await api("/api/progress/mark", { key, value, evidence }); renderNav(); renderGuide(); });

// ---------------------------------------------------------------- canvas charts
function prep(cv) {
  if (!cv || !cv.getContext) return null;
  const ctx = cv.getContext("2d"); if (!ctx) return null;
  const dpr = window.devicePixelRatio || 1, w = cv.clientWidth, h = cv.clientHeight;
  if (!w || !h) return null;
  cv.width = w * dpr; cv.height = h * dpr; ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.clearRect(0, 0, w, h);
  const cs = getComputedStyle(document.documentElement);
  return { ctx, w, h, text: cs.getPropertyValue("--muted").trim() || "#888", line: cs.getPropertyValue("--line").trim() || "#444", fg: cs.getPropertyValue("--text").trim() || "#fff" };
}
const palette = () => { const cs = getComputedStyle(document.documentElement); return [1, 2, 3, 4, 5, 6].map(i => cs.getPropertyValue("--c" + i).trim() || "#2dd4bf"); };
function chart(cv, series, o = {}) {
  const c = prep(cv); if (!c) return; cv._redraw = () => chart(cv, series, o); const COLORS = palette();
  const { ctx, w, h } = c, L = 42, R = 8, T = 18, B = 20;
  let lo = Infinity, hi = -Infinity, n = 0;
  series.forEach(s => { n = Math.max(n, s.data.length); s.data.forEach(v => { if (v !== null && v !== undefined && isFinite(v)) { lo = Math.min(lo, v); hi = Math.max(hi, v); } }); });
  if (!isFinite(lo)) { ctx.fillStyle = c.text; ctx.fillText("no data yet", L + 10, h / 2); return; }
  if (o.min !== undefined) lo = o.min; if (o.max !== undefined) hi = o.max;
  if (hi === lo) { hi += 1; lo -= 1; }
  const pad = (hi - lo) * 0.06; if (o.min === undefined) lo -= pad; if (o.max === undefined) hi += pad;
  const X = i => L + (n <= 1 ? 0 : i * (w - L - R) / (n - 1)), Y = v => T + (hi - v) * (h - T - B) / (hi - lo);
  ctx.strokeStyle = c.line; ctx.fillStyle = c.text; ctx.font = "11px system-ui"; ctx.lineWidth = 1;
  for (let k = 0; k <= 4; k++) { const v = lo + (hi - lo) * k / 4, y = Y(v); ctx.beginPath(); ctx.moveTo(L, y); ctx.lineTo(w - R, y); ctx.stroke(); ctx.fillText(Math.abs(v) >= 100 ? v.toFixed(0) : v.toFixed(1), 4, y + 4); }
  if (o.xlabel) ctx.fillText(o.xlabel, w / 2 - 30, h - 4);
  if (o.hline !== undefined) { ctx.strokeStyle = "#f87171"; ctx.setLineDash([5, 4]); ctx.beginPath(); ctx.moveTo(L, Y(o.hline)); ctx.lineTo(w - R, Y(o.hline)); ctx.stroke(); ctx.setLineDash([]); }
  series.forEach((s, si) => {
    const col = s.color || COLORS[si % COLORS.length];
    ctx.strokeStyle = col; ctx.fillStyle = col; ctx.lineWidth = s.width || 1.6;
    if (!s.dots) {
      ctx.beginPath(); let pen = false;
      s.data.forEach((v, i) => { if (v === null || v === undefined || !isFinite(v)) { pen = false; return; } pen ? ctx.lineTo(X(i), Y(v)) : ctx.moveTo(X(i), Y(v)); pen = true; });
      ctx.stroke();
    }
    if (s.dots || s.marks) s.data.forEach((v, i) => { if (v === null || v === undefined || !isFinite(v)) return; if (s.marks && !s.marks[i]) return; ctx.beginPath(); ctx.arc(X(i), Y(v), s.marks ? 4 : 2, 0, 7); ctx.fill(); });
  });
  ctx.font = "11px system-ui"; let lx = L + 6;
  series.forEach((s, si) => { if (!s.name) return; ctx.fillStyle = s.color || COLORS[si % COLORS.length]; ctx.fillRect(lx, 4, 10, 8); ctx.fillStyle = c.text; ctx.fillText(s.name, lx + 14, 12); lx += 22 + ctx.measureText(s.name).width; });
}
function bars(cv, labels, values, o = {}) {
  const c = prep(cv); if (!c) return; cv._redraw = () => bars(cv, labels, values, o); const COLORS = palette();
  const { ctx, w, h } = c, L = 10, B = 34, T = 14, n = values.length;
  const mx = o.max ?? Math.max(...values, 1e-9), bw = (w - L * 2) / n;
  ctx.font = "11px system-ui";
  values.forEach((v, i) => {
    const bh = (h - T - B) * v / mx, x = L + i * bw + bw * 0.15;
    ctx.fillStyle = (o.colors && o.colors[i]) || COLORS[i % COLORS.length]; ctx.fillRect(x, h - B - bh, bw * 0.7, bh);
    ctx.fillStyle = c.fg; ctx.textAlign = "center"; ctx.fillText(o.fmt ? o.fmt(v) : fx(v, 2), x + bw * 0.35, h - B - bh - 3);
    ctx.fillStyle = c.text; ctx.fillText(labels[i], x + bw * 0.35, h - 16); if (o.sub) ctx.fillText(o.sub[i], x + bw * 0.35, h - 4);
  });
  ctx.textAlign = "left";
}
function gauge(cv, v, min, max, label, unit, warn) {
  const c = prep(cv); if (!c) return; cv._redraw = () => gauge(cv, v, min, max, label, unit, warn);
  const { ctx, w, h } = c, cx = w / 2, cy = h * 0.82, r = Math.min(w / 2 - 12, h * 0.72);
  const f = Math.max(0, Math.min(1, ((v ?? min) - min) / (max - min))), a0 = Math.PI * 0.85, a1 = Math.PI * 2.15;
  ctx.lineWidth = 13; ctx.lineCap = "round";
  ctx.strokeStyle = c.line; ctx.beginPath(); ctx.arc(cx, cy, r, a0, a1); ctx.stroke();
  ctx.strokeStyle = (warn !== undefined && v > warn) ? getComputedStyle(document.documentElement).getPropertyValue("--bad") : getComputedStyle(document.documentElement).getPropertyValue("--accent"); ctx.beginPath(); ctx.arc(cx, cy, r, a0, a0 + (a1 - a0) * f); ctx.stroke();
  ctx.fillStyle = c.fg; ctx.textAlign = "center"; ctx.font = "bold 22px system-ui"; ctx.fillText(v === null || v === undefined ? "–" : (Math.abs(v) >= 100 ? v.toFixed(0) : v.toFixed(1)) + unit, cx, cy - 4);
  ctx.font = "12px system-ui"; ctx.fillStyle = c.text; ctx.fillText(label, cx, cy + 18); ctx.textAlign = "left";
}

// ---------------------------------------------------------------- HTML builders
const table = (head, rows, num = []) => `<table><thead><tr>${head.map((h, i) => `<th class="${num.includes(i) ? "num" : ""}">${esc(h)}</th>`).join("")}</tr></thead><tbody>${rows.map(r => `<tr>${r.map((c, i) => `<td class="${num.includes(i) ? "num" : ""}">${c}</td>`).join("")}</tr>`).join("")}</tbody></table>`;
const kpi = (v, l) => `<div class="kpi"><b>${v}</b><span>${esc(l)}</span></div>`;
const badge = (t, k) => `<span class="badge b-${k}">${esc(t)}</span>`;
const fld = (id, label, val, type = "number", extra = "") => `<label>${esc(label)}<input id="${id}" type="${type}" value="${esc(val)}" ${extra}></label>`;
const sel = (id, label, opts, val) => `<label>${esc(label)}<select id="${id}">${opts.map(o => { const [v, t] = Array.isArray(o) ? o : [o, o]; return `<option value="${esc(v)}" ${String(v) === String(val) ? "selected" : ""}>${esc(t)}</option>`; }).join("")}</select></label>`;
const val = id => { const e = document.getElementById(id); return e ? (e.type === "checkbox" ? e.checked : e.value) : undefined; };
const num = id => Number(val(id));
const setHTML = (id, h) => { const e = document.getElementById(id); if (e) e.innerHTML = h; };
const mkCheck = (it) => `<li class="${it.done ? "done" : ""}">${it.manual ? `<input type="checkbox" ${it.done ? "checked" : ""} data-act="tick" data-key="${it.key}">` : `<span class="mk">${it.done ? "✓" : ""}</span>`}<span>${esc(it.label)}${it.evidence && it.evidence !== "ticked by student" ? `<span class="ev">${esc(it.evidence)}</span>` : ""}</span></li>`;

// ---------------------------------------------------------------- layout renderers
function renderNav() {
  const nav = $("#labnav"); if (!nav) return;
  nav.innerHTML = `<button class="nav ${CUR === 0 ? "on" : ""}" data-act="circuit"><span class="n">⚡</span><span class="t"><b>Circuit Lab</b><br><span class="muted tiny">wire &amp; run an Arduino Uno</span></span></button>` + S.progress.map(p => {
    const d = p.req.filter(r => r.done).length, st = p.done ? "done" : d ? "part" : "";
    return `<button class="nav ${p.lab === CUR ? "on" : ""}" data-act="lab" data-n="${p.lab}"><span class="n">${p.lab}</span><span class="t">${esc(p.title)}</span><span class="dot ${st}" title="${d}/${p.req.length} required steps"></span></button>`;
  }).join("");
}
function renderTop() {
  const b = S.board || {}, m = S.mqtt || {};
  $("#topbar").innerHTML = `<span class="pill ${b.running ? "ok" : "warn"}">Board <b>${b.running ? "running ×" + b.speed : "stopped"}</b></span>
   <span class="pill">Sketch <b>${esc(((S.sketches || []).find(s => s.ID === b.sketch) || {}).Name || b.sketch || "")}</b></span>
   <span class="pill">MQTT <b>localhost:${m.port || "?"}</b> · clients <b>${m.clients ?? 0}</b> · ${m.allowAnonymous ? "anonymous allowed" : "authentication required"}</span>
   <span class="pill click ${(S.student && S.student.name) ? "" : "warn"}" data-act="student">Student <b>${(S.student && S.student.name) ? esc(S.student.name) + " · " + esc(S.student.reg) : "not set – click to enter"}</b></span>
   <span class="pill ${S.logging ? "ok" : ""}">Logger <b>${S.logging ? "recording" : "idle"}</b> · rows <b>${S.rows ?? 0}</b></span>`;
  $("#ver").textContent = "v" + (S.version || "");
  $("#wsPath").textContent = "Files: " + (S.workspace || "");
  const al = $("#authorLine"); if (al) al.textContent = (S.author || "") + " · " + (S.authorTitle || "") + ", " + (S.authorDept || "");
}
function renderGuide() {
  if (CUR === 0) { $("#guide").innerHTML = ""; return; }
  const L = LABS[CUR - 1], p = S.progress[CUR - 1]; if (!L || !p) return;
  $("#guide").innerHTML = `<div class="guide-actions"><button class="btn tiny" data-act="labpdf" data-n="${L.n}" title="Download this lab's report as PDF">⬇ PDF</button><button class="btn tiny ghost" data-act="labreset" data-n="${L.n}" title="Clear this lab so it can be redone">↺ Reset lab</button></div><h2>Lab ${L.n}: ${esc(L.title)}</h2><div class="muted small">${esc(L.clo)}</div>
   <h4>Objectives</h4><ul>${L.obj.map(o => `<li>${esc(o)}</li>`).join("")}</ul>
   <h4>Procedure (from the manual)</h4><ol>${L.steps.map(o => `<li>${esc(o)}</li>`).join("")}</ol>
   <h4>Student task</h4><p>${esc(L.task)}</p>
   <div class="note">${esc(L.sim)}</div>
   <h4>Required outcomes ${p.done ? badge("complete", "ok") : ""}</h4><ul class="chk">${p.req.map(mkCheck).join("")}</ul>
   ${p.task.length ? `<h4>Student task outcomes ${p.taskDone ? badge("complete", "ok") : ""}</h4><ul class="chk">${p.task.map(mkCheck).join("")}</ul>` : ""}`;
}
function renderLab() {
  Object.keys(HOOK).forEach(k => delete HOOK[k]);
  if (typeof CL !== "undefined" && CUR !== 0) CL.stop();
  $("#content").classList.toggle("wide", CUR === 0);
  renderNav(); renderGuide();
  const P = PANELS[CUR]; const w = $("#work"); w.innerHTML = "";
  if (P) { const r = P(); w.innerHTML = r.html; if (r.init) setTimeout(r.init, 0); }
  $("#main").scrollTop = 0;
}

// ---------------------------------------------------------------- bench (board, env, serial)
const BAUDS = [9600, 19200, 57600, 115200];
function renderBoard() {
  const b = S.board || {}, w = S.wiring || {};
  const bad = (S.wiringCheck || []).filter(c => c.status === "WRONG" || c.status === "CONFLICT").length;
  $("#boardCtl").innerHTML = `<div class="grid2">${sel("bSketch", "Uploaded sketch", (S.sketches || []).map(s => [s.ID, s.Name]), b.sketch)}${sel("bBaud", "Serial.begin()", BAUDS, b.baud)}</div>
   <div class="grid3" style="margin-top:8px">${sel("bSpeed", "Speed", [[1, "1× real time"], [5, "5×"], [20, "20×"], [50, "50×"]], b.speed)}${sel("bDht", "Sensor", [["11", "DHT11"], ["22", "DHT22"]], S.dht22 ? "22" : "11")}${sel("bFields", "Logger fields", [[4, "4 (T,H,M,L)"], [5, "5 (+extra)"]], S.logFields)}</div>
   <div class="row" style="margin-top:8px"><button class="btn" data-act="boardrun">${b.running ? "■ Stop board" : "▶ Start board"}</button><label class="inline"><input type="checkbox" id="bAuto" ${S.autoVary ? "checked" : ""}> Auto-vary conditions</label></div>
   ${bad ? `<div class="note warn small">Wiring problem on ${bad} sensor(s): readings will fail. See Lab 2.</div>` : ""}`;
  ["bSketch", "bBaud", "bSpeed", "bDht", "bFields", "bAuto"].forEach(id => { const e = $("#" + id); if (e) e.onchange = () => run(async () => { await api("/api/board", { sketch: val("bSketch"), baud: num("bBaud"), speed: num("bSpeed"), dht22: val("bDht") === "22", autoVary: val("bAuto"), logFields: num("bFields") }); await refresh(); }); });
}
const ENV = [["temp", "Temperature °C", -10, 60, 0.5], ["hum", "Humidity %RH", 0, 100, 1], ["light", "Light level %", 0, 100, 1], ["motionProb", "Motion probability", 0, 1, 0.05], ["distance", "Distance cm (HC-SR04)", 2, 400, 1], ["noise", "Sensor noise ×", 0, 3, 0.1], ["glitch", "Glitch rate (bad lines)", 0, 0.3, 0.01]];
function renderEnv() {
  const e = S.env || {};
  $("#envCtl").innerHTML = ENV.map(([k, l, lo, hi, st]) => `<label>${l}: <b id="ev_${k}">${fx(e[k], st < 1 ? 2 : 1)}</b><input type="range" id="env_${k}" min="${lo}" max="${hi}" step="${st}" value="${e[k] ?? 0}"></label>`).join("") +
    `<div class="row" style="margin-top:8px"><button class="btn small ghost" data-act="evt" data-name="heat">🔥 Warm sensor</button><button class="btn small ghost" data-act="evt" data-name="cover">🌑 Cover LDR</button><button class="btn small ghost" data-act="evt" data-name="shake">📳 Shake rig</button></div>`;
  ENV.forEach(([k, , , , st]) => { const r = $("#env_" + k); r.oninput = () => { $("#ev_" + k).textContent = fx(r.value, st < 1 ? 2 : 1); r.dataset.drag = "1"; }; r.onchange = () => { r.dataset.drag = ""; run(() => api("/api/env", { [k]: Number(r.value) })); }; });
}
async function syncEnv() {
  if (!S.autoVary && !(S.board && S.board.running)) return;
  try { const s = await api("/api/state"); S = s; ENV.forEach(([k, , , , st]) => { const r = $("#env_" + k); if (r && !r.dataset.drag) { r.value = s.env[k]; $("#ev_" + k).textContent = fx(s.env[k], st < 1 ? 2 : 1); } }); renderTop(); } catch { }
}
function addSerial(e) {
  SERIAL.push(e); if (SERIAL.length > 160) SERIAL.shift();
  const box = $("#serialbox"); if (!box) return;
  const d = document.createElement("div");
  const cls = /[^\x20-\x7e]/.test(e.line) ? "sl-bad" : (/nan|#/.test(e.line) ? "sl-bad" : "");
  d.className = cls; d.textContent = e.line; box.appendChild(d);
  while (box.childElementCount > 160) box.firstChild.remove();
  box.scrollTop = box.scrollHeight;
}
function parseLine(line) {
  const p = line.split(","); if (p.length < 4) return null;
  const v = p.slice(0, 4).map(Number); if (v.some(x => !isFinite(x))) return null;
  return { t: v[0], h: v[1], m: v[2], l: v[3] };
}
function addLog(l) {
  const box = $("#logbox"); if (!box) return;
  const d = document.createElement("div"); d.className = "lg-" + l.level;
  d.textContent = new Date(l.ts).toLocaleTimeString() + "  " + l.msg; box.appendChild(d);
  while (box.childElementCount > 120) box.firstChild.remove(); box.scrollTop = box.scrollHeight;
}

// ---------------------------------------------------------------- actions
Object.assign(A, {
  lab: el => { CUR = Number(el.dataset.n); renderLab(); },
  tick: el => mark(el.dataset.key, el.checked),
  boardrun: () => run(async () => { await api("/api/board", { running: !(S.board || {}).running }); await refresh(); }),
  evt: el => run(() => api("/api/event", { name: el.dataset.name })),
  serialclear: () => run(async () => { await api("/api/serial/clear", {}); $("#serialbox").innerHTML = ""; }),
  clearlog: () => { $("#logbox").innerHTML = ""; },
  report: () => { window.location.href = "/api/report.md"; },
  quit: () => { if (confirm("Quit the simulator? Your files stay in the workspace folder.")) api("/api/quit", {}).then(() => document.body.innerHTML = "<div style='padding:40px;font:16px system-ui;color:#888'>AIoT Simulator has stopped. You can close this window.</div>"); },
});
document.addEventListener("click", ev => { const el = ev.target.closest("[data-act]"); if (!el) return; const f = A[el.dataset.act]; if (el.tagName === "A") ev.preventDefault(); if (f && el.type !== "checkbox") run(() => f(el, ev)); });
document.addEventListener("change", ev => { const el = ev.target; if (el.type === "checkbox" && el.dataset && el.dataset.act === "tick") A.tick(el); });

// ---------------------------------------------------------------- live events
function connect() {
  const es = new EventSource("/api/stream");
  es.onmessage = m => {
    let e; try { e = JSON.parse(m.data); } catch { return; }
    const d = e.data;
    switch (e.type) {
      case "serial": addSerial(d); { const v = parseLine(d.line); if (v) { DASH.t.push(v.t); DASH.h.push(v.h); DASH.l.push(v.l); DASH.m = v.m; DASH.n++; ["t", "h", "l"].forEach(k => { if (DASH[k].length > 120) DASH[k].shift(); }); DASH.last = v; } } HOOK.serial && HOOK.serial(d); break;
      case "log": addLog(d); break;
      case "progress": S.progress = d; renderNav(); renderGuide(); HOOK.progress && HOOK.progress(); break;
      case "mqtt": TOPICS[d.topic] = { topic: d.topic, payload: d.payload, count: ((TOPICS[d.topic] || {}).count || 0) + 1, from: d.from }; HOOK.mqtt && HOOK.mqtt(d); break;
      case "mqttstate": S.mqtt = d; renderTop(); HOOK.mqttstate && HOOK.mqttstate(d); break;
      case "tb": HOOK.tb && HOOK.tb(d); break;
      case "tbalarm": toast("ThingsBoard alarm: " + d.rule + " (value " + d.value + ")", true); HOOK.tbalarm && HOOK.tbalarm(d); break;
      case "anomaly": S.anomaly = d; HOOK.anomaly && HOOK.anomaly(d); break;
      case "reset": HOOK.reset ? HOOK.reset(d) : (window.onSimReset && window.onSimReset(d)); break;
      case "logger": S.rows = d.rows; renderTop(); HOOK.logger && HOOK.logger(d); break;
    }
  };
  es.onerror = () => { };
}
async function refresh() { S = await api("/api/state"); renderTop(); renderBoard(); renderNav(); renderGuide(); HOOK.state && HOOK.state(); }
async function init() {
  const sb = $("#monBaud"); sb.innerHTML = BAUDS.map(b => `<option>${b}</option>`).join("");
  sb.onchange = () => run(() => api("/api/serial/baud", { baud: Number(sb.value) }));
  S = await api("/api/state"); sb.value = S.monitorBaud;
  (S.serialTail || []).forEach(addSerial);
  renderTop(); renderBoard(); renderEnv(); renderLab(); connect();
  setInterval(() => fetch("/api/ping").catch(() => { }), 5000); fetch("/api/ping");
  setInterval(syncEnv, 3000);
  window.addEventListener("resize", () => HOOK.state && HOOK.state());
}
document.addEventListener("DOMContentLoaded", () => run(init));
