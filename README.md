# cl — command launcher

<img src="media/cl.png" alt="cl screenshot" width="100%" />

> *I've watched the up arrow fire in the dark, hunting through history for a command lost three weeks and a thousand keystrokes ago.  
> All those commands, scattered across gists, notes, and files no one will ever open again.  
> All those moments will be lost in time… like uncommitted changes before a force push.  
> Time to launch.*

A tiny personal command launcher. `cl` stores a `name -> shell command`
dictionary in a JSON file and lets you search it interactively from
your terminal. By default the command itself stays hidden — only
names are shown — so you can run it without ever seeing its value.
Toggle `Ctrl+S` in the picker to also show each command under its
name in the list (display-only — `Enter` always runs the command
directly). All output from a run command reaches your console
normally.

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
  [Placeholders](#placeholders) below. In the list, commands with
  placeholders show a hint next to the name, e.g.
  `ssh server (user, host)` or
  `git push (remote[default:origin], branch[default:main])`.
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

Installs into `%LOCALAPPDATA%\cl\bin` (override with `CL_INSTALL_DIR`).
Make sure this directory is on your `PATH` after installing.

#### Special permissions?

None of the above needs admin/elevated rights: installing into
`~/.local/bin` / `%LOCALAPPDATA%`, editing your own shell rc files,
and setting the **User**-scope `PATH` (as opposed to Machine-scope)
are all plain per-user operations. The installer also clears the
macOS quarantine flag defensively (harmless if absent, no `sudo`
needed since it's your own file).

If you are on Windows and your PowerShell execution policy resolves
to `Restricted` or `AllSigned`, you may need to allow scripts to run
with `Set-ExecutionPolicy -Scope CurrentUser RemoteSigned` before
the install command above can execute.

### From source

Requires Go 1.25+:

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
pull request (Linux, Windows, and macOS).

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

In the list, placeholders appear as a hint after the command name:

```
ssh server (user, host)
git push (remote[default:origin], branch[default:main])
echo hi (name[default:pippo])
```

Required parameters are listed as bare names; parameters with a
default use `name[default:value]`.

### Example

Add a command with a placeholder:

```
ctrl+a → name: "ssh server" → command: ssh {{user}}@{{host}}
```

Now pick it:

```
$ cl ssh
cl> ssh
> ssh server (user, host)

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

> Execute: ssh server
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
docker run -p {{port:3000}}:3000 {{image:node:18}}

# Kubernetes with namespace
kubectl logs {{pod}} -n {{namespace:default}}
```

Placeholders work regardless of whether commands are shown or hidden
in the list. `cl` always resolves them and runs the command directly.

### Shell builtins that need `&& exec $SHELL`

Some commands, like shell builtins, modify the current shell
environment and need a replacement shell after running to persist
their effects. Without `&& exec $SHELL` the command runs in a
subshell and the changes are lost when it exits.

The most common commands that need this pattern:

| Command  | Why it needs `&& exec $SHELL`                          |
|----------|--------------------------------------------------------|
| `cd`     | Changes directory only in the subshell, then is lost   |
| `source` | Sources a file in a subshell, leaving your env unchanged |
| `.`      | Same as `source` — the dot builtin                     |
| `export` | Sets env vars that disappear when the subshell ends    |
| `alias`  | Defines aliases that are lost when the subshell exits  |
| `unset`  | Unsets variables only in the subshell                  |

**Example with `cd`:**

```
ctrl+a → name: "go to project" → command: cd ~/projects/target-folder && exec $SHELL
```

When you pick this command, `cl` runs it and replaces the current
shell with a fresh one in the target directory — so the `cd`
actually persists.

> **Tip:** You only need `&& exec $SHELL` for commands that *change
> your shell state*. Regular commands like `git`, `npm`, `docker`,
> `ssh`, etc. work fine without it.

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

> Execute: build
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
