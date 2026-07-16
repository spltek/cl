// Command cl is a personal command launcher: it persists a
// name -> shell command dictionary and lets you search it
// interactively. Adding, editing, renaming and deleting commands all
// happen inside the interactive picker itself
// (ctrl+a/ctrl+e/ctrl+r/ctrl+d/ctrl+l) - see printUsage below. Enter
// always runs the picked command directly (after prompting for any
// {{placeholders}}). Whether the list shows each entry's command
// under its name is controlled by the picker's own ctrl+s toggle
// (store.Config.ShowCommand).
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/silvio/cl/internal/store"
	"github.com/silvio/cl/internal/tui"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		// A command run directly (see runInteractive) failing on its
		// own terms - e.g. `grep` finding nothing - isn't a cl error:
		// propagate its exit code silently instead of wrapping it in
		// a misleading "cl: error: exit status 1".
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}

		fmt.Fprintln(os.Stderr, "cl: error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return runInteractive("")
	}

	switch args[0] {
	case "-v", "--version":
		fmt.Println("cl", version)
		return nil

	case "init":
		// Shell integration is no longer needed (Enter always runs
		// commands directly regardless of Ctrl+S). Kept as a silent
		// no-op so existing eval "$(cl init ...)" lines in shell rc
		// files don't cause errors or open the picker.
		return nil

	case "-h", "--help", "help":
		printUsage()
		return nil

	default:
		return runInteractive(strings.Join(args, " "))
	}
}

func runInteractive(filter string) error {
	s, err := store.Load()
	if err != nil {
		return err
	}

	cfg, err := store.LoadConfig()
	if err != nil {
		return err
	}

	entry, err := tui.Run(filter, s, cfg)
	if err != nil {
		return err
	}

	if entry.Command == "" {
		return nil
	}

	// ShowCommand (toggled in-picker with ctrl+s) only controls
	// whether the command is displayed in the list. Enter always
	// runs the picked command directly, with its output reaching
	// the console exactly as if it had been typed there.
	return runDirectly(entry)
}

// runDirectly executes entry's command through the user's shell
// with its stdin/stdout/stderr connected straight to the
// controlling terminal so its output reaches the console exactly as
// if it had been typed and run there directly. Before running it,
// it announces the entry's name so there's still some feedback
// about what's about to run.
func runDirectly(entry store.Entry) error {
	ttyIn, ttyOut, err := tea.OpenTTY()
	if err != nil {
		return fmt.Errorf("open controlling terminal: %w", err)
	}
	defer ttyIn.Close()
	defer ttyOut.Close()

	printExecuting(ttyOut, entry.Name)

	shellPath, shellArgs := shellCommand(entry.Command)
	cmd := exec.Command(shellPath, shellArgs...)
	cmd.Stdin = ttyIn
	cmd.Stdout = ttyOut
	cmd.Stderr = ttyOut

	return cmd.Run()
}

// printExecuting announces, on the controlling terminal, which
// command is about to run, with its name picked out in color. w is
// wrapped in a colorprofile.Writer so the color is automatically
// downgraded (or dropped, honoring NO_COLOR) to whatever w actually
// supports, the same way Bubble Tea itself does for the picker.
func printExecuting(w io.Writer, name string) {
	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	fmt.Fprintln(colorprofile.NewWriter(w, nil), "> Execute: "+nameStyle.Render(name))
}

// shellCommand returns the shell binary and arguments used to run
// an arbitrary shell command string, honoring the user's configured
// shell (or Windows' ComSpec) so it behaves the same as if typed
// directly at their prompt.
func shellCommand(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		comspec := os.Getenv("COMSPEC")
		if comspec == "" {
			comspec = "cmd.exe"
		}
		return comspec, []string{"/C", command}
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return shell, []string{"-c", command}
}

func printUsage() {
	fmt.Println(`cl - a personal command launcher

Usage:
  cl                  Open the interactive picker
  cl <filter>           Open the picker pre-filtered by <filter>

Inside the picker:
  ctrl+a   add a new command (asks for a name, then the shell command)
  ctrl+e   edit the highlighted command
  ctrl+r   rename the highlighted command
  ctrl+d   delete the highlighted command
  ctrl+s   toggle showing each command under its name in the list
  ctrl+l   set how many list entries are visible (default 20)
  enter    run the highlighted command (prompts for {{placeholders}} first)
  esc      cancel`)
}
