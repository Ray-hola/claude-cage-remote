package cage

import _ "embed"

//go:embed guest-desktop-install.sh
var guestDesktopInstall string

//go:embed guest-desktop-session.sh
var guestDesktopSession string

//go:embed guest-desktop-start.sh
var guestDesktopStart string

func GuestDesktopInstallScript() string {
	return ToolInstallPrelude() + guestDesktopInstall + "\n" + GuestDesktopProvisionScript()
}

// Like Claude Desktop, the session is installed on request and never enabled
// at boot. Only a user who opens the guest desktop pays for its memory.
func GuestDesktopProvisionScript() string {
	return "set -euo pipefail\n" +
		"sudo tee /usr/local/bin/cage-guest-desktop-session >/dev/null <<'CAGE_GUEST_DESKTOP_SESSION'\n" + guestDesktopSession + "\nCAGE_GUEST_DESKTOP_SESSION\n" +
		"sudo tee /usr/local/bin/cage-guest-desktop-start >/dev/null <<'CAGE_GUEST_DESKTOP_START'\n" + guestDesktopStart + "\nCAGE_GUEST_DESKTOP_START\n" +
		"sudo chmod 0755 /usr/local/bin/cage-guest-desktop-session /usr/local/bin/cage-guest-desktop-start\n" + `sudo tee /etc/systemd/system/cage-guest-desktop.service >/dev/null <<'CAGE_GUEST_DESKTOP_UNIT'
[Unit]
Description=Guest desktop in Cage
[Service]
Type=notify
NotifyAccess=all
User=cage
WorkingDirectory=/home/cage
Environment=HOME=/home/cage
EnvironmentFile=/etc/claude-cage/environment
RuntimeDirectory=cage-guest-desktop
RuntimeDirectoryMode=0700
ExecStart=/usr/local/bin/cage-log-run /usr/local/bin/cage-guest-desktop-session
LogNamespace=cage
KillMode=control-group
TimeoutStartSec=45
TimeoutStopSec=20
CAGE_GUEST_DESKTOP_UNIT
sudo systemctl daemon-reload
`
}
