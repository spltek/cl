// Package shellintegration provides the shell-specific snippets that
// wire the `cl` binary into an interactive shell session, so a
// selected command can be written back into the editing buffer.
package shellintegration

import (
	"embed"
	"fmt"
)

//go:embed templates/*
var templates embed.FS

// Supported returns the list of shell names cl init accepts.
func Supported() []string {
	return []string{"zsh", "bash", "powershell"}
}

// Script returns the integration snippet for the given shell name.
func Script(shell string) (string, error) {
	var file string
	switch shell {
	case "zsh":
		file = "templates/zsh.sh"
	case "bash":
		file = "templates/bash.sh"
	case "powershell", "pwsh":
		file = "templates/powershell.ps1"
	default:
		return "", fmt.Errorf("unsupported shell %q (supported: %v)", shell, Supported())
	}

	data, err := templates.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("read template for %q: %w", shell, err)
	}

	return string(data), nil
}
