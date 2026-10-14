package watcher

import (
	"regexp"
	"strings"
	"path/filepath"
	"fmt"

	"github.com/dunglas/frankenphp/internal/fastabs"

)

func extractRootAndRegex(pattern string) (string, string, error) {
	forceDir := strings.HasSuffix(pattern, "/") || strings.HasSuffix(pattern, string(filepath.Separator))

	absPattern, err := fastabs.FastAbs(pattern) // joins with cwd if relative, and cleans
	if err != nil {
		return "", "", err
	}

	segs := strings.Split(filepath.ToSlash(absPattern), "/")
	n := 0
	for n < len(segs) && !strings.ContainsAny(segs[n], `*?[{\`) {
		n++
	}

	// No wildcard anywhere: the pattern is a concrete path.
	if n == len(segs) {
		// ".x" counts as an extension, a bare "." (e.g. "foo.") does not.
		isFile := !forceDir && len(filepath.Ext(absPattern)) > 1
		if isFile {
			// Watch the parent so atomic saves (write temp + rename)
			// don't invalidate the watch; filter events by name.
			return filepath.Dir(absPattern), absPattern, nil
		}
		return absPattern, "", nil
	}

	// Wildcard present: watch the topmost static directory.
	dir := strings.Join(segs[:n], "/")
	if dir == "" {
		dir = "/"
	}

	regex, err := wildcardToRegex(absPattern)
	if err != nil {
		return "", "", err
	}

	return filepath.FromSlash(dir), regex, nil
}

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
	pattern, err := fastabs.FastAbs(pattern)
	if err != nil {
		return "", err
	}

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

			switch {
			case !isDouble:
				// plain "*" -> anything except '/'
				sb.WriteString("[^/]*")

			case i+1 < len(runes) && runes[i+1] == '/' &&
				(start == 0 || runes[start-1] == '/'):
				// "**/" at a segment start -> zero or more directories,
				// so "a/**/b" matches "a/b", "a/x/b", "a/x/y/b".
				i++ // consume the slash
				sb.WriteString("(?:.*/)?")

			default:
				// any other "**" -> anything, including '/'
				sb.WriteString(".*")
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
