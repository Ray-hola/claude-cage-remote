package workbench

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sylvinus/agent-vm/internal/cage"
	"github.com/sylvinus/agent-vm/internal/diagnostics"
	"github.com/sylvinus/agent-vm/internal/usererror"
)

// The guest desktop is a second RFB endpoint beside Claude Desktop: a full Xfce
// session for programs that have no pane of their own. It is installed and
// started on request, and reuses the same transport: SSH into this profile's
// guest, then socat onto a unix socket. No host port or VNC listener is added.

type guestDesktopState struct {
	Installed     bool   `json:"installed"`
	Running       bool   `json:"running"`
	Failed        bool   `json:"failed"`
	FailureReason string `json:"failure_reason,omitempty"`
}

const guestDesktopSnapshot = `python3 - <<'CAGE_GUEST_DESKTOP_STATE'
import json, subprocess
def run(*args):
    return subprocess.run(args, capture_output=True, text=True).stdout.strip()
installed=run('dpkg-query','-W','-f=${Status}','xfce4-session')=='install ok installed'
state=run('systemctl','is-active','cage-guest-desktop.service')
result=run('systemctl','show','--property=Result','--value','cage-guest-desktop.service') if state=='failed' else ''
print(json.dumps({'installed':installed,'running':state=='active','failed':state=='failed','failure_reason':'oom-kill' if result=='oom-kill' else ''}))
CAGE_GUEST_DESKTOP_STATE`

// Native guests show their own desktop through the platform's screen sharing.
func (s *Server) guestDesktopUnavailable(w http.ResponseWriter) bool {
	if s.macosGuest != nil || s.windowsGuest != nil {
		http.Error(w, "客机桌面目前仅支持 Linux 客机", http.StatusNotImplemented)
		return true
	}
	if !s.vmRunning() {
		http.Error(w, "请先启动 Cage 环境", 409)
		return true
	}
	return false
}

func (s *Server) guestDesktopStatus(w http.ResponseWriter, r *http.Request) {
	if s.guestDesktopUnavailable(w) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	b, err := s.guestQuick(ctx, guestDesktopSnapshot)
	if err != nil {
		_ = diagnostics.New(s.profile).Record("warn", "guest-desktop", fmt.Sprintf("read state: %v", err))
		http.Error(w, usererror.Wrap("读取客机桌面状态", err, "请确认环境仍在运行；重新连接后再打开。").Error(), 502)
		return
	}
	var state guestDesktopState
	if json.Unmarshal(b, &state) != nil {
		http.Error(w, "读取客机桌面状态失败：环境返回的状态格式无效。请更新环境组件。", 502)
		return
	}
	jsonReply(w, state)
}

func (s *Server) guestDesktopAction(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	if action != "install" && action != "start" && action != "stop" {
		http.NotFound(w, r)
		return
	}
	var input struct{}
	if !readJSON(w, r, &input) {
		return
	}
	if s.guestDesktopUnavailable(w) {
		return
	}
	s.launch(w, "guest-desktop-"+action, func() error {
		switch action {
		case "install":
			s.setMessage("正在安装客机桌面，可在操作详情查看进度…")
			return s.runGuestInstall(cage.GuestDesktopInstallScript())
		case "start":
			s.setMessage("正在打开客机桌面…")
			script := `test "$(dpkg-query -W -f='${Status}' xfce4-session 2>/dev/null)" = 'install ok installed' || { echo '请先安装客机桌面。'; exit 1; }
` + cage.GuestDesktopProvisionScript() + "/usr/local/bin/cage-guest-desktop-start\n"
			return s.runDesktopScript(script, 50*time.Second)
		default:
			s.setMessage("正在关闭客机桌面…")
			return s.runDesktopScript("sudo systemctl stop cage-guest-desktop.service", 30*time.Second)
		}
	})
}

func (s *Server) guestDesktopCredentials(w http.ResponseWriter, r *http.Request) {
	if s.guestDesktopUnavailable(w) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	b, err := s.guestQuick(ctx, `set -eu
systemctl is-active --quiet cage-guest-desktop.service
password=$(cat "$HOME/.config/claude-cage/guest-desktop-password")
printf '%s\n' "$password"
sed -n 's/.*name="WindowScalingFactor" type="int" value="\([0-9]\)".*/\1/p' "$HOME/.config/xfce4/xfconf/xfce-perchannel-xml/xsettings.xml" 2>/dev/null | head -n 1`)
	password, scale, _ := strings.Cut(strings.TrimSpace(string(b)), "\n")
	if err != nil || len(password) != 8 {
		http.Error(w, "客机桌面尚未启动", 409)
		return
	}
	// Xfce scales by whole numbers. The viewer sizes the display for the factor
	// the session uses, including one the user picked in Xfce's settings.
	density, _ := strconv.Atoi(strings.TrimSpace(scale))
	if density < 1 || density > 3 {
		density = 2
	}
	jsonReply(w, map[string]any{"password": password, "display_scale": float64(density), "dynamic_scale": false})
}

func (s *Server) guestDesktopResize(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Width  int     `json:"width"`
		Height int     `json:"height"`
		Scale  float64 `json:"scale"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	if input.Width < 320 || input.Height < 240 || input.Width > 8192 || input.Height > 8192 || !validDisplayDensity(input.Scale) {
		http.Error(w, "画面尺寸无效", 400)
		return
	}
	if s.guestDesktopUnavailable(w) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	if _, err := s.guestQuick(ctx, guestDesktopResizeScript(input.Width, input.Height)); err != nil {
		http.Error(w, "客机桌面画面尺寸未能调整", 502)
		return
	}
	jsonReply(w, map[string]any{"ok": true})
}

// Fixed commands and validated numbers only; no host shell executes input.
func guestDesktopResizeScript(wid, hei int) string {
	return `set -eu
export DISPLAY=:79 XAUTHORITY=/run/cage-guest-desktop/Xauthority
exec 9>/run/cage-guest-desktop/resize.lock
flock 9
` + vncModeScript(wid, hei)
}

func (s *Server) guestDesktopVNC(w http.ResponseWriter, r *http.Request) {
	if s.guestDesktopUnavailable(w) {
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	cmd := s.sshCommand(ctx, "-T", s.sshTarget(), "--", "exec socat STDIO UNIX-CONNECT:/run/cage-guest-desktop/vnc.sock")
	reader, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, "无法连接客机桌面", 502)
		return
	}
	writer, err := cmd.StdinPipe()
	if err != nil {
		_ = reader.Close()
		http.Error(w, "无法连接客机桌面", 502)
		return
	}
	if err = cmd.Start(); err != nil {
		_ = reader.Close()
		_ = writer.Close()
		http.Error(w, "无法连接客机桌面", 502)
		return
	}
	defer func() { cancel(); writer.Close(); reader.Close(); _ = cmd.Wait() }()
	s.streamVNC(w, r, reader, writer, func() { cancel(); _ = writer.Close(); _ = reader.Close() })
}
