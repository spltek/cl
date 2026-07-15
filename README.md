# cl — command list

A tiny personal command manager. `cl` stores a `name -> shell command`
dictionary in a JSON file and lets you fuzzy-search it interactively
from your terminal. Selecting a command writes it onto your shell
prompt (not run yet) so a second Enter executes it exactly as if you
had typed it yourself.

## How it works

Everything happens inside a single interactive picker — there's no
separate "add"/"remove" subcommand to remember:

- `cl` opens the picker; `cl <filter>` opens it pre-filtered.
- Type to fuzzy-filter by name, use the arrow keys to move, `Enter`
  to pick, `Esc`/`Ctrl-C` to cancel.
- `Ctrl+A` adds a new command: it first asks for a name (spaces are
  allowed — there's no CLI token boundary to worry about anymore),
  then, on `Enter`, asks for the shell command itself and saves it
  immediately on the next `Enter`. There's no editor involved at any
  point. A name that's already in use, or a name/command that's
  empty (after trimming whitespace), is rejected in place with an
  inline message so you can just try again.
- `Ctrl+E` edits the highlighted command: its current value appears
  pre-filled and editable; `Enter` asks "save?" (`y` saves, `Esc`/`n`
  discards the edit and leaves the stored command untouched).
- `Ctrl+R` removes the highlighted command, after a `y`/`N`
  confirmation.
- Commands are persisted as JSON in your user config directory
  (`~/Library/Application Support/cl` on macOS, `~/.config/cl` on
  Linux, `%AppData%\cl` on Windows) — written to disk immediately as
  each add/edit/remove is confirmed, not just when you quit.

Picking a command from a plain binary invocation can't, by itself,
write into your shell's input line — a child process has no way to
reach into its parent shell's editing buffer. That's why `cl` also
ships shell integration: a small `cl` **function** that shadows the
`cl` binary on your `PATH`, calls the real binary, and then hands the
result back to the shell using whatever native mechanism is
available (see below). This is the same pattern tools like `zoxide`
or `navi` use.

## Install

### From a release (macOS/Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/silviopola/cl/main/install.sh | sh
```

Downloads the right binary for your OS/arch from the
[latest release](https://github.com/silviopola/cl/releases) into
`~/.local/bin` (override with `CL_INSTALL_DIR`), and **automatically
wires up shell integration** for every shell it finds on the system
(`~/.zshrc`, `~/.bashrc`, and `~/.bash_profile` if present) — open a
new terminal afterwards and `cl` works immediately, no manual editing
needed. Re-running the installer is safe; it never adds the same line
twice.

### From a release (Windows)

```powershell
iwr https://raw.githubusercontent.com/silviopola/cl/main/install.ps1 | iex
```

Installs into `%LOCALAPPDATA%\cl\bin` (override with `CL_INSTALL_DIR`),
persists it on your **User**-scope `PATH`, and adds the integration
line to your PowerShell profile (`$PROFILE`) automatically.

#### Special permissions?

None of the above needs admin/elevated rights: installing into
`~/.local/bin` / `%LOCALAPPDATA%`, editing your own shell rc files,
and setting the **User**-scope `PATH` (as opposed to Machine-scope)
are all plain per-user operations. The installer also clears the
macOS quarantine flag defensively (harmless if absent, no `sudo`
needed since it's your own file).

The one real gotcha is Windows-specific: if your PowerShell execution
policy resolves to `Restricted` or `AllSigned`, your `$PROFILE` script
won't run at all (this predates and is unrelated to `cl`), so the
added integration line would silently never execute. The installer
detects this and prints a warning with the exact command to fix it
(`Set-ExecutionPolicy -Scope CurrentUser RemoteSigned`) — it
deliberately does not change this policy itself, since it's a
security setting you should opt into consciously.

### From source

Requires Go 1.21+:

```bash
make build   # -> bin/cl
```

Put the resulting `bin/cl` binary somewhere on your `PATH` (e.g.
`~/.local/bin` or `/usr/local/bin`), or run `make install` to install
it into `$GOPATH/bin`/`$GOBIN` via `go install`.

### Releasing (maintainers)

Releases are built and published automatically by
[`.github/workflows/release.yml`](.github/workflows/release.yml) via
[GoReleaser](https://goreleaser.com) whenever a `vX.Y.Z` tag is pushed:

```bash
git tag v0.1.0
git push origin v0.1.0
```

`.github/workflows/ci.yml` runs build/vet/test/gofmt on every push and
pull request (Linux + Windows; macOS is covered locally by the
maintainer to save Actions minutes on macOS runners).

## Development

```bash
make build        # compile into bin/cl
make test         # run the test suite
make test-verbose # run the test suite with -v
make cover        # run tests with coverage report
make vet          # go vet
make fmt          # gofmt -w .
make fmt-check    # fail if any file is not gofmt-formatted
make run ARGS="foo"  # build then run with the given args
make clean        # remove bin/ and dist/
```

## Shell integration

### Zsh

Add to `~/.zshrc`:

```zsh
eval "$(cl init zsh)"
```

Uses `print -z` to push the picked command into the *next* prompt's
editing buffer — the exact two-step "write, then Enter to run" flow,
no compromises.

### Bash

Add to `~/.bashrc`:

```bash
eval "$(cl init bash)"
```

Bash has no exact equivalent of `print -z` for a plain typed command
(the `READLINE_LINE` trick only works inside an active key binding),
so the integration shows the picked command as an editable pre-filled
line via `read -e -i` right after you pick it — functionally the same
experience, implemented differently under the hood.

### PowerShell

Add to your profile (`$PROFILE`):

```powershell
Invoke-Expression (cl init powershell | Out-String)
```

Tries `[Microsoft.PowerShell.PSConsoleReadLine]::Insert()` (the same
mechanism used by modules like PSFzf); if that's unavailable it falls
back to an explicit `Run: <command>? [Y/n]` confirmation.

## Example

```
$ cl
cl>
  no matching commands
↑/↓ move
enter select
esc cancel
ctrl+a add

# press ctrl+a, type a name, enter, type the command, enter:

Add command "build" - shell command:
npm run build -- --watch
enter save · esc cancel

# back at the list:

$ cl bui
cl> bui
> build  npm run build -- --watch
↑/↓ move
enter select
esc cancel
ctrl+a add
ctrl+e edit
ctrl+r remove
# Enter picks it, it appears on your prompt, Enter again runs it

# press ctrl+e on "build" to edit it in place:

Edit "build":
npm run build -- --watch --fast
enter continue · esc cancel
# Enter then asks to confirm:
Save "build" -> npm run build -- --watch --fast ? [y/N]
y confirm · n/esc cancel

# press ctrl+r on "build" to remove it:

Remove "build" (npm run build -- --watch --fast) ? [y/N]
y confirm · n/esc cancel
```
