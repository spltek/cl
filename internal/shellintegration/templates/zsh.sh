# cl shell integration for zsh.
# Add to ~/.zshrc:
#   eval "$(cl init zsh)"
#
# This defines a `cl` function that shadows the `cl` binary on PATH.
# Informational commands are passed straight through so their output
# prints normally instead of being captured and pushed into the
# prompt buffer. Everything else opens the interactive picker, where
# adding/editing/removing commands happens via ctrl+a/ctrl+e/ctrl+r -
# that's the only invocation that needs the capture-and-inject dance
# below.
cl() {
  case "$1" in
    init|-v|--version|-h|--help|help)
      command cl "$@"
      ;;
    *)
      local out
      out="$(command cl "$@")" || return $?
      if [[ -n "$out" ]]; then
        print -z -- "$out"
      fi
      ;;
  esac
}
