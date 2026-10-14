package watcher

import (
	"fmt"
	"regexp"
	"strings"
)

// ToRegex converts a glob pattern to an anchored regular expression string.
//
// Supported syntax:
//
//	*        any chars except '/'
//	?        exactly one char except '/'
//	**       any chars including '/' (when it forms a full path segment)
//	/**/     zero or more directories
//	{a,b,c}  alternation (can be nested)
//	[abc]    character class, [!abc] / [^abc] negated
//	\x       literal x
func wildcardToRegex(pattern string) (string, error) {
	var sb strings.Builder
	sb.WriteString("^")

	runes := []rune(pattern)
	braceDepth := 0

	for i := 0; i < len(runes); i++ {
		c := runes[i]

		switch c {
		case '*':
			// Consume a run of stars.
			start := i
			for i+1 < len(runes) && runes[i+1] == '*' {
				i++
			}
			isDouble := i > start

			// A "**" is a globstar only if it is a whole path segment.
			atSegStart := start == 0 || runes[start-1] == '/'
			atSegEnd := i+1 >= len(runes) || runes[i+1] == '/'

			switch {
			case isDouble && atSegStart && atSegEnd:
				if i+1 < len(runes) && runes[i+1] == '/' {
					// "**/" -> zero or more directories
					i++ // consume the slash
					sb.WriteString("(?:.*/)?")
				} else {
					// trailing "**" -> everything
					sb.WriteString(".*")
				}
			default:
				// plain "*" (or "**" embedded in a name, treated as "*")
				sb.WriteString("[^/]*")
			}

		case '?':
			sb.WriteString("[^/]")

		case '{':
			braceDepth++
			sb.WriteString("(?:")

		case '}':
			if braceDepth == 0 {
				sb.WriteString(`\}`) // stray '}' is literal
			} else {
				braceDepth--
				sb.WriteString(")")
			}

		case ',':
			if braceDepth > 0 {
				sb.WriteString("|")
			} else {
				sb.WriteString(",")
			}

		case '[':
			// Find closing bracket.
			j := i + 1
			if j < len(runes) && (runes[j] == '!' || runes[j] == '^') {
				j++
			}
			if j < len(runes) && runes[j] == ']' { // leading ']' is literal
				j++
			}
			for j < len(runes) && runes[j] != ']' {
				j++
			}
			if j >= len(runes) {
				// No closing bracket: treat '[' as literal.
				sb.WriteString(`\[`)
				break
			}
			class := runes[i+1 : j]
			sb.WriteString("[")
			if len(class) > 0 && (class[0] == '!' || class[0] == '^') {
				sb.WriteString("^")
				class = class[1:]
			}
			for _, r := range class {
				if r == '\\' || r == '[' {
					sb.WriteRune('\\')
				}
				sb.WriteRune(r)
			}
			sb.WriteString("]")
			i = j

		case '\\':
			if i+1 < len(runes) {
				i++
				sb.WriteString(regexp.QuoteMeta(string(runes[i])))
			} else {
				sb.WriteString(`\\`)
			}

		default:
			sb.WriteString(regexp.QuoteMeta(string(c)))
		}
	}

	if braceDepth != 0 {
		return "", fmt.Errorf("glob %q: unbalanced '{'", pattern)
	}

	sb.WriteString("$")
	return sb.String(), nil
}
