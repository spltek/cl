# cl shell integration for bash.
# Add to ~/.bashrc:
#   eval "$(cl init bash)"
#
# This defines a `cl` function that shadows the `cl` binary on PATH.
# Informational commands are passed straight through so their output
# prints normally instead of being captured. Everything else opens
# the interactive picker, where adding/editing/renaming/deleting
# commands happens via ctrl+a/ctrl+e/ctrl+r/ctrl+d, and ctrl+s toggles
# whether commands are shown/hidden. With commands hidden (the
# default), picking one runs it directly and the binary prints
# nothing, so there's nothing below for this function to do. With
# commands shown, the binary hands the picked command back on
# stdout; bash has no equivalent of zsh's `print -z` that works after
# a plain command invocation (the READLINE_LINE trick only works
# inside an active `bind -x` keybinding), so as a pragmatic
# equivalent it's shown as an editable, pre-filled line via
# `read -e -i`: a second Enter runs it.
cl() {
  case "$1" in
    init|-v|--version|-h|--help|help)
      command cl "$@"
      ;;
    *)
      local out
      out="$(command cl "$@")" || return $?
      if [[ -n "$out" ]]; then
        local line
        read -e -i "$out" -p "" line
        if [[ -n "$line" ]]; then
          eval -- "$line"
        fi
      fi
      ;;
  esac
}
