package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.remotex.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.DaemonPath}}</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>{{.LogDir}}/daemon.log</string>
    <key>StandardErrorPath</key>
    <string>{{.LogDir}}/daemon.err</string>
</dict>
</plist>
`

// InstallLaunchd writes the launchd plist for remotex-daemon and loads it.
func InstallLaunchd(daemonBinaryPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	logDir := filepath.Join(home, ".remotex", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	plistDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(plistDir, 0755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	plistPath := filepath.Join(plistDir, "com.remotex.daemon.plist")
	f, err := os.Create(plistPath)
	if err != nil {
		return fmt.Errorf("create plist: %w", err)
	}
	defer f.Close()

	t := template.Must(template.New("plist").Parse(plistTemplate))
	if err := t.Execute(f, map[string]string{
		"DaemonPath": daemonBinaryPath,
		"LogDir":     logDir,
	}); err != nil {
		return fmt.Errorf("render plist: %w", err)
	}

	if err := exec.Command("launchctl", "load", plistPath).Run(); err != nil {
		return fmt.Errorf("launchctl load: %w", err)
	}
	return nil
}

// UninstallLaunchd unloads and removes the launchd plist.
func UninstallLaunchd() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.remotex.daemon.plist")
	exec.Command("launchctl", "unload", plistPath).Run() // best-effort
	return os.Remove(plistPath)
}
