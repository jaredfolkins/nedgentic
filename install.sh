#!/bin/sh
set -eu

fail() {
    printf 'ned install: %s\n' "$*" >&2
    exit 1
}

find_install_dir() {
    remaining=${PATH:-}
    first_writable=
    while :; do
        directory=${remaining%%:*}
        directory=${directory:-.}
        target=$directory/ned

        case "$directory" in
            /*)
                if [ -d "$directory" ] && [ -w "$directory" ] && [ -x "$directory" ] && [ ! -L "$target" ]; then
                    if [ -f "$target" ] && [ -x "$target" ]; then
                        printf '%s\n' "$directory"
                        return
                    fi
                    if [ -z "$first_writable" ] && [ ! -e "$target" ]; then
                        first_writable=$directory
                    fi
                fi
                ;;
        esac

        # A later install would be hidden by this executable.
        if [ -f "$target" ] && [ -x "$target" ]; then
            break
        fi
        case "$remaining" in
            *:*) remaining=${remaining#*:} ;;
            *) break ;;
        esac
    done

    [ -n "$first_writable" ] || return 1
    printf '%s\n' "$first_writable"
}

main() {
    if [ "$(uname -s)" != Darwin ] || [ "$(sysctl -in hw.optional.arm64)" != 1 ]; then
        fail 'Ned requires an Apple silicon Mac.'
    fi
    if ! install_dir=$(find_install_dir); then
        printf '%s\n' 'No writable PATH directory can provide ned. Add one at the front of PATH and rerun:' \
            '  mkdir -p "$HOME/.local/bin"' \
            '  export PATH="$HOME/.local/bin:$PATH"' >&2
        exit 1
    fi

    repository=https://github.com/jaredfolkins/nedgentic
    release_page=$(curl -fsSLI --retry 2 -o /dev/null -w '%{url_effective}' "$repository/releases/latest")
    version=${release_page#"$repository/releases/tag/"}
    case "$version" in
        ''|*[!A-Za-z0-9._-]*) fail 'Could not determine the latest release tag.' ;;
    esac
    release=$repository/releases/download/$version
    asset=ned-darwin-arm64

    # Stage on the same filesystem so an interrupted install leaves the old binary intact.
    scratch=$(mktemp -d "$install_dir/.ned-install.XXXXXXXX")
    trap 'rm -f "$scratch/$asset" "$scratch/SHA256SUMS"; rmdir "$scratch"' EXIT
    trap 'exit 1' HUP INT TERM
    curl -fsSL --retry 2 -o "$scratch/$asset" "$release/$asset"
    curl -fsSL --retry 2 -o "$scratch/SHA256SUMS" "$release/SHA256SUMS"
    expected=$(awk -v asset="$asset" '$2 == asset { print $1 }' "$scratch/SHA256SUMS")
    actual=$(shasum -a 256 < "$scratch/$asset")
    [ "${actual%% *}" = "$expected" ] || fail "Checksum verification failed for $asset."

    target=$install_dir/ned
    if [ -L "$target" ] || { [ -e "$target" ] && [ ! -f "$target" ]; }; then
        fail "Cannot replace $target."
    fi
    chmod 0755 "$scratch/$asset"
    mv -f "$scratch/$asset" "$target"
    printf 'Installed ned %s to %s\n' "$version" "$target"
}

main
