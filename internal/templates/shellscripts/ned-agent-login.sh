#!/bin/bash
# ned-agent-login: sign this VM in to each agent CLI. Enter installs the CLI
# if needed (ned-agent-install) and runs its sign-in. Logins are stored on the
# VM's own auth volume (/mnt/ned-auth), so every VM keeps its own accounts.
#
#   ned-agent-login           walk every provider
#   ned-agent-login --attach  walk only signed-out providers not skipped before
set -u

INSTALLER=/usr/local/bin/ned-agent-install

AUTH_DIR=/mnt/ned-auth
SKIP_FILE="$AUTH_DIR/.ned-login-skipped"
MODE=all
case "${1:-}" in
  --attach) MODE=attach ;;
  "") ;;
  -h|--help)
    sed -n '2,8p' "$0" | sed 's/^# \{0,1\}//'
    exit 0
    ;;
  *) echo "usage: ned-agent-login [--attach]" >&2; exit 2 ;;
esac

[ -t 0 ] && [ -t 1 ] || exit 0

# Ned's glasses, as on the host.
NED_TAG=$'\e[38;5;180m[\e[38;5;15m\xe2\x8c\x90\e[38;5;251m\xe2\x97\xaf\e[38;5;15m-\e[38;5;251m\xe2\x97\xaf\e[38;5;180m]\e[0m'
say() { printf '%s %s\n' "$NED_TAG" "$*"; }

# id|label|credential file|sign-in command|hint
# Credential files match ned-agent-install --signed-in.
PROVIDERS=(
  "claude|Claude|$HOME/.claude/.credentials.json|claude auth login|Open the link, click Authorize, then paste the code here."
  "codex|Codex|$HOME/.codex/auth.json|codex login --device-auth|Open the link and enter the code. Device code sign-in must be on in ChatGPT security settings."
  "grok|Grok|$HOME/.grok/auth.json|grok login --device-auth|Open the link and enter the code."
  "gemini|Gemini|$HOME/.gemini/oauth_creds.json|env NO_BROWSER=true gemini|Choose Sign in with Google, open the link, paste the code, then type /quit."
  "opencode|opencode|$HOME/.local/share/opencode/auth.json|opencode auth login|Pick a provider and follow its prompts."
)

is_skipped() {
  [ -f "$SKIP_FILE" ] && grep -qx "$1" "$SKIP_FILE"
}

remember_skip() {
  if [ -d "$AUTH_DIR" ] && ! is_skipped "$1"; then
    printf '%s\n' "$1" >>"$SKIP_FILE"
  fi
}

forget_skip() {
  if [ -f "$SKIP_FILE" ]; then
    grep -vx "$1" "$SKIP_FILE" >"$SKIP_FILE.tmp" || true
    mv "$SKIP_FILE.tmp" "$SKIP_FILE"
  fi
}

# Prints enter, skip, or quit.
read_key() {
  local key rest
  while true; do
    IFS= read -rsn1 key || { echo quit; return; }
    case "$key" in
      "") echo enter; return ;;
      $'\e')
        rest=""
        IFS= read -rsn2 -t 0.1 rest || true
        case "$rest" in
          "[B"|"[C") echo skip; return ;;
          "") echo quit; return ;;
        esac
        ;;
      s|S|n|N) echo skip; return ;;
      q|Q) echo quit; return ;;
    esac
  done
}

pending=()
for entry in "${PROVIDERS[@]}"; do
  IFS='|' read -r id label cred cmd hint <<<"$entry"
  # Without the installer (other images), only CLIs already present.
  command -v "$id" >/dev/null 2>&1 || [ -x "$INSTALLER" ] || continue
  if [ "$MODE" = attach ]; then
    [ -s "$cred" ] && continue
    is_skipped "$id" && continue
  fi
  pending+=("$entry")
done

[ "${#pending[@]}" -gt 0 ] || exit 0

echo
say "Sign in to agent CLIs for this VM. Enter installs the CLI if needed and signs in, Down skips, q quits."
for entry in "${pending[@]}"; do
  IFS='|' read -r id label cred cmd hint <<<"$entry"
  status="signed out"
  [ -s "$cred" ] && status="signed in"
  action="sign in"
  if ! command -v "$id" >/dev/null 2>&1; then
    status="$status, not installed"
    action="install + sign in"
  fi
  printf '\n%s %-9s (%s)  Enter %s · Down skip · q quit ' "$NED_TAG" "$label" "$status" "$action"
  choice=$(read_key)
  echo
  case "$choice" in
    quit) say "Stopped. Run ned-agent-login any time to continue."; break ;;
    skip)
      say "Skipped $label."
      [ "$MODE" = attach ] && remember_skip "$id"
      continue
      ;;
  esac
  if ! command -v "$id" >/dev/null 2>&1; then
    if ! "$INSTALLER" "$id" || ! command -v "$id" >/dev/null 2>&1; then
      say "$label did not install. Run ned-agent-login to try again."
      continue
    fi
  fi
  say "$hint"
  if $cmd && [ -s "$cred" ]; then
    say "Signed in: $label"
    forget_skip "$id"
  else
    say "$label sign-in did not finish. Run ned-agent-login to try again."
  fi
done

echo
say "Logins are stored on this VM's disk and kept until ned remove. Run ned-agent-login to change them."
exit 0
