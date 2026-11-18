# AGENTS.md

## What Ned Is
`ned` is a macOS Go CLI that wraps Apple's `container` runtime. One command
(`ned gentic personal <profile> [vm]`) starts a sandboxed VM from the minimal
image (`containers/*/minimal`) with Claude, Codex, Gemini, Grok, and opencode,
mounts the current folder read-write, and attaches a shell. Agent CLIs update on
boot; each VM signs in with `ned-agent-login` and keeps its logins on its own
`ned-auth-<vm>` volume.

## Layout
- `cmd/ned`: entry point.
- `internal/frontend`: Cobra commands and user-facing output.
- `internal/runtime`: container lifecycle, mounts, staging, profiles, auth volume.
- `internal/profile`: profile schema and templates.
- `internal/templates`: guest shell scripts (runner, attach, `ned-agent-login`).
- `containers/`: image Dockerfiles and Makefiles.

## Rules
- Keep it simple. Remove code rather than add options; no legacy or
  compatibility shims (beta).
- Target current Apple `container` behavior only.
- Never copy host agent logins into a VM; tokens rotate and copies break the
  host login.
- Mount paths: reject `..` and commas; directory mounts stay under the host home.
- Secret staging: copy into a 0700 directory, files 0600/0400, mount read-only.
- Never change ownership of bind-mounted host directories.
- Do not log secrets or file contents.
- Keep the README short and current when behavior changes.
- No emojis. Ask before destructive commands, new command aliases, or pushes.

## Test
```bash
gofmt -l internal cmd e2e   # must print nothing
go vet ./...
go test ./internal/...
go test ./e2e -run 'TestE2EAlpineDefault$|TestE2EMinimalImage$' -count=1   # needs the runtime
go build -o bin/ned ./cmd/ned
```
E2E tests use explicit timeouts and remove every VM, volume, and image they create.
