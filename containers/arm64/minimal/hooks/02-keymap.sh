#!/bin/sh
set -e

SHIFT_ENTER_SEQ='\e[27;2;13~'
INPUTRC_PATH="/etc/inputrc"
INPUTRC_MARKER="# Nedgentic keybindings (shift+enter)"
INPUTRC_BINDING="\"${SHIFT_ENTER_SEQ}\": accept-line"

if [ -w /etc ]; then
  if [ -f "$INPUTRC_PATH" ]; then
    if ! grep -Fq "$INPUTRC_MARKER" "$INPUTRC_PATH" 2>/dev/null; then
      {
        printf '\n%s\n' "$INPUTRC_MARKER"
        printf '%s\n' "$INPUTRC_BINDING"
      } >> "$INPUTRC_PATH" 2>/dev/null || true
      chmod 0644 "$INPUTRC_PATH" 2>/dev/null || true
    fi
  else
    {
      printf '%s\n' "$INPUTRC_MARKER"
      printf '%s\n' "$INPUTRC_BINDING"
    } > "$INPUTRC_PATH" 2>/dev/null || true
    chmod 0644 "$INPUTRC_PATH" 2>/dev/null || true
  fi
fi

if command -v zsh >/dev/null 2>&1; then
  ZSH_DIR="/etc/zsh"
  ZSH_BIND_PATH="$ZSH_DIR/ned-keymap.zsh"
  ZSHRC_PATH="$ZSH_DIR/zshrc"
  if [ -d "$ZSH_DIR" ] || mkdir -p "$ZSH_DIR" 2>/dev/null; then
    if [ ! -f "$ZSH_BIND_PATH" ] || ! grep -Fq "Nedgentic keybindings" "$ZSH_BIND_PATH" 2>/dev/null; then
      cat <<'EOF' > "$ZSH_BIND_PATH"
# Nedgentic keybindings (shift+enter)
bindkey -s '\e[27;2;13~' '^M'
EOF
      chmod 0644 "$ZSH_BIND_PATH" 2>/dev/null || true
    fi
    if [ ! -f "$ZSHRC_PATH" ] || ! grep -Fq "ned-keymap.zsh" "$ZSHRC_PATH" 2>/dev/null; then
      {
        printf '\n# Nedgentic keybindings\n'
        printf '. /etc/zsh/ned-keymap.zsh\n'
      } >> "$ZSHRC_PATH" 2>/dev/null || true
      chmod 0644 "$ZSHRC_PATH" 2>/dev/null || true
    fi
  fi
fi
