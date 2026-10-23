package frankenphp

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitPosPrefixSearch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		path    string
		markers []string
		want    int
	}{
		{"letter prefix folds case", "/INDEX.PHP/info", []string{"php"}, 10},
		{"single letter folds case", "/P/info", []string{"p"}, 2},
		{"single punctuation", "/a./info", []string{"."}, 3},
		{"nonmatching prefix before match", "/a.phx/b.PHP/info", []string{".php"}, 12},
		{"last possible start", "/a.php", []string{".php"}, 6},
		{"marker longer than path", "/a", []string{".php"}, -1},
		{"invalid first occurrence stops marker", "/a.php.txt/b.php", []string{".php"}, -1},
		{"later marker after invalid occurrence", "/a.php.txt/b.phtml", []string{".php", ".phtml"}, 18},
		{"marker order wins over path order", "/a.php/b.phtml", []string{".phtml", ".php"}, 14},
		{"empty marker skipped", "/a.php", []string{"", ".php"}, 6},
		{"nil markers", "/a.php", nil, 0},
		{"invalid UTF8 before extension", "/\xff.PHP/info", []string{".php"}, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitPos(tt.path, tt.markers); got != tt.want {
				t.Fatalf("splitPos(%q, %q) = %d, want %d", tt.path, tt.markers, got, tt.want)
			}
		})
	}
}

// Keep the original scan as an independent oracle for fuzzing the optimized
// prefix search, including unvalidated marker bytes and marker ordering.
func splitPosReference(path string, splitPath []string) int {
	if len(splitPath) == 0 {
		return 0
	}

	pathLen := len(path)

	for _, split := range splitPath {
		splitLen := len(split)
		if splitLen == 0 || splitLen > pathLen {
			continue
		}

		for i := 0; i <= pathLen-splitLen; i++ {
			match := true
			for j := range splitLen {
				c := path[i+j]
				if c >= utf8.RuneSelf {
					match = false

					break
				}

				if 'A' <= c && c <= 'Z' {
					c += 'a' - 'A'
				}

				if c != split[j] {
					match = false

					break
				}
			}

			if match {
				pos := i + splitLen
				// the split must end the path or a whole path segment: without
				// this, ".php" embedded in a filename ("/a.php.txt/b.php")
				// would split inside that filename and run a script the router
				// never matched. Give up on this split string entirely rather
				// than looking for a later occurrence of it, so the result
				// stays identical to Caddy's file matcher (firstSplit).
				if pos != pathLen && path[pos] != '/' {
					break
				}

				return pos
			}
		}
	}

	return -1
}

func FuzzSplitPosEquivalent(f *testing.F) {
	for _, seed := range []struct{ path, first, second string }{
		{"/index.php/info", ".php", ".phtml"},
		{"/a.php.txt/b.phtml", ".php", ".phtml"},
		{"/INDEX.PHP/info", "php", ".php"},
		{"/a.php/b.phtml", ".phtml", ".php"},
		{"/shell.ｐｈｐ", ".php", ""},
		{"/\xff.PHP", ".php", "\xff"},
		{"", "", ""},
	} {
		f.Add(seed.path, seed.first, seed.second)
	}
	f.Fuzz(func(t *testing.T, path, first, second string) {
		markers := []string{first, second}
		got, want := splitPos(path, markers), splitPosReference(path, markers)
		if got != want {
			t.Fatalf("splitPos(%q, %q) = %d, reference = %d", path, markers, got, want)
		}
	})
}

func BenchmarkSplitPos(b *testing.B) {
	cases := []struct {
		name, path string
		markers    []string
	}{
		{"short", "/index.php", []string{".php"}},
		{"long", "/some/long/application/public/directory/routes/controller/index.php/path/info", []string{".php"}},
		{"no_match", "/some/long/application/public/directory/assets/styles/main.css", []string{".php"}},
		{"uppercase", "/INDEX.PHP/path", []string{".php"}},
		{"many_dots", "/" + strings.Repeat(".", 64) + "/index.php", []string{".php"}},
		{"partial_matches", "/" + strings.Repeat(".phx/", 16) + "index.php", []string{".php"}},
		{"invalid_first", "/a.php.txt/b.php", []string{".php"}},
		{"second_marker", "/index.phtml/info", []string{".php", ".phtml"}},
		{"letter_prefix", "/INDEX.PHP/path", []string{"php"}},
		{"unicode_path", "/приложение/публичный/index.PHP/info", []string{".php"}},
		{"empty_path", "", []string{".php"}},
		{"empty_markers", "/index.php", nil},
	}
	for _, tt := range cases {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				benchmarkSplitPosResult = splitPos(tt.path, tt.markers)
			}
		})
	}
}

var benchmarkSplitPosResult int
