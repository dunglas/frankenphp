package frankenphp

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"path/filepath"
)

func TestSimpleRecursiveWatchOption(t *testing.T) {
	const fileName = "/some/path"

	watchOpt, err := createWatchOption(fileName)

	assert.NoError(t, err)
	assert.Empty(t, watchOpt.pattern)
	assert.Equal(t, "/some/path", watchOpt.dirName)
	assert.True(t, watchOpt.isRecursive)
}

func TestSingleFileWatchOption(t *testing.T) {
	const fileName = "/some/path/watch-me.json"

	watchOpt, err := createWatchOption(fileName)

	assert.NoError(t, err)
	assert.Equal(t, "watch-me.json", watchOpt.pattern)
	assert.Equal(t, "/some/path", watchOpt.dirName)
	assert.False(t, watchOpt.isRecursive)
}

func TestNonRecursivePatternWatchOption(t *testing.T) {
	const fileName = "/some/path/*.json"

	watchOpt, err := createWatchOption(fileName)

	assert.NoError(t, err)
	assert.Equal(t, "*.json", watchOpt.pattern)
	assert.Equal(t, "/some/path", watchOpt.dirName)
	assert.False(t, watchOpt.isRecursive)
}

func TestRecursivePatternWatchOption(t *testing.T) {
	const fileName = "/some/path/**/*.json"

	watchOpt, err := createWatchOption(fileName)

	assert.NoError(t, err)
	assert.Equal(t, "*.json", watchOpt.pattern)
	assert.Equal(t, "/some/path", watchOpt.dirName)
	assert.True(t, watchOpt.isRecursive)
}

func TestRelativePathname(t *testing.T) {
	const fileName = "../testdata/**/*.txt"
	absPath, err1 := filepath.Abs("../testdata")

	watchOpt, err2 := createWatchOption(fileName)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, "*.txt", watchOpt.pattern)
	assert.Equal(t, absPath, watchOpt.dirName)
	assert.True(t, watchOpt.isRecursive)
}

func TestMatchLiteralFilePattern(t *testing.T) {
	const fileName = "/some/path/**/fileName"

	watchOpt, err := createWatchOption(fileName)

	assert.NoError(t, err)
	assert.Equal(t, "fileName", watchOpt.pattern)
	assert.Equal(t, "/some/path", watchOpt.dirName)
	assert.True(t, watchOpt.isRecursive)
}

func TestPatternShouldMatch(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	wOpt := watchOpt{pattern: "*.php", dirName: "/some/path", isRecursive: true}

	assert.True(t, fileMatchesPattern(fileName, wOpt))
}

func TestPatternShouldMatchExactly(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	wOpt := watchOpt{pattern: "watch-me.php", dirName: "/some/path", isRecursive: true}

	assert.True(t, fileMatchesPattern(fileName, wOpt))
}

func TestPatternShouldNotMatch(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	wOpt := watchOpt{pattern: "*.json", dirName: "/some/path", isRecursive: true}

	assert.False(t, fileMatchesPattern(fileName, wOpt))
}

func TestPatternShouldNotMatchExactly(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	wOpt := watchOpt{pattern: "watch-me-too.php", dirName: "/some/path", isRecursive: true}

	assert.False(t, fileMatchesPattern(fileName, wOpt))
}

func TestEmptyPatternShouldAlwaysMatch(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	wOpt := watchOpt{pattern: "", dirName: "/some/path", isRecursive: true}

	assert.True(t, fileMatchesPattern(fileName, wOpt))
}


