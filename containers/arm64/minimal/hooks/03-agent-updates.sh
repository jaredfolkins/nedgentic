#!/bin/sh
set -e

NED_PREFIX="$(printf '\033[38;5;180m[\033[38;5;15m\342\214\220\033[38;5;251m\342\227\257\033[38;5;15m-\033[38;5;251m\342\227\257\033[38;5;180m]\033[0m')"
LOG_PATH=/var/log/ned-agent-update.log
LOCK_DIR=/run/ned-agent-update.lock

# One background update per boot; later attaches reuse the running or finished pass.
if ! mkdir "$LOCK_DIR" 2>/dev/null; then
  exit 0
fi

echo "${NED_PREFIX} Checking for agent CLI updates in the background (log: $LOG_PATH)"
nohup setsid /usr/local/bin/ned-agent-update >"$LOG_PATH" 2>&1 </dev/null &
