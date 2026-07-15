// Command cl is a personal command-list manager: it persists a
// name -> shell command dictionary and lets you search it
// interactively. Adding, editing, renaming and deleting commands all
// happen inside the interactive picker itself
// (ctrl+a/ctrl+e/ctrl+r/ctrl+d) - see printUsage below. Whether
// picking a command shows its value and pre-fills it on the prompt,
// or runs it directly without ever showing it, is controlled by the
// picker's own ctrl+s toggle (store.Config.ShowCommand).
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

	"github.com/silvio/cl/internal/shellintegration"
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
		if len(args) < 2 {
			return fmt.Errorf("usage: cl init <%s>", strings.Join(shellintegration.Supported(), "|"))
		}
		return runInit(args[1])

	case "-h", "--help", "help":
		printUsage()
		return nil

	default:
		return runInteractive(strings.Join(args, " "))
	}
}

func runInit(shell string) error {
	script, err := shellintegration.Script(shell)
	if err != nil {
		return err
	}

	fmt.Print(script)
	return nil
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

	// cfg.ShowCommand (toggled in-picker with ctrl+s) decides what
	// to do with the picked command: shown, it's printed to stdout
	// for the shell integration to capture and pre-fill on the
	// prompt (a second Enter runs it); hidden, its value was never
	// shown, and cl runs it directly instead so it's never visible.
	if cfg.ShowCommand() {
		fmt.Println(entry.Command)
		return nil
	}

	return runDirectly(entry)
}

// runDirectly executes entry's command through the user's shell
// with its stdin/stdout/stderr connected straight to the
// controlling terminal - not to cl's own stdout, which the shell
// integration captures via command substitution - so its output
// reaches the console exactly as if it had been typed and run there
// directly. Since the command's value is never shown in this mode,
// it first announces the entry's name so there's still some
// feedback about what's about to run.
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
	fmt.Fprintln(colorprofile.NewWriter(w, nil), "> Execute "+nameStyle.Render(name))
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
	fmt.Println(`cl - a personal command-list manager

Usage:
  cl                  Open the interactive picker
  cl <filter>           Open the picker pre-filtered by <filter>
  cl init <shell>        Print shell integration snippet (zsh, bash, powershell)

Inside the picker:
  ctrl+a   add a new command (asks for a name, then the shell command)
  ctrl+e   edit the highlighted command
  ctrl+r   rename the highlighted command
  ctrl+d   delete the highlighted command
  ctrl+s   toggle showing each command next to its name in the list
  enter    if commands are shown: pick the highlighted command, to
           run on a second Enter; if hidden: run it directly
  esc      cancel`)
}
