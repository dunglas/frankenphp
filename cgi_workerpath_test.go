package frankenphp

import (
	"net/http"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAssignedWorkerScriptPath(t *testing.T) {
	t.Parallel()
	sep := string(filepath.Separator)
	root := filepath.Join(sep, "var", "www")
	cases := []struct{ name, root, file string }{
		{"inside", root, root + sep + "index.php"},
		{"neighbor", root, root + "-other" + sep + "index.php"},
		{"exact", root, root},
		{"empty filename", root, ""},
		{"empty root", "", sep + "index.php"},
		{"empty paths", "", ""},
		{"trailing separator", root + sep, root + sep + sep + "index.php"},
		{"missing second separator", root + sep, root + sep + "index.php"},
		{"filesystem root", sep, sep + sep + "index.php"},
		{"unicode", root + sep + "приложение", root + sep + "приложение" + sep + "index.php"},
		{"binary", root + "\x00\xff", root + "\x00\xff" + sep + "index.php"},
		{"long root", strings.Repeat(root, 16), strings.Repeat(root, 16) + sep + "index.php"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) { checkAssignedWorkerScriptPath(t, tt.root, tt.file, "/index.php/info") })
	}
}

func checkAssignedWorkerScriptPath(t *testing.T, root, file, path string) {
	t.Helper()
	actual := frankenPHPContext{
		documentRoot: root,
		request:      &http.Request{URL: &url.URL{Path: path}},
		worker:       &worker{fileName: file},
	}
	expected := actual
	splitCgiPathBeforePrefixSearch(&expected)
	splitCgiPath(&actual)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("root=%q file=%q path=%q: got script=%q docURI=%q pathInfo=%q, want script=%q docURI=%q pathInfo=%q", root, file, path, actual.scriptName, actual.docURI, actual.pathInfo, expected.scriptName, expected.docURI, expected.pathInfo)
	}
}

func FuzzAssignedWorkerScriptPath(f *testing.F) {
	sep := string(filepath.Separator)
	f.Add(sep+"var"+sep+"www", sep+"var"+sep+"www"+sep+"index.php", "/index.php/info")
	f.Add("", "", "/index.php")
	f.Add(sep, sep+sep+"index.php", "/a.php.txt/b.php")
	f.Add("\x00\xff", "\x00\xff"+sep+"index.php", "/shell.ｐｈｐ")
	f.Fuzz(func(t *testing.T, root, file, path string) { checkAssignedWorkerScriptPath(t, root, file, path) })
}

// The previous implementation is retained as the differential fuzzing oracle.
func splitCgiPathBeforePrefixSearch(fc *frankenPHPContext) {
	path := fc.request.URL.Path
	splitPath := fc.splitPath

	if splitPath == nil {
		splitPath = []string{".php"}
	}

	if splitPos := splitPos(path, splitPath); splitPos > -1 {
		fc.docURI = path[:splitPos]
		fc.pathInfo = path[splitPos:]
	}

	// If a worker is already assigned explicitly, derive SCRIPT_NAME from its filename
	if fc.worker != nil {
		fc.scriptFilename = fc.worker.fileName
		docRootWithSep := fc.documentRoot + string(filepath.Separator)
		if strings.HasPrefix(fc.worker.fileName, docRootWithSep) {
			fc.scriptName = filepath.ToSlash(strings.TrimPrefix(fc.worker.fileName, fc.documentRoot))
		} else {
			fc.docURI = ""
			fc.pathInfo = ""
		}
		return
	}

	// Strip PATH_INFO from SCRIPT_NAME
	// Ensure the SCRIPT_NAME has a leading slash for compliance with RFC3875
	// Info: https://tools.ietf.org/html/rfc3875#section-4.1.13
	fc.scriptName = ensureLeadingSlash(strings.TrimSuffix(path, fc.pathInfo))

	// TODO: is it possible to delay this and avoid saving everything in the context?
	// SCRIPT_FILENAME is the absolute path of SCRIPT_NAME
	fc.scriptFilename = sanitizedPathJoin(fc.documentRoot, fc.scriptName)

	// see if a php_server worker or global worker matches the request path
	// aka: root + request path == worker.filename
	if fc.worker = fc.server.workersByPath[fc.scriptFilename]; fc.worker == nil {
		fc.worker = globalWorkersByPath[fc.scriptFilename]
	}
}

func BenchmarkAssignedWorkerScriptPath(b *testing.B) {
	sep := string(filepath.Separator)
	root := filepath.Join(sep, "var", "www")
	longRoot := filepath.Join(sep, "srv", "apps", "organization", "production", "releases", "2026", "public")
	for _, tt := range []struct{ name, root, file string }{
		{"short_root", root, root + sep + "index.php"},
		{"long_root", longRoot, longRoot + sep + "index.php"},
		{"outside_root", longRoot, filepath.Join(sep, "srv", "elsewhere", "index.php")},
		{"empty_root", "", sep + "index.php"},
		{"trailing_separator", root + sep, root + sep + sep + "index.php"},
		{"exact_root", root, root},
	} {
		b.Run(tt.name, func(b *testing.B) {
			fc := &frankenPHPContext{documentRoot: tt.root, request: &http.Request{URL: &url.URL{Path: "/index.php/info"}}, worker: &worker{fileName: tt.file}}
			b.ReportAllocs()
			for b.Loop() {
				splitCgiPath(fc)
			}
			benchmarkAssignedWorkerScriptName = fc.scriptName
		})
	}
}

var benchmarkAssignedWorkerScriptName string
