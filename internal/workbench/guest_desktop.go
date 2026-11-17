package workbench

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// The guest desktop is a second RFB endpoint beside Claude Desktop. It carries a
// full Linux desktop session that the guest starts on demand, and it reuses the
// same transport: SSH into this profile's guest, then socat onto a unix socket.
// No host port and no public VNC listener is added.

const (
	// guestDesktopSocket is created by the guest session unit.
	guestDesktopSocket = "/run/cage-guest-desktop/vnc.sock"
	// guestDesktopPassword is written by the same unit, 8 characters like the
	// Claude Desktop password so the viewer needs no special case.
	guestDesktopPassword = "$HOME/.config/claude-cage/guest-desktop-password"
	// guestDesktopUnit is the guest-side session that owns the display.
	guestDesktopUnit = "cage-guest-desktop.service"
)

// guestDesktopSupported reports whether this build can show a guest desktop.
// Native guests already render their own desktop through another route.
func (s *Server) guestDesktopSupported() bool {
	return s.macosGuest == nil && s.windowsGuest == nil
}

func (s *Server) guestDesktopCredentials(w http.ResponseWriter, r *http.Request) {
	if !s.guestDesktopSupported() {
		http.Error(w, "客机桌面目前仅支持 Linux 客机", http.StatusNotImplemented)
		return
	}
	if !s.vmRunning() {
		http.Error(w, "环境尚未启动", http.StatusConflict)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	b, err := s.guestQuick(ctx, `set -eu
systemctl is-active --quiet `+guestDesktopUnit+`
password=$(cat "`+guestDesktopPassword+`")
printf '%s\n' "$password"`)
	password := strings.TrimSpace(string(b))
	if err != nil || len(password) != 8 {
		http.Error(w, "客机桌面尚未启动", http.StatusConflict)
		return
	}
	// The session owns its own resolution; the viewer follows it instead of
	// asking the guest to rescale.
	jsonReply(w, map[string]any{"password": password, "display_scale": 1.0, "dynamic_scale": false})
}

// guestDesktopResize accepts the viewer's size sync without changing the guest.
// A full desktop is shared by more than one window, so the viewer follows the
// session's resolution rather than driving it.
func (s *Server) guestDesktopResize(w http.ResponseWriter, r *http.Request) {
	if !s.guestDesktopSupported() || !s.vmRunning() {
		http.Error(w, "客机桌面尚未启动", http.StatusConflict)
		return
	}
	jsonReply(w, map[string]any{"applied": false})
}

func (s *Server) guestDesktopVNC(w http.ResponseWriter, r *http.Request) {
	if !s.guestDesktopSupported() {
		http.Error(w, "客机桌面目前仅支持 Linux 客机", http.StatusNotImplemented)
		return
	}
	if !s.vmRunning() {
		http.Error(w, "环境尚未启动", http.StatusConflict)
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	cmd := s.sshCommand(ctx, "-T", s.sshTarget(), "--", "exec socat STDIO UNIX-CONNECT:"+guestDesktopSocket)
	reader, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, "无法连接客机桌面", http.StatusBadGateway)
		return
	}
	writer, err := cmd.StdinPipe()
	if err != nil {
		_ = reader.Close()
		http.Error(w, "无法连接客机桌面", http.StatusBadGateway)
		return
	}
	if err = cmd.Start(); err != nil {
		_ = reader.Close()
		_ = writer.Close()
		http.Error(w, "无法连接客机桌面", http.StatusBadGateway)
		return
	}
	defer func() { cancel(); writer.Close(); reader.Close(); _ = cmd.Wait() }()
	s.streamVNC(w, r, reader, writer, func() { cancel(); _ = writer.Close(); _ = reader.Close() })
}
