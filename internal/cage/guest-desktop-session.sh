#!/usr/bin/env bash
set -euo pipefail
# A full Xfce session for programs that have no pane of their own. It shares
# nothing with Claude Desktop's display and listens on a unix socket only.
# Keep dbus-daemon and other children from announcing readiness for the session.
if [ "${CAGE_GUEST_DESKTOP_SESSION:-}" != 1 ]; then
  export CAGE_GUEST_DESKTOP_SESSION=1 CAGE_NOTIFY_SOCKET="$NOTIFY_SOCKET"
  unset NOTIFY_SOCKET
  exec dbus-run-session -- "$0"
fi
export DISPLAY=:79 XAUTHORITY=/run/cage-guest-desktop/Xauthority
export XDG_RUNTIME_DIR=/run/cage-guest-desktop XDG_CURRENT_DESKTOP=XFCE
# Links opened from the desktop go to the Cage browser, never a second browser
# whose fingerprint Cage does not manage.
export PATH="/usr/local/bin:$HOME/.local/bin:$PATH" BROWSER=/usr/local/bin/cage-open
dbus-update-activation-environment DISPLAY XAUTHORITY XDG_RUNTIME_DIR XDG_CURRENT_DESKTOP BROWSER
umask 077
touch "$XAUTHORITY"
xauth -f "$XAUTHORITY" add "$DISPLAY" . "$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')"
password="$HOME/.config/claude-cage/guest-desktop-password"
if [ ! -s "$password" ]; then
  mkdir -p "$(dirname "$password")"
  od -An -N4 -tx1 /dev/urandom | tr -d ' \n' > "$password"
fi
tigervncpasswd -f < "$password" > "$XDG_RUNTIME_DIR/vnc-password"
# The first session starts at the density the workbench sizes for. Later
# sessions keep whatever the user chose in Xfce's own settings.
xsettings="$HOME/.config/xfce4/xfconf/xfce-perchannel-xml/xsettings.xml"
if [ ! -e "$xsettings" ]; then
  mkdir -p "$(dirname "$xsettings")"
  cat > "$xsettings" <<'CAGE_GUEST_XSETTINGS'
<?xml version="1.0" encoding="UTF-8"?>
<channel name="xsettings" version="1.0">
  <property name="Gdk" type="empty">
    <property name="WindowScalingFactor" type="int" value="2"/>
  </property>
</channel>
CAGE_GUEST_XSETTINGS
fi
# Xfce's own "web browser" launchers resolve through its helper, not $BROWSER.
mkdir -p "$HOME/.local/share/xfce4/helpers" "$HOME/.config/xfce4"
cat > "$HOME/.local/share/xfce4/helpers/cage-browser.desktop" <<'CAGE_GUEST_BROWSER'
[Desktop Entry]
NoDisplay=true
Version=1.0
Type=X-XFCE-Helper
X-XFCE-Category=WebBrowser
X-XFCE-Commands=/usr/local/bin/cage-open about:blank
X-XFCE-CommandsWithParameter=/usr/local/bin/cage-open "%s"
Icon=web-browser
Name=Cage 浏览器
CAGE_GUEST_BROWSER
if ! grep -q '^WebBrowser=' "$HOME/.config/xfce4/helpers.rc" 2>/dev/null; then
  printf 'WebBrowser=cage-browser\n' >> "$HOME/.config/xfce4/helpers.rc"
fi
Xtigervnc "$DISPLAY" -geometry 2560x1600 -depth 24 -rfbport -1 -rfbunixpath "$XDG_RUNTIME_DIR/vnc.sock" -rfbunixmode 0600 -nolisten tcp -auth "$XAUTHORITY" -SecurityTypes VncAuth -PasswordFile "$XDG_RUNTIME_DIR/vnc-password" -AlwaysShared &
display_pid=$!
trap 'kill "$display_pid" 2>/dev/null || true' EXIT
for i in $(seq 1 100); do
  if xdpyinfo -display "$DISPLAY" >/dev/null 2>&1; then break; fi
  kill -0 "$display_pid"
  sleep .1
done
xdpyinfo -display "$DISPLAY" >/dev/null
xfce4-session &
session_pid=$!
# The service becomes ready once the session has a window manager.
for i in $(seq 1 300); do
  kill -0 "$session_pid"
  if xprop -root _NET_SUPPORTING_WM_CHECK 2>/dev/null | grep -q 'window id'; then
    NOTIFY_SOCKET="$CAGE_NOTIFY_SOCKET" systemd-notify --ready
    break
  fi
  sleep .1
done
wait "$session_pid"
