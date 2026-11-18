# AIoT Simulator

Software replacement for the **AI-4301 Artificial Intelligence of Things** lab kit. One self-contained
program (Go, no runtime dependencies) with a local web interface that simulates Labs 1-16 of the
Laboratory Manual: virtual Arduino and sensors, a real MQTT 3.1.1 broker, CSV logging and
preprocessing, dashboards, machine learning, TFLite / TFLite-Micro simulation, a ThingsBoard-style
platform, anomaly detection, predictive maintenance, mini-project testing and benchmarking.

Author and course teacher: **Iqbal Uddin Khan**, Assistant Professor, DIT | FEST - HU.

> Educational tool. Timings, memory and file sizes are estimates. It supplements, and does not
> replace, supervised work on real hardware.

## Highlights (v0.0.2)

- **Circuit Lab**: drag parts, draw wires, run Arduino sketches on a simulated Uno ([guide](docs/CIRCUIT-LAB.md)).
- **PDF lab reports** for every lab (and a full portfolio) with the student's **name and registration number**,
  evidence, results, charts, a reflection prompt and an instructor marks box.
- **Reset** the whole simulator or any single lab so tasks can be redone.
- **9 themes** (Ocean Dark, Daylight, Navy & Purple Neon, Solarized, Forest, Sunset, High Contrast, Paper, Auto).
- About dialog, author, licence and copyright information (`aiot-simulator --about`).

## Try it without installing (dry run)

Requires Go 1.22+ only.

```bash
git clone git@github.com:<you>/aiot-simulator.git && cd aiot-simulator
make run                      # or: go run ./cmd/aiot-simulator
make test                     # Go tests (race detector) + JavaScript tests
```

Useful flags: `--no-browser --serve` (run headless, open http://127.0.0.1:8765 yourself),
`--mqtt-port 1884`, `--workspace DIR`, `--seed 42`, `--version`.
Generated files (CSV, models, test log, progress) go to `~/AIoT-Simulator`.

## Install

Download the files from the latest **Releases** page (or from a CI run's *installers* artifact):

| System | File | Command |
|---|---|---|
| Debian / Ubuntu / Kubuntu | `aiot-simulator_<ver>_amd64.deb` | `sudo apt install ./aiot-simulator_*_amd64.deb` |
| Fedora / Rocky / RHEL | `aiot-simulator-<ver>-1.x86_64.rpm` | `sudo dnf install ./aiot-simulator-*.x86_64.rpm` |
| Windows 10/11 64-bit | `AIoT-Simulator-<ver>-setup.exe` | double-click (unsigned: SmartScreen > More info > Run anyway) |

Verify downloads with `sha256sum -c SHA256SUMS`.

## Build the installers

On GitHub: see [CI and releases](#ci-and-releases). Locally (Ubuntu/Debian host):

```bash
sudo apt install golang-go dpkg-dev fakeroot rpm nsis python3-pil
./build.sh            # version comes from the VERSION file; output in ./dist
./build.sh 0.1.0      # or pass a version
```

## CI and releases

| Workflow | Trigger | What it does |
|---|---|---|
| `ci.yml` | push to `main`, pull requests, manual | gofmt, vet, tests (race detector) -> builds DEB, RPM and Windows installer -> smoke-tests them: DEB on Ubuntu, RPM in a Rocky Linux 9 container, installer on a Windows runner (silent install, start, API check, uninstall) -> uploads the *installers* artifact |
| `release.yml` | push of a tag `v*` | builds with the version taken from the tag and publishes a GitHub Release with all files |

Make a release:

```bash
echo 0.0.2 > VERSION && git commit -am "Release 0.0.2"
git tag v0.0.2 && git push origin main v0.0.2
```

## Layout

```
cmd/aiot-simulator/   program entry point (flags, browser launch, auto-exit)
internal/sim/         sensors, wiring faults, virtual Arduino
internal/mqtt/        MQTT 3.1.1 broker and client
internal/ml/          decision tree, forest, k-NN, neural net + int8, Isolation Forest, rolling features
internal/app/         HTTP API and lab logic (Labs 1-16), progress tracking, evidence report
internal/pdf/         dependency-free PDF writer used for lab reports
internal/web/static/  browser UI (plain HTML/CSS/JS, no external requests); cint.js = sketch interpreter,
                      circuit-sim.js = electrical model, circuit-ui.js = Circuit Lab editor
web-tests/            JavaScript tests (node --test)
packaging/            .desktop file, RPM spec, NSIS script, icons, README/COPYING for installers
build.sh              reproducible DEB + RPM + Windows build
```

## Security notes

The web UI listens on 127.0.0.1 only, rejects foreign `Host` headers (DNS rebinding) and requires a custom
header on every POST. The MQTT broker listens on localhost until "Listen on the local network" is
ticked in Lab 5; keep authentication on for shared networks.

## Licence and authorship

Copyright (c) 2026 Iqbal Uddin Khan. Released under the **Educational Use License (v1)** in [`LICENSE`](LICENSE):
free for students and staff of the course, redistribution of unmodified copies allowed for non-commercial teaching,
modification and commercial use only with the author's written permission. It is a custom licence, not legal advice:
if you prefer an open-source licence (MIT, Apache-2.0, GPL), replace `LICENSE` and run `./build.sh`, which copies it
into `internal/app/license.txt` and `packaging/COPYING.txt` (a test fails if the copies drift apart).
The program contains no third-party code besides the Go standard library (BSD-3-Clause).
