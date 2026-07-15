// Package editor resolves which text editor to use and opens it on a
// temporary file so the user can write/edit a shell command without
// worrying about the calling shell's own quoting rules.
package editor

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/silvio/cl/internal/config"
)

func defaultEditor() string {
	if runtime.GOOS == "windows" {
		return "notepad"
	}
	return "vi"
}

// Resolve returns the editor command to use, in order of priority:
//  1. the $EDITOR environment variable, if set;
//  2. the preference previously saved by cl itself;
//  3. an interactive prompt, whose answer is then persisted for next time.
func Resolve() (string, error) {
	if e := strings.TrimSpace(os.Getenv("EDITOR")); e != "" {
		return e, nil
	}

	cfg, err := config.Load()
	if err != nil {
		return "", err
	}

	if cfg.Editor != "" {
		return cfg.Editor, nil
	}

	editor, err := promptForEditor()
	if err != nil {
		return "", err
	}

	cfg.Editor = editor
	if err := cfg.Save(); err != nil {
		return "", fmt.Errorf("save editor preference: %w", err)
	}

	return editor, nil
}

func promptForEditor() (string, error) {
	def := defaultEditor()
	fmt.Printf("No editor configured. Which command should be used to edit commands? [%s] ", def)

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("read editor choice: %w", err)
	}

	answer := strings.TrimSpace(line)
	if answer == "" {
		return def, nil
	}

	return answer, nil
}

// EditValue opens the resolved editor on a temp file pre-filled with
// initial, and returns the trimmed content after the editor exits.
func EditValue(initial string) (string, error) {
	editorCmd, err := Resolve()
	if err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp("", "cl-edit-*.sh")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.WriteString(initial); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}

	fields := strings.Fields(editorCmd)
	if len(fields) == 0 {
		return "", fmt.Errorf("empty editor command")
	}
	args := append(append([]string{}, fields[1:]...), tmpPath)

	cmd := exec.Command(fields[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("run editor %q: %w", editorCmd, err)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("read edited file: %w", err)
	}

	return strings.TrimSpace(string(data)), nil
}
