// Command cl is a personal command-list manager: it persists a
// name -> shell command dictionary and lets you fuzzy-search it
// interactively.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/silvio/cl/internal/editor"
	"github.com/silvio/cl/internal/shellintegration"
	"github.com/silvio/cl/internal/store"
	"github.com/silvio/cl/internal/tui"
)

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
	case "-add":
		if len(args) < 2 {
			return fmt.Errorf("usage: cl -add <name>")
		}
		return runAdd(args[1])

	case "-remove":
		if len(args) < 2 {
			return fmt.Errorf("usage: cl -remove <name>")
		}
		return runRemove(args[1])

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

func runAdd(name string) error {
	s, err := store.Load()
	if err != nil {
		return err
	}

	existing, exists := s.Get(name)
	if exists {
		fmt.Printf("Command %q already exists: %s\n", name, existing)
		ok, err := confirm("Overwrite?")
		if err != nil {
			return err
		}
		if !ok {
			fmt.Println("Aborted.")
			return nil
		}
	}

	value, err := editor.EditValue(existing)
	if err != nil {
		return err
	}

	if value == "" {
		fmt.Println("Aborted: empty command.")
		return nil
	}

	s.Set(name, value)
	if err := s.Save(); err != nil {
		return err
	}

	fmt.Printf("Saved %q -> %s\n", name, value)
	return nil
}

func runRemove(name string) error {
	s, err := store.Load()
	if err != nil {
		return err
	}

	if !s.Remove(name) {
		return fmt.Errorf("command %q not found", name)
	}

	if err := s.Save(); err != nil {
		return err
	}

	fmt.Printf("Removed %q\n", name)
	return nil
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

	selected, err := tui.Run(filter, s.List())
	if err != nil {
		return err
	}

	if selected != "" {
		fmt.Println(selected)
	}

	return nil
}

func confirm(prompt string) (bool, error) {
	fmt.Printf("%s [y/N] ", prompt)

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return false, err
	}

	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func printUsage() {
	fmt.Println(`cl - a personal command-list manager

Usage:
  cl                    Open the interactive fuzzy picker
  cl <filter>            Open the picker pre-filtered by <filter>
  cl -add <name>          Add or edit the command stored as <name>
  cl -remove <name>       Remove the command stored as <name>
  cl init <shell>         Print shell integration snippet (zsh, bash, powershell)`)
}
