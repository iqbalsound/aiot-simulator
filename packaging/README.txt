AIoT Simulator v0.0.1  -  AI-4301 Artificial Intelligence of Things
====================================================================

What it is
  A single program that replaces the lab kit for Labs 1-16 of the AI-4301 Laboratory Manual:
  virtual Arduino + DHT11/PIR/LDR sensors, a real MQTT broker (port 1883), CSV logging and
  preprocessing, dashboards, ML training, TFLite/int8 and TFLite-Micro simulation, ThingsBoard-style
  platform, anomaly detection, predictive maintenance, mini-project test log and benchmarking.

Install
  Debian / Ubuntu / Kubuntu :  sudo apt install ./aiot-simulator_0.0.1_amd64.deb
  Fedora / Rocky / RHEL     :  sudo dnf install ./aiot-simulator-0.0.1-1.x86_64.rpm
  Windows 10/11 (64-bit)    :  run AIoT-Simulator-0.0.1-setup.exe
                               (SmartScreen may warn: the installer is not code-signed. Choose More info > Run anyway.)

Run
  Start "AIoT Simulator" from the application menu, or run  aiot-simulator  in a terminal.
  A window opens (Chrome/Edge app mode when available, otherwise your default browser).
  The program stops by itself shortly after you close the window, or use "Quit simulator".

Useful options
  aiot-simulator --no-browser --serve     run in the background, open http://127.0.0.1:8765 yourself
  aiot-simulator --mqtt-port 1884         use another MQTT port (default 1883)
  aiot-simulator --workspace DIR          folder for generated files (default ~/AIoT-Simulator)
  aiot-simulator --seed 42                reproducible sensor data
  aiot-simulator --version

Files
  Everything you create (sensor_log.csv, models, test_log.csv, progress.json, ...) is stored in
  ~/AIoT-Simulator (Windows: C:\Users\<you>\AIoT-Simulator).

Security notes
  The web interface listens on 127.0.0.1 only. The MQTT broker listens on localhost only until you
  tick "Listen on the local network" in Lab 5. Keep authentication enabled on shared networks.
  If a real Mosquitto already uses port 1883 the simulator falls back to port 18830 and says so in Lab 5.

Uninstall
  sudo apt remove aiot-simulator  |  sudo dnf remove aiot-simulator  |  Windows Settings > Apps.
  Your workspace folder is never deleted.
