#!/bin/sh
set -e

USER_NAME=${USER_NAME:-user}
USER_UID=${USER_UID:-1000}
USER_GID=${USER_GID:-1000}
NED_PREFIX="$(printf '\033[38;5;180m[\033[38;5;15m\342\214\220\033[38;5;251m\342\227\257\033[38;5;15m-\033[38;5;251m\342\227\257\033[38;5;180m]\033[0m')"
HOME_DIR=${HOME_DIR:-/home/$USER_NAME}
SHELL_PATH=${SHELL_PATH:-/bin/bash}

echo "${NED_PREFIX} Bootstrapping user: '$USER_NAME' ($USER_UID:$USER_GID) on Debian/Ubuntu"

mkdir -p /home

# User creation logic
if ! id -u "$USER_NAME" >/dev/null 2>&1; then
  echo "${NED_PREFIX} Creating user: $USER_NAME"
  
  # Determine group name
  GROUP_NAME=""
  if command -v getent >/dev/null 2>&1; then
    GROUP_NAME=$(getent group "$USER_GID" | cut -d: -f1 || true)
  else
    GROUP_NAME=$(awk -F: -v gid="$USER_GID" '$3==gid {print $1}' /etc/group 2>/dev/null | head -n1)
  fi
  if [ -z "$GROUP_NAME" ]; then GROUP_NAME="$USER_NAME"; fi

  # Create group if missing
  if ! grep -q "^$GROUP_NAME:" /etc/group 2>/dev/null; then
    if command -v groupadd >/dev/null 2>&1; then
       groupadd -g "$USER_GID" "$GROUP_NAME" >/dev/null 2>&1 || true
    elif command -v addgroup >/dev/null 2>&1; then
       addgroup --gid "$USER_GID" "$GROUP_NAME" >/dev/null 2>&1 || true
    fi
  fi

  # Create user
  GROUP_ARG="-g $GROUP_NAME"
  if command -v useradd >/dev/null 2>&1; then
    useradd -m -d "$HOME_DIR" -s "$SHELL_PATH" -u "$USER_UID" $GROUP_ARG "$USER_NAME" >/dev/null 2>&1 || true
  elif command -v adduser >/dev/null 2>&1; then
     adduser --uid "$USER_UID" --gid "$USER_GID" --home "$HOME_DIR" --shell "$SHELL_PATH" --disabled-password --gecos "" "$USER_NAME" >/dev/null 2>&1 || true
  fi
fi

# Ensure the home directory exists before updating rc files.
mkdir -p "$HOME_DIR"
chown "$USER_UID":"$USER_GID" "$HOME_DIR" || true

# Sudo setup
if [ "$USER_NAME" != "root" ]; then
  if command -v sudo >/dev/null 2>&1; then
    SUDO_GROUP=""
    if grep -q "^sudo:" /etc/group 2>/dev/null; then
      SUDO_GROUP="sudo"
    elif grep -q "^wheel:" /etc/group 2>/dev/null; then
      SUDO_GROUP="wheel"
    fi
    
    if [ -n "$SUDO_GROUP" ]; then
        if ! grep -q "^$SUDO_GROUP:" /etc/group 2>/dev/null; then
             if command -v groupadd >/dev/null 2>&1; then groupadd "$SUDO_GROUP" >/dev/null 2>&1 || true; fi
             if command -v addgroup >/dev/null 2>&1; then addgroup "$SUDO_GROUP" >/dev/null 2>&1 || true; fi
        fi
        
        if command -v usermod >/dev/null 2>&1; then
             usermod -a -G "$SUDO_GROUP" "$USER_NAME" >/dev/null 2>&1 || true
        elif command -v addgroup >/dev/null 2>&1; then
             addgroup "$USER_NAME" "$SUDO_GROUP" >/dev/null 2>&1 || true
        fi
    fi

    mkdir -p /etc/sudoers.d
    echo "$USER_NAME ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/90-ned-user
    chmod 0440 /etc/sudoers.d/90-ned-user
  fi
fi

# Agent CLIs install into /usr/local/bin (ned-agent-install); keep it first on PATH.
echo "${NED_PREFIX} Configuring PATH for user: $USER_NAME"
for rcfile in "$HOME_DIR/.bashrc" "$HOME_DIR/.profile" "$HOME_DIR/.zshrc"; do
  if ! grep -q "ned-agent-path" "$rcfile" 2>/dev/null; then
    echo 'export PATH=/usr/local/bin:$PATH # ned-agent-path' >> "$rcfile"
    chown "$USER_UID":"$USER_GID" "$rcfile" || true
  fi
done

echo "${NED_PREFIX} Bootstrap complete: ok"
