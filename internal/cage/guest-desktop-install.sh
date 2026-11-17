#!/usr/bin/env bash
set -euo pipefail
mkdir -p "$HOME/.config/claude-cage"
exec 9>"$HOME/.config/claude-cage/guest-desktop-install.lock"
flock -n 9 || { echo '客机桌面正在安装。' >&2; exit 1; }
if systemctl is-active --quiet cage-guest-desktop.service; then
  echo '请先保存工作并关闭客机桌面，再修复安装。' >&2
  exit 1
fi
printf '正在安装客机桌面…\n'
sudo env DEBIAN_FRONTEND=noninteractive APT_CONFIG="${CAGE_APT_CONFIG:-/dev/null}" apt-get -o DPkg::Lock::Timeout=60 update </dev/null
# A small Xfce without recommends: no display manager, office suite or browser.
sudo env DEBIAN_FRONTEND=noninteractive APT_CONFIG="${CAGE_APT_CONFIG:-/dev/null}" apt-get -o DPkg::Lock::Timeout=60 install -y --no-install-recommends dbus-x11 xauth x11-utils x11-xserver-utils tigervnc-standalone-server tigervnc-tools socat xfce4-session xfwm4 xfce4-panel xfdesktop4 xfce4-settings xfconf thunar xfce4-terminal adwaita-icon-theme </dev/null
test "$(dpkg-query -W -f='${Status}' xfce4-session)" = 'install ok installed'
printf '客机桌面已安装。\n'
