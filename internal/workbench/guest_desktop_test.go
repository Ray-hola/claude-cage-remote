package workbench

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGuestDesktopSnapshot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("snapshot executes in the Linux guest; fixture requires POSIX executables")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 unavailable")
	}
	dir := t.TempDir()
	for name, script := range map[string]string{
		"dpkg-query": "#!/bin/sh\nprintf '%s' \"$TEST_PACKAGE\"\n",
		"systemctl":  "#!/bin/sh\nif [ \"$1\" = is-active ]; then printf '%s\\n' \"$TEST_ACTIVE\"; else printf '%s\\n' \"$TEST_RESULT\"; fi\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		pkg, active, result string
		want                guestDesktopState
	}{
		{"", "inactive", "", guestDesktopState{}},
		{"install ok installed", "active", "", guestDesktopState{Installed: true, Running: true}},
		{"install ok installed", "failed", "oom-kill", guestDesktopState{Installed: true, Failed: true, FailureReason: "oom-kill"}},
		{"deinstall ok config-files", "failed", "exit-code", guestDesktopState{Failed: true}},
	} {
		cmd := exec.Command("sh", "-c", guestDesktopSnapshot)
		cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "TEST_PACKAGE="+tc.pkg, "TEST_ACTIVE="+tc.active, "TEST_RESULT="+tc.result)
		out, err := cmd.CombinedOutput()
		var state guestDesktopState
		if err != nil || json.Unmarshal(out, &state) != nil {
			t.Fatalf("snapshot: %v %s", err, out)
		}
		if state != tc.want {
			t.Fatalf("%+v: %+v", tc, state)
		}
	}
}

func TestGuestDesktopControlBoundary(t *testing.T) {
	s := testServer(t)
	for _, path := range []string{"/api/guest-desktop/install", "/api/guest-desktop/start", "/api/guest-desktop/stop"} {
		if w := request(s, "POST", path, s.origin, "{}", false); w.Code != 401 {
			t.Fatalf("%s without session: %d", path, w.Code)
		}
		if w := request(s, "POST", path, "https://claude.ai", "{}", true); w.Code != 403 {
			t.Fatalf("%s cross-origin: %d", path, w.Code)
		}
		if w := request(s, "POST", path, s.origin, `{"command":"touch /tmp/untrusted"}`, true); w.Code != 400 {
			t.Fatalf("%s accepted arbitrary command: %d", path, w.Code)
		}
		if w := request(s, "POST", path, s.origin, "{}", true); w.Code != 409 {
			t.Fatalf("%s must require running VM: %d", path, w.Code)
		}
	}
	if w := request(s, "POST", "/api/guest-desktop/update", s.origin, "{}", true); w.Code != 404 {
		t.Fatalf("unknown action: %d", w.Code)
	}
	for _, body := range []string{`{"width":1,"height":1000}`, `{"width":8193,"height":1000}`, `{"width":"1024;command","height":768}`, `{"width":1000,"height":800,"scale":4.5}`} {
		if w := request(s, "POST", "/api/guest-desktop/resize", s.origin, body, true); w.Code != 400 {
			t.Fatalf("invalid geometry: %d", w.Code)
		}
	}
	for _, path := range []string{"/api/guest-desktop", "/api/guest-desktop/vnc-credentials"} {
		if w := request(s, "GET", path, s.origin, "", true); w.Code != 409 {
			t.Fatalf("%s must require running VM: %d", path, w.Code)
		}
	}
}

func TestGuestDesktopResizeTargetsItsOwnDisplay(t *testing.T) {
	script := guestDesktopResizeScript(1280, 800)
	if !strings.Contains(script, "DISPLAY=:79") || strings.Contains(script, "/run/cage-desktop/") || !strings.Contains(script, "name=cage-1280x800") {
		t.Fatal(script)
	}
}
