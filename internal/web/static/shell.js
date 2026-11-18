"use strict";
// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.
// Themes, student details, About, reset and PDF download.

const THEMES = [["auto", "Auto (follow system)"], ["ocean", "Ocean Dark"], ["daylight", "Daylight"], ["neon", "Navy & Purple Neon"], ["solar", "Solarized Dark"], ["forest", "Forest"], ["sunset", "Sunset"], ["contrast", "High Contrast"], ["paper", "Paper (print friendly)"]];
const lsGet = k => { try { return localStorage.getItem(k) || ""; } catch { return ""; } };
const lsSet = (k, v) => { try { localStorage.setItem(k, v); } catch { } };
const openDlg = id => { const d = document.getElementById(id); if (!d) return; d.showModal ? d.showModal() : d.setAttribute("open", ""); };
const closeDlg = id => { const d = document.getElementById(id); if (!d) return; d.close ? d.close() : d.removeAttribute("open"); };

function applyTheme(choice) {
  let eff = choice;
  if (choice === "auto") eff = (window.matchMedia && matchMedia("(prefers-color-scheme: light)").matches) ? "daylight" : "ocean";
  document.documentElement.dataset.theme = eff;
  document.documentElement.dataset.themeChoice = choice;
  setTimeout(() => $$("canvas").forEach(c => c._redraw && c._redraw()), 30);
}
function initThemes() {
  const sel = $("#themeSel"); if (!sel) return;
  const cur = lsGet("aiot.theme") || "auto";
  sel.innerHTML = THEMES.map(([v, t]) => `<option value="${v}" ${v === cur ? "selected" : ""}>${esc(t)}</option>`).join("");
  sel.onchange = () => { lsSet("aiot.theme", sel.value); applyTheme(sel.value); };
  applyTheme(cur);
  if (window.matchMedia) { try { matchMedia("(prefers-color-scheme: light)").addEventListener("change", () => { if ((lsGet("aiot.theme") || "auto") === "auto") applyTheme("auto"); }); } catch { } }
}
// apply immediately to avoid a flash of the wrong theme
(() => { const c = lsGet("aiot.theme") || "auto"; applyTheme(c); })();

// ---- download helper (keeps errors visible instead of navigating away)
async function download(url) {
  const r = await fetch(url);
  if (!r.ok) throw new Error((await r.text()).trim() || r.statusText);
  const blob = await r.blob();
  const cd = r.headers.get("Content-Disposition") || "";
  const m = /filename="([^"]+)"/.exec(cd);
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob); a.download = m ? m[1] : "report.pdf";
  document.body.appendChild(a); a.click(); a.remove();
  setTimeout(() => URL.revokeObjectURL(a.href), 4000);
  return a.download;
}
const needStudent = () => { if (S.student && S.student.name) return false; toast("Enter your name and registration number first – they are printed on the report.", true); A.student(); return true; };

const RESET_NOTES = {
  1: "Your inventory ticks and serial numbers will be cleared.",
  2: "Wiring is cleared (all sensors unconnected) and the board stops, so you can wire it again.",
  3: "The logged dataset and CSV files are deleted. Labs 6, 7 and 11 use that dataset: their results stay, but you will need new data to repeat them.",
  5: "Broker users, the publisher, topic history and the network-listener setting return to defaults.",
  6: "Trained models and comparison are cleared.",
  7: "The network and converted models are cleared. This also resets Lab 9, which depends on them.",
  9: "model_data.h and the on-device results are cleared.",
  10: "Devices, access tokens, alarm rules and alarms are deleted.",
  11: "The Isolation Forest and live-detector counters are cleared.",
  13: "The selected project is cleared.",
  14: "The test log is cleared."
};

Object.assign(A, {
  student: () => { const s = S.student || {}; $("#stName").value = s.name || ""; $("#stReg").value = s.reg || ""; $("#stSec").value = s.section || ""; $("#stErr").hidden = true; openDlg("studentDlg"); },
  studentsave: async () => {
    try {
      S.student = await api("/api/student", { name: $("#stName").value, reg: $("#stReg").value, section: $("#stSec").value });
      closeDlg("studentDlg"); await refresh(); toast("Student details saved");
    } catch (e) { const el = $("#stErr"); el.textContent = e.message; el.hidden = false; }
  },
  dlgclose: el => closeDlg(el.dataset.d),
  about: async () => {
    const a = await api("/api/about"); const ini = a.author.split(" ").map(w => w[0]).slice(0, 2).join("");
    $("#aboutBody").innerHTML = `<div class="who"><div class="av">${esc(ini)}</div><div><b>${esc(a.author)}</b><br><span class="muted small">${esc(a.title)} · ${esc(a.dept)}</span><br><span class="muted small">Course teacher, ${esc(a.course)}</span></div></div>
      <p><b>${esc(a.name)}</b> v${esc(a.version)} — a software replacement for the AI-4301 lab kit: virtual boards and sensors, MQTT broker, ML and TinyML labs, Circuit Lab and PDF lab reports.</p>
      <p class="muted small">${esc(a.copyright)}. All rights reserved.</p><h4>License</h4><pre>${esc(a.license)}</pre>`;
    openDlg("aboutDlg");
  },
  allpdf: async () => { if (needStudent()) return; const n = await download("/api/report.pdf?lab=all"); toast("Saved " + n); },
  labpdf: async el => { if (needStudent()) return; const n = await download("/api/report.pdf?lab=" + el.dataset.n); toast("Saved " + n); },
  labreset: async el => {
    const n = Number(el.dataset.n);
    if (!confirm(`Reset Lab ${n}?\n\nIts results, progress ticks and generated files are cleared so you can do it again. Other labs are not changed.\n${RESET_NOTES[n] ? "\n" + RESET_NOTES[n] : ""}`)) return;
    await api("/api/reset", { scope: "lab", lab: n }); toast(`Lab ${n} reset`);
  },
  resetdlg: () => openDlg("resetDlg"),
  resetall: async () => { closeDlg("resetDlg"); await api("/api/reset", { scope: "all", files: $("#rsFiles").checked, student: $("#rsStud").checked }); toast("Simulator reset to factory state"); },
});

// react to resets (this window or another)
window.onSimReset = async d => {
  DASH.t.length = DASH.h.length = DASH.l.length = 0; DASH.m = 0; DASH.last = null; DASH.n = 0;
  Object.keys(TOPICS).forEach(k => delete TOPICS[k]);
  if (d.scope === "all") { SERIAL.length = 0; const sb = $("#serialbox"); if (sb) sb.innerHTML = ""; for (let i = 0; i < 12; i++) lsSet("aiot.inv" + i, ""); [0, 1, 2].forEach(i => lsSet("aiot.asset" + i, "")); lsSet("aiot.risk", ""); lsSet("aiot.circuit", ""); }
  if (d.scope === "lab" && d.lab === 1) { for (let i = 0; i < 12; i++) lsSet("aiot.inv" + i, ""); [0, 1, 2].forEach(i => lsSet("aiot.asset" + i, "")); }
  if (d.scope === "lab" && d.lab === 13) lsSet("aiot.risk", "");
  if (d.scope === "lab" && d.lab === 2) lsSet("aiot.circuit", "");
  await refresh(); renderEnv(); renderLab();
};

document.addEventListener("DOMContentLoaded", () => {
  initThemes();
  setTimeout(() => { if (S.student && !S.student.name && !sessionStorage.getItem("aiot.askedStudent")) { try { sessionStorage.setItem("aiot.askedStudent", "1"); } catch { } A.student(); } }, 1200);
});
