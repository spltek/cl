// Command cl is a personal command-list manager: it persists a
// name -> shell command dictionary and lets you fuzzy-search it
// interactively. Adding, editing, renaming and deleting commands all
// happen inside the interactive picker itself
// (ctrl+a/ctrl+e/ctrl+r/ctrl+d) - see printUsage below.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/silvio/cl/internal/shellintegration"
	"github.com/silvio/cl/internal/store"
	"github.com/silvio/cl/internal/tui"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
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

	selected, err := tui.Run(filter, s)
	if err != nil {
		return err
	}

	if selected != "" {
		fmt.Println(selected)
	}

	return nil
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
  enter    pick the highlighted command
  esc      cancel`)
}
