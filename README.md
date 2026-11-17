# AIoT Simulator

Software replacement for the **AI-4301 Artificial Intelligence of Things** lab kit. One self-contained
program (Go, no runtime dependencies) with a local web interface that simulates Labs 1-16 of the
Laboratory Manual: virtual Arduino and sensors, a real MQTT 3.1.1 broker, CSV logging and
preprocessing, dashboards, machine learning, TFLite / TFLite-Micro simulation, a ThingsBoard-style
platform, anomaly detection, predictive maintenance, mini-project testing and benchmarking.

> Educational tool. Timings, memory and file sizes are estimates. It supplements, and does not
> replace, supervised work on real hardware.

## Try it without installing (dry run)

Requires Go 1.22+ only.

```bash
git clone git@github.com:<you>/aiot-simulator.git && cd aiot-simulator
make run                      # or: go run ./cmd/aiot-simulator
make test                     # unit + API tests with the race detector
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
internal/web/static/  browser UI (plain HTML/CSS/JS, no external requests)
packaging/            .desktop file, RPM spec, NSIS script, icons, README/COPYING for installers
build.sh              reproducible DEB + RPM + Windows build
```

## Security notes

The web UI listens on 127.0.0.1 only, rejects foreign `Host` headers (DNS rebinding) and requires a custom
header on every POST. The MQTT broker listens on localhost until "Listen on the local network" is
ticked in Lab 5; keep authentication on for shared networks.

## Licence

Placeholder: all rights reserved (see `packaging/COPYING.txt`). Add a `LICENSE` file of your choice
before making the repository public.
