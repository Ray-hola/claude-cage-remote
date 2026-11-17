#!/usr/bin/env bash
set -euo pipefail
if systemctl is-active --quiet cage-guest-desktop.service; then exit 0; fi
if systemctl is-failed --quiet cage-guest-desktop.service; then
  sudo systemctl reset-failed cage-guest-desktop.service
fi
sudo systemctl start --no-block cage-guest-desktop.service
for i in $(seq 1 300); do
  if systemctl is-active --quiet cage-guest-desktop.service; then exit 0; fi
  if systemctl is-failed --quiet cage-guest-desktop.service; then
    echo '客机桌面未能启动。' >&2; exit 1
  fi
  sleep .1
done
echo '客机桌面启动超时。' >&2
exit 1
