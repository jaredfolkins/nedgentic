#!/bin/sh
set -e

USER_NAME=${USER_NAME:-user}
USER_UID=${USER_UID:-1000}
USER_GID=${USER_GID:-1000}
HOME_DIR=${HOME_DIR:-/home/$USER_NAME}
SHELL_PATH=${SHELL_PATH:-/bin/sh}

TERM_VALUE=${TERM:-xterm-256color}
COLORTERM_VALUE=${COLORTERM:-truecolor}
export TERM="$TERM_VALUE"
export COLORTERM="$COLORTERM_VALUE"
if [ -d /etc/profile.d ]; then
  cat <<EOF >/etc/profile.d/ned-term.sh
export TERM=$TERM_VALUE
export COLORTERM=$COLORTERM_VALUE
EOF
fi

mkdir -p /home

HOST_NAME=$(hostname 2>/dev/null || true)
if [ -n "$HOST_NAME" ] && [ -w /etc/hosts ]; then
  if ! grep -qE "^[[:space:]]*127\\.0\\.1\\.1[[:space:]]+$HOST_NAME(\\s|$)" /etc/hosts 2>/dev/null; then
    echo "127.0.1.1 $HOST_NAME" >> /etc/hosts
  fi
fi
