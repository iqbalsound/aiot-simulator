// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.

// AIoT Simulator — a software replacement for the AI-4301 lab kit.
package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"

	"aiotsim/internal/app"
	"aiotsim/internal/web"
)

// Version is overridden at build time with -ldflags "-X main.Version=...".
var Version = "0.0.1"

func openBrowser(url string) {
	var cmds [][]string
	switch runtime.GOOS {
	case "windows":
		for _, p := range []string{
			os.Getenv("ProgramFiles(x86)") + `\Microsoft\Edge\Application\msedge.exe`,
			os.Getenv("ProgramFiles") + `\Microsoft\Edge\Application\msedge.exe`,
			os.Getenv("ProgramFiles") + `\Google\Chrome\Application\chrome.exe`,
		} {
			if _, err := os.Stat(p); err == nil {
				cmds = append(cmds, []string{p, "--app=" + url, "--window-size=1360,880"})
				break
			}
		}
		cmds = append(cmds, []string{"rundll32", "url.dll,FileProtocolHandler", url})
	case "darwin":
		cmds = append(cmds, []string{"open", url})
	default:
		for _, b := range []string{"google-chrome", "chromium", "chromium-browser", "microsoft-edge"} {
			if p, err := exec.LookPath(b); err == nil {
				cmds = append(cmds, []string{p, "--app=" + url, "--window-size=1360,880"})
				break
			}
		}
		cmds = append(cmds, []string{"xdg-open", url})
	}
	for _, c := range cmds {
		if exec.Command(c[0], c[1:]...).Start() == nil {
			return
		}
	}
	fmt.Println("Open this address in your browser:", url)
}

func main() {
	port := flag.Int("port", 8765, "port for the simulator web interface (127.0.0.1 only)")
	mqttPort := flag.Int("mqtt-port", 1883, "port for the built-in MQTT broker")
	ws := flag.String("workspace", "", "folder for generated lab files (default ~/AIoT-Simulator)")
	noBrowser := flag.Bool("no-browser", false, "do not open a browser window")
	serve := flag.Bool("serve", false, "keep running after the browser window is closed")
	seed := flag.Int64("seed", 0, "random seed for reproducible sensor data (0 = random)")
	ver := flag.Bool("version", false, "print version and exit")
	about := flag.Bool("about", false, "print author and license information and exit")
	flag.Parse()
	if *ver {
		fmt.Printf("aiot-simulator %s\nCopyright (c) 2026 %s, %s, %s\n", Version, app.Author, app.AuthorTitle, app.AuthorDept)
		return
	}
	if *about {
		fmt.Printf("AIoT Simulator %s - companion software for %s %s\nAuthor and course teacher: %s, %s, %s\n\n%s", Version, app.CourseCode, app.CourseName, app.Author, app.AuthorTitle, app.AuthorDept, app.LicenseText())
		return
	}
	dir := *ws
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		dir = filepath.Join(home, "AIoT-Simulator")
	}
	a := app.New(Version, dir, *seed)
	a.StartBroker(*mqttPort, false)

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		if ln, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
			fmt.Fprintln(os.Stderr, "cannot start web server:", err)
			os.Exit(1)
		}
	}
	url := "http://" + ln.Addr().String()
	a.Addr = ln.Addr().String()
	srv := &http.Server{Handler: a.Handler(web.FS()), ReadHeaderTimeout: 10 * time.Second}
	go srv.Serve(ln)
	fmt.Printf("AIoT Simulator %s  (c) 2026 %s\n  Interface : %s\n  Workspace : %s\n", Version, app.Author, url, dir)
	if !*noBrowser {
		openBrowser(url)
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-sig:
			return
		case <-a.Quit():
			time.Sleep(200 * time.Millisecond)
			return
		case <-tick.C:
			if *serve {
				continue
			}
			pinged, since := a.Idle()
			if (pinged && since > 40*time.Second) || (!pinged && since > 3*time.Minute) {
				return
			}
		}
	}
}
