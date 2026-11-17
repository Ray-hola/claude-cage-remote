package cage

import (
	"os/exec"
	"strings"
	"testing"
)

func TestGuestDesktopScriptsParse(t *testing.T) {
	for name, script := range map[string]string{"install": GuestDesktopInstallScript(), "provision": GuestDesktopProvisionScript(), "session": guestDesktopSession, "start": guestDesktopStart} {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command("bash", "-n")
			cmd.Stdin = strings.NewReader(script)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%v: %s", err, out)
			}
		})
	}
}

func TestGuestDesktopStaysOffUntilOpened(t *testing.T) {
	script := GuestDesktopProvisionScript()
	if strings.Contains(script, "systemctl enable") || strings.Contains(script, "WantedBy=") {
		t.Fatal("the guest desktop must not start with the environment")
	}
	// No TCP listener: the workbench reaches the session through SSH only.
	for _, want := range []string{"-rfbport -1", "-nolisten tcp", "-SecurityTypes VncAuth", "BROWSER=/usr/local/bin/cage-open", "WebBrowser=cage-browser"} {
		if !strings.Contains(guestDesktopSession, want) {
			t.Fatalf("session lacks %q", want)
		}
	}
	// The browser stays the Cage browser; the desktop installs none of its own.
	for _, browser := range []string{"firefox", "chromium", "epiphany", "xfce4-goodies", " xfce4 "} {
		if strings.Contains(guestDesktopInstall, browser) {
			t.Fatalf("install pulls in %q", browser)
		}
	}
}
