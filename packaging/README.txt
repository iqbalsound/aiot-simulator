AIoT Simulator v0.0.3  -  AI-4301 Artificial Intelligence of Things
====================================================================
Author and course teacher: Iqbal Uddin Khan, Assistant Professor, DIT | FEST - HU
Copyright (c) 2026 Iqbal Uddin Khan. Educational Use License (see COPYING.txt / LICENSE).

What it is
  One program that replaces the lab kit for Labs 1-16 of the AI-4301 Laboratory Manual:
  virtual Arduino + sensors, a Circuit Lab to wire and run an Uno, a real MQTT broker (port 1883),
  CSV logging and preprocessing, dashboards, ML training, TFLite / TFLite-Micro simulation,
  a ThingsBoard-style platform, anomaly detection, predictive maintenance, mini-project testing,
  benchmarking and PDF lab reports with your name and registration number.
  Fifteen colour themes: pick one from the Theme list in the top bar.

Install
  Debian / Ubuntu / Kubuntu :  sudo apt install ./aiot-simulator_0.0.3_amd64.deb
  Fedora / Rocky / RHEL     :  sudo dnf install ./aiot-simulator-0.0.3-1.x86_64.rpm
  Windows 10/11 (64-bit)    :  run AIoT-Simulator-0.0.3-setup.exe
                               (SmartScreen may warn: the installer is not code-signed. Choose More info > Run anyway.)

First steps
  1. Start "AIoT Simulator" from the application menu (or run: aiot-simulator).
  2. Enter your name and registration number (Student details) - they are printed on every PDF report.
  3. Work through Labs 1-16. Each lab has "PDF" (report) and "Reset lab" (do it again) buttons.
  4. Open the Circuit Lab (top of the lab list) to wire an Arduino Uno and run a sketch.
  5. "All labs report (PDF)" in the sidebar exports the complete portfolio.

Useful options
  aiot-simulator --no-browser --serve     run in the background, open http://127.0.0.1:8765 yourself
  aiot-simulator --mqtt-port 1884         use another MQTT port (default 1883)
  aiot-simulator --workspace DIR          folder for generated files (default ~/AIoT-Simulator)
  aiot-simulator --seed 42                reproducible sensor data
  aiot-simulator --version / --about      version, author and license

Files
  Everything you create (CSV, models, circuits, reports, progress.json, student.json) is stored in
  ~/AIoT-Simulator (Windows: C:\Users\<you>\AIoT-Simulator). "Reset > delete generated files" clears it.

Security notes
  The web interface listens on 127.0.0.1 only. The MQTT broker listens on localhost only until you
  tick "Listen on the local network" in Lab 5. Keep authentication enabled on shared networks.
  If a real Mosquitto already uses port 1883 the simulator falls back to port 18830 and says so in Lab 5.

Uninstall
  sudo apt remove aiot-simulator  |  sudo dnf remove aiot-simulator  |  Windows Settings > Apps.
  Your workspace folder is never deleted.
