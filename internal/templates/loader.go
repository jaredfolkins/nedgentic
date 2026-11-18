package templates

import (
	"embed"
)

//go:embed shellconfig/* shellscripts/runner.sh.tmpl shellscripts/systemd_bootstrap.sh.tmpl shellscripts/shell_fallback.sh.tmpl shellscripts/agent_auth.sh.tmpl shellscripts/ned-agent-login.sh
var Assets embed.FS
