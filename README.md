# cl — command list

A tiny personal command manager. `cl` stores a `name -> shell command`
dictionary in a JSON file and lets you search it interactively from
your terminal. By default the command itself stays hidden — only
names are shown — and picking one runs it immediately, without ever
displaying its value. Toggle `Ctrl+S` in the picker to flip that: the
command then shows next to its name, and picking it writes it onto
your shell prompt instead (not run yet), so a second Enter executes
it exactly as if you had typed it yourself. Either way, all output
from a run command reaches your console normally.

## How it works

Everything happens inside a single interactive picker — there's no
separate "add"/"remove" subcommand to remember:

- `cl` opens the picker; `cl <filter>` opens it pre-filtered.
- Type to filter by name (case-insensitive substring match — the
  whole typed text has to appear together, not just its letters
  scattered anywhere in the name), use the arrow keys to move,
  `Enter` to pick, `Esc`/`Ctrl-C` to cancel.
- `Ctrl+S` toggles whether the list shows each command next to its
  name (persisted immediately, so it's remembered next time).
  **Enter always runs the command directly** — the toggle only
  affects what you see in the list, never what Enter does.
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
- `Ctrl+R` renames the highlighted command: its current name appears
  pre-filled and editable (spaces are allowed); `Enter` asks
  "rename?" (`y` saves, `Esc`/`n` discards the rename and leaves the
  stored name untouched). A name that's already used by another
  command is rejected in place with an inline message.
- `Ctrl+D` deletes the highlighted command, after a `y`/`N`
  confirmation.
- A command can contain **placeholders** with the `{{name}}` or
  `{{name:default}}` syntax. When you pick such a command, `cl`
  prompts you to fill each placeholder before running it — see
  [Placeholders](#placeholders) below.
- Commands are persisted as JSON in your user config directory
  (`~/Library/Application Support/cl` on macOS, `~/.config/cl` on
  Linux, `%AppData%\cl` on Windows) — written to disk immediately as
  each add/edit/remove is confirmed, not just when quit.

## Install

### Homebrew (macOS/Linux)

```bash
brew install silviopola/tap/cl
```

### From a release (macOS/Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/silviopola/cl/main/install.sh | sh
```

Downloads the right binary for your OS/arch from the
[latest release](https://github.com/silviopola/cl/releases) into
`~/.local/bin` (override with `CL_INSTALL_DIR`).

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
make install-local
```

Builds the binary and installs it to `~/.local/bin/cl`.
At the end it prints the one command you need to
start using `cl` in the current shell without restarting it.

> If you prefer to manage the binary yourself, `make build` compiles
> into `bin/cl` and `make install` puts it into `$GOPATH/bin`.

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

## Placeholders

Commands can contain `{{name}}` or `{{name:default}}` placeholders.
When you pick such a command, `cl` prompts you to fill each
placeholder in sequence before running the resolved command.

### Syntax

```
{{name}}              required placeholder — must be filled in
{{name:default}}      optional placeholder — pre-filled with default,
                      press Enter to accept it as-is
```

Placeholder names can contain letters, digits and underscores
(`\w+`). The default value can be any text that does not contain
`}}`.

### Example

Add a command with a placeholder:

```
ctrl+a → name: "ssh server" → command: ssh {{user}}@{{host}}
```

Now pick it:

```
$ cl ssh
cl> ssh
> ssh server

↑/↓ move  enter run selected  ...

# Press Enter — cl detects the {{placeholders}} and prompts:

ssh {{user}}@{{host}}

user:
_
enter continue · esc cancel

# Type a value, press Enter:

ssh admin@{{host}}

host:
_
enter run · esc cancel

# Type the host and press Enter — the resolved command runs:

> Execute ssh server
Last login: ...
```

With defaults:

```
ctrl+a → name: "git push" → command: git push {{remote:origin}} {{branch:main}}
```

Picking it pre-fills "origin" for `remote` and "main" for `branch`
— press Enter through both to accept the defaults, or type over them.

### In practice

```
# Database connections
psql -h {{host:localhost}} -p {{port:5432}} -U {{user}} -d {{db:postgres}}

# SSH with numbered hosts
ssh {{user}}@prod-{{num:1}}.example.com

# Docker with configurable ports
ocker run -p {{port:3000}}:3000 {{image:node:18}}

# Kubernetes with namespace
kubectl logs {{pod}} -n {{namespace:default}}
```

Placeholders work regardless of whether commands are shown or hidden
in the list. `cl` always resolves them and runs the command directly.

## Example

```
$ cl
cl>
  no matching commands

↑/↓ move
enter run selected
ctrl+a add new command
ctrl+s command show toggle
esc cancel

# press ctrl+a, type a name, enter, type the command, enter:

Add command "build" - shell command:
npm run build -- --watch
enter save · esc cancel

# back at the list, command hidden (the default) - Enter runs it
# directly, with its own output printing normally, and you never
# see "npm run build -- --watch" itself:

$ cl bui
cl> bui
> build

↑/↓ move
enter run selected
ctrl+a add new command
ctrl+e edit selected
ctrl+r rename selected
ctrl+d delete selected
ctrl+s command show toggle
esc cancel

# press Enter on "build": cl announces the name (colored) before
# running it, then the command's own output follows normally

> Execute build
...(build output)...

# press ctrl+s to show commands in the list (display-only —
# Enter always runs the command directly):

$ cl bui
cl> bui
> build
  npm run build -- --watch

↑/↓ move
enter run selected
ctrl+a add new command
ctrl+e edit selected
ctrl+r rename selected
ctrl+d delete selected
ctrl+s command show toggle
esc cancel
# The command is visible below its name so you know what you're
# about to run. Enter always executes it directly.

# Long commands wrap:

$ cl
cl> deploy
> deploy
  kubectl apply -f production/overlays/us-east-1/kustomization.yaml
  --prune --selector app=api

↑/↓ move  enter run selected  ...
esc cancel

# press ctrl+e on "build" to edit it in place:

Edit "build":
npm run build -- --watch --fast
enter continue · esc cancel
# Enter then asks to confirm:
Save "build" -> npm run build -- --watch --fast ? [y/N]
y confirm · n/esc cancel

# press ctrl+r on "build" to rename it:

Rename "build":
release
enter continue · esc cancel
# Enter then asks to confirm:
Rename "build" -> "release" ? [y/N]
y confirm · n/esc cancel

# press ctrl+d on "release" to delete it:

Delete "release" (npm run build -- --watch --fast) ? [y/N]
y confirm · n/esc cancel
```

## License

[MIT](LICENSE)
