package tui

import (
	"regexp"
	"strings"
)

// placeholderRe matches {{name}} and {{name:default}} patterns in
// command strings. Named capture groups aren't used; positions from
// FindAllStringSubmatchIndex are used instead so byte offsets within
// the original command are also available for reconstruction.
var placeholderRe = regexp.MustCompile(`\{\{(\w+)(?::([^}]*))?\}\}`)

// placeholder represents a single {{name}} or {{name:default}}
// occurrence inside a command string, together with its byte offsets
// into the original (unresolved) command.
type placeholder struct {
	Name    string
	Default string
	Start   int // byte offset of the opening "{{"
	End     int // byte offset just past the closing "}}"
}

// parsePlaceholders extracts every {{name[:default]}} placeholder
// from command and returns them in left-to-right order.
func parsePlaceholders(command string) []placeholder {
	matches := placeholderRe.FindAllStringSubmatchIndex(command, -1)
	phs := make([]placeholder, 0, len(matches))
	for _, m := range matches {
		ph := placeholder{
			Name:  command[m[2]:m[3]], // capture group 1 (name)
			Start: m[0],
			End:   m[1],
		}
		if m[4] >= 0 { // capture group 2 (default) is present
			ph.Default = command[m[4]:m[5]]
		}
		phs = append(phs, ph)
	}
	return phs
}

// resolveCommand builds the final command string by replacing every
// placeholder with the corresponding value in values. If a value is
// empty, the placeholder's Default is used instead. Replacements are
// applied from right to left so that earlier byte offsets remain
// valid regardless of how much each replacement changes the string
// length.
func resolveCommand(command string, placeholders []placeholder, values []string) string {
	result := command
	for i := len(placeholders) - 1; i >= 0; i-- {
		ph := placeholders[i]
		val := values[i]
		if val == "" {
			val = ph.Default
		}
		result = result[:ph.Start] + val + result[ph.End:]
	}
	return result
}

// buildParamHint returns a human-readable parameter list like
// "[param1, param2(default:10)]" from the given placeholders. It
// returns an empty string when there are no placeholders.
func buildParamHint(placeholders []placeholder) string {
	if len(placeholders) == 0 {
		return ""
	}
	parts := make([]string, len(placeholders))
	for i, ph := range placeholders {
		if ph.Default != "" {
			parts[i] = ph.Name + "(default:" + ph.Default + ")"
		} else {
			parts[i] = ph.Name
		}
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// buildPreview returns a human-readable preview of command where:
//   - every placeholder with index < currentIdx shows its resolved
//     value from values (or the Default if the value is empty),
//   - the placeholder at currentIdx shows currentText when non-empty
//     (the in-progress typed value — checked before values[i], which
//     may already hold a prefilled default), otherwise keeps its
//     original {{name[:default]}} form,
//   - every placeholder with index > currentIdx is left in its
//     original {{name[:default]}} form so the user sees what's left.
//
// Replacements are applied right-to-left to preserve byte offsets.
func buildPreview(command string, placeholders []placeholder, values []string, currentIdx int, currentText string) string {
	result := command
	for i := len(placeholders) - 1; i >= 0; i-- {
		ph := placeholders[i]
		var replacement string
		switch {
		case i == currentIdx:
			// Live form text always wins over a stale prefilled value
			// in values[currentIdx] (defaults are stored there before
			// the user finishes editing the field).
			if currentText != "" {
				replacement = currentText
			} else if ph.Default != "" {
				replacement = "{{" + ph.Name + ":" + ph.Default + "}}"
			} else {
				replacement = "{{" + ph.Name + "}}"
			}
		case i < currentIdx:
			val := ""
			if i < len(values) {
				val = values[i]
			}
			if val == "" {
				val = ph.Default
			}
			replacement = val
		default: // i > currentIdx — still to fill
			if ph.Default != "" {
				replacement = "{{" + ph.Name + ":" + ph.Default + "}}"
			} else {
				replacement = "{{" + ph.Name + "}}"
			}
		}
		result = result[:ph.Start] + replacement + result[ph.End:]
	}
	return result
}
