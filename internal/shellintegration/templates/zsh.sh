# cl shell integration for zsh.
# Add to ~/.zshrc:
#   eval "$(cl init zsh)"
#
# This defines a `cl` function that shadows the `cl` binary on PATH.
# Management commands (-add, -remove, init) are passed straight
# through. Interactive selections are pushed into the editing buffer
# of the *next* prompt via `print -z`, so the command appears
# pre-filled and a second Enter runs it exactly as if typed by hand.
cl() {
  case "$1" in
    -add|-remove|init)
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
