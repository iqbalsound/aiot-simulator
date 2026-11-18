%global debug_package %{nil}
%global __os_install_post %{nil}
%define _binary_payload w9.gzdio
%define _source_payload w9.gzdio
Name:           aiot-simulator
Version:        0.0.1
Release:        1
Summary:        Simulator for the AI-4301 Artificial Intelligence of Things labs (by Iqbal Uddin Khan)
License:        Educational Use License (see LICENSE)
Packager:       Iqbal Uddin Khan
URL:            https://localhost/
BuildArch:      x86_64
Requires:       xdg-utils
AutoReqProv:    no

%description
AIoT Simulator replaces the AI-4301 lab kit with software: virtual Arduino and sensors, a built-in
MQTT 3.1.1 broker, data logging and preprocessing, dashboards, machine learning and TinyML
simulation, a ThingsBoard-style platform, anomaly detection, predictive maintenance and
mini-project benchmarking. A single self-contained program with a local web interface.

%install
rm -rf %{buildroot}
install -D -m 0755 %{_sourcedir}/aiot-simulator %{buildroot}%{_bindir}/aiot-simulator
install -D -m 0644 %{_sourcedir}/aiot-simulator.desktop %{buildroot}%{_datadir}/applications/aiot-simulator.desktop
install -D -m 0644 %{_sourcedir}/aiot-simulator.svg %{buildroot}%{_datadir}/icons/hicolor/scalable/apps/aiot-simulator.svg
install -D -m 0644 %{_sourcedir}/aiot-simulator.png %{buildroot}%{_datadir}/icons/hicolor/256x256/apps/aiot-simulator.png
install -D -m 0644 %{_sourcedir}/README.txt %{buildroot}%{_docdir}/%{name}/README.txt
install -D -m 0644 %{_sourcedir}/COPYING.txt %{buildroot}%{_datadir}/licenses/%{name}/LICENSE

%post
gtk-update-icon-cache -q -t -f %{_datadir}/icons/hicolor >/dev/null 2>&1 || :
update-desktop-database -q %{_datadir}/applications >/dev/null 2>&1 || :

%postun
gtk-update-icon-cache -q -t -f %{_datadir}/icons/hicolor >/dev/null 2>&1 || :
update-desktop-database -q %{_datadir}/applications >/dev/null 2>&1 || :

%files
%{_bindir}/aiot-simulator
%{_datadir}/applications/aiot-simulator.desktop
%{_datadir}/icons/hicolor/scalable/apps/aiot-simulator.svg
%{_datadir}/icons/hicolor/256x256/apps/aiot-simulator.png
%doc %{_docdir}/%{name}/README.txt
%license %{_datadir}/licenses/%{name}/LICENSE

%changelog
* Tue Nov 17 2026 Iqbal Uddin Khan - 0.0.2-1
- Circuit Lab, PDF lab reports, resets, themes, author and licence information
* Mon Nov 16 2026 Iqbal Uddin Khan - 0.0.1-1
- Initial release
