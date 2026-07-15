# cl shell integration for bash.
# Add to ~/.bashrc:
#   eval "$(cl init bash)"
#
# This defines a `cl` function that shadows the `cl` binary on PATH.
# Management commands (-add, -remove, init) are passed straight
# through. Bash has no equivalent of zsh's `print -z` that works after
# a plain command invocation (the READLINE_LINE trick only works
# inside an active `bind -x` keybinding), so as a pragmatic
# equivalent we immediately show the selected command as an editable,
# pre-filled line via `read -e -i`: a second Enter runs it.
cl() {
  case "$1" in
    -add|-remove|init)
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
