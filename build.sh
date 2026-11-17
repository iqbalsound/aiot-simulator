#!/usr/bin/env bash
# Builds the AIoT Simulator and its installers: DEB, RPM (x86_64) and Windows (NSIS).
# Needs: go >= 1.22, dpkg-deb, fakeroot, rpmbuild, makensis (nsis), python3 + Pillow.
#   Ubuntu/Debian: sudo apt install golang-go dpkg-dev fakeroot rpm nsis python3-pil
set -euo pipefail
cd "$(dirname "$0")"
V="${1:-$(cat VERSION)}"; P=packaging; B=build
PV="${V//-/\~}"   # pre-release tags: 0.1.0-rc1 -> 0.1.0~rc1 (valid for deb and rpm)
case "$V" in [0-9]*.[0-9]*.[0-9]*) ;; *) echo "version must look like 1.2.3 (got $V)"; exit 1;; esac
export CGO_ENABLED=0 GOFLAGS=-mod=mod
rm -rf "$B" dist; mkdir -p "$B/linux" "$B/win" dist
go vet ./... && go test ./...
LD="-s -w -X main.Version=$V"
GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags "$LD"            -o "$B/linux/aiot-simulator"     ./cmd/aiot-simulator
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$LD -H windowsgui" -o "$B/win/aiot-simulator.exe" ./cmd/aiot-simulator
python3 "$P/make_icons.py" "$B"
# ---- DEB
D="$B/deb/aiot-simulator_${PV}_amd64"; mkdir -p "$D/DEBIAN" "$D/usr/bin" "$D/usr/share/applications" \
  "$D/usr/share/icons/hicolor/scalable/apps" "$D/usr/share/icons/hicolor/256x256/apps" "$D/usr/share/doc/aiot-simulator"
install -m 0755 "$B/linux/aiot-simulator" "$D/usr/bin/"
install -m 0644 "$P/aiot-simulator.desktop" "$D/usr/share/applications/"
install -m 0644 internal/web/static/icon.svg "$D/usr/share/icons/hicolor/scalable/apps/aiot-simulator.svg"
install -m 0644 "$B/aiot-simulator.png" "$D/usr/share/icons/hicolor/256x256/apps/"
install -m 0644 "$P/README.txt" "$D/usr/share/doc/aiot-simulator/README"
install -m 0644 "$P/COPYING.txt" "$D/usr/share/doc/aiot-simulator/copyright"
cat > "$D/DEBIAN/control" <<EOT
Package: aiot-simulator
Version: $PV
Section: education
Priority: optional
Architecture: amd64
Installed-Size: $(du -sk "$D/usr" | cut -f1)
Recommends: xdg-utils, chromium | chromium-browser | google-chrome-stable | microsoft-edge-stable | firefox
Maintainer: AI-4301 Course Team <noreply@localhost>
Description: Simulator for the AI-4301 Artificial Intelligence of Things labs
 Replaces the lab kit with software: virtual Arduino and DHT11/PIR/LDR sensors,
 a built-in MQTT 3.1.1 broker, CSV logging and preprocessing, dashboards,
 machine learning and TinyML simulation, a ThingsBoard-style platform,
 anomaly detection, predictive maintenance and mini-project benchmarking.
 Self-contained program with a local web interface (127.0.0.1 only).
EOT
for s in postinst postrm; do cat > "$D/DEBIAN/$s" <<'EOT'
#!/bin/sh
set -e
command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor || true
command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database -q /usr/share/applications || true
exit 0
EOT
chmod 0755 "$D/DEBIAN/$s"; done
fakeroot dpkg-deb --root-owner-group -Zxz --build "$D" "dist/aiot-simulator_${PV}_amd64.deb"
# ---- RPM
R="$PWD/$B/rpm"; mkdir -p "$R"/{SOURCES,SPECS,BUILD,RPMS,SRPMS}
cp "$B/linux/aiot-simulator" "$P/aiot-simulator.desktop" "$B/aiot-simulator.png" "$P/README.txt" "$P/COPYING.txt" "$R/SOURCES/"
cp internal/web/static/icon.svg "$R/SOURCES/aiot-simulator.svg"
sed "s/^Version:.*/Version:        $PV/" "$P/aiot-simulator.spec" > "$R/SPECS/aiot-simulator.spec"
rpmbuild -bb --quiet --define "_topdir $R" --target x86_64 "$R/SPECS/aiot-simulator.spec"
cp "$R"/RPMS/x86_64/*.rpm dist/
# ---- Windows installer
W="$B/winpkg"; mkdir -p "$W"
cp "$B/win/aiot-simulator.exe" "$B/win/aiot-simulator.ico" "$P/installer.nsi" "$P/README.txt" "$P/COPYING.txt" "$W/"
NUMV="${V%%-*}.0"
sed -i -e "s/!define VER \".*\"/!define VER \"$V\"/" -e "s/VIProductVersion \".*\"/VIProductVersion \"$NUMV\"/" "$W/installer.nsi"
(cd "$W" && makensis -V2 installer.nsi)
cp "$W/AIoT-Simulator-${V}-setup.exe" "$P/README.txt" dist/
(cd dist && sha256sum -- * | grep -v README > SHA256SUMS)
echo; ls -la dist
