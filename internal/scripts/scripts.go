package scripts

import (
	"bytes"
	"text/template"

	"nedgentic/internal/templates"
)

type RunnerData struct {
	UserName       string
	UserUID        int
	UserGID        int
	HomeDir        string
	ShellPath      string
	CopyBlock      string
	ShellCommand   string
	WorkspacePath  string
	WorkspaceMount string
	AgentAuthBlock string
}

// AgentAuthData renders the runner block that links agent CLI config into the
// per-VM auth volume and installs ned-agent-login.
type AgentAuthData struct {
	AuthDir     string
	LoginScript string
}

type ShellData struct {
	ShellPath string
	Workdir   string
}

var runnerTpl *template.Template
var bootstrapTpl *template.Template
var shellTpl *template.Template
var agentAuthTpl *template.Template
var agentLoginScript string

func init() {
	runnerContent, err := templates.Assets.ReadFile("shellscripts/runner.sh.tmpl")
	if err != nil {
		panic(err)
	}
	runnerTpl = template.Must(template.New("runner").Parse(string(runnerContent)))

	bootstrapContent, err := templates.Assets.ReadFile("shellscripts/systemd_bootstrap.sh.tmpl")
	if err != nil {
		panic(err)
	}
	bootstrapTpl = template.Must(template.New("bootstrap").Parse(string(bootstrapContent)))

	shellContent, err := templates.Assets.ReadFile("shellscripts/shell_fallback.sh.tmpl")
	if err != nil {
		panic(err)
	}
	shellTpl = template.Must(template.New("shell").Parse(string(shellContent)))

	agentAuthContent, err := templates.Assets.ReadFile("shellscripts/agent_auth.sh.tmpl")
	if err != nil {
		panic(err)
	}
	agentAuthTpl = template.Must(template.New("agent-auth").Parse(string(agentAuthContent)))

	loginContent, err := templates.Assets.ReadFile("shellscripts/ned-agent-login.sh")
	if err != nil {
		panic(err)
	}
	agentLoginScript = string(loginContent)
}

// AgentLoginScript returns the guest ned-agent-login walkthrough script.
func AgentLoginScript() string {
	return agentLoginScript
}

func RenderAgentAuth(authDir string) (string, error) {
	var buf bytes.Buffer
	if err := agentAuthTpl.Execute(&buf, AgentAuthData{AuthDir: authDir, LoginScript: agentLoginScript}); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func RenderRunner(data RunnerData) (string, error) {
	var buf bytes.Buffer
	if err := runnerTpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func RenderBootstrap(data RunnerData) (string, error) {
	var buf bytes.Buffer
	if err := bootstrapTpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func RenderShell(data ShellData) (string, error) {
	var buf bytes.Buffer
	if err := shellTpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
