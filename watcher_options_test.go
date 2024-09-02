package frankenphp

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"path/filepath"
)

func TestSimpleRecursiveWatchOption(t *testing.T) {
	const fileName = "/some/path"

	watchOpt, err := createWatchOption(fileName)

	assert.Nil(t, err)
	assert.Equal(t, "", watchOpt.pattern)
	assert.Equal(t, "/some/path", watchOpt.dirName)
	assert.True(t, watchOpt.isRecursive)
}

func TestSingleFileWatchOption(t *testing.T) {
	const fileName = "/some/path/watch-me.json"

	watchOpt, err := createWatchOption(fileName)

	assert.Nil(t, err)
	assert.Equal(t, "watch-me.json", watchOpt.pattern)
	assert.Equal(t, "/some/path", watchOpt.dirName)
	assert.False(t, watchOpt.isRecursive)
}

func TestNonRecursivePatternWatchOption(t *testing.T) {
	const fileName = "/some/path/*.json"

	watchOpt, err := createWatchOption(fileName)

	assert.Nil(t, err)
	assert.Equal(t, "*.json", watchOpt.pattern)
	assert.Equal(t, "/some/path", watchOpt.dirName)
	assert.False(t, watchOpt.isRecursive)
}

func TestRecursivePatternWatchOption(t *testing.T) {
	const fileName = "/some/path/**/*.json"

	watchOpt, err := createWatchOption(fileName)

	assert.Nil(t, err)
	assert.Equal(t, "*.json", watchOpt.pattern)
	assert.Equal(t, "/some/path", watchOpt.dirName)
	assert.True(t, watchOpt.isRecursive)
}

func TestRelativePathname(t *testing.T) {
	const fileName = "../testdata/**/*.txt"
	absPath, err1 := filepath.Abs("../testdata")

	watchOpt, err2 := createWatchOption(fileName)

	assert.Nil(t, err1)
	assert.Nil(t, err2)
	assert.Equal(t, "*.txt", watchOpt.pattern)
	assert.Equal(t, absPath, watchOpt.dirName)
	assert.True(t, watchOpt.isRecursive)
}

func TestShouldWatchWithoutPattern(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	wOpt := watchOpt{pattern: "", dirName: "/some/path", isRecursive: false}
	watchOpts := []watchOpt{wOpt}

	assert.True(t, fileMatchesPattern(fileName, watchOpts))
}

func TestShouldNotWatchBecauseOfNoRecursion(t *testing.T) {
	const fileName = "/some/path/sub-path/watch-me.php"
	wOpt := watchOpt{pattern: ".php", dirName: "/some/path", isRecursive: false}
	watchOpts := []watchOpt{wOpt}

	assert.False(t, fileMatchesPattern(fileName, watchOpts))
}

func TestShouldWatchBecauseOfRecursion(t *testing.T) {
	const fileName = "/some/path/sub-path/watch-me.php"
	wOpt := watchOpt{pattern: "", dirName: "/some/path", isRecursive: true}
	watchOpts := []watchOpt{wOpt}

	assert.True(t, fileMatchesPattern(fileName, watchOpts))
}

func TestShouldWatchBecauseOfPatters(t *testing.T) {
	const fileName = "/some/path/sub-path/watch-me.php"
	wOpt := watchOpt{pattern: "*.php", dirName: "/some/path", isRecursive: true}
	watchOpts := []watchOpt{wOpt}

	assert.True(t, fileMatchesPattern(fileName, watchOpts))
}

func TestShouldNotWatchBecauseOfPattern(t *testing.T) {
	const fileName = "/some/path/sub-path/watch-me.php"
	wOpt := watchOpt{pattern: "*.json", dirName: "/some/path", isRecursive: true}
	watchOpts := []watchOpt{wOpt}

	assert.False(t, fileMatchesPattern(fileName, watchOpts))
}

func TestShouldMatchWithMultipleWatchOptions(t *testing.T) {
	const fileName = "/third/path/watch-me.php"
	wOpt1 := watchOpt{pattern: "*.php", dirName: "/first/path", isRecursive: true}
	wOpt2 := watchOpt{pattern: "*.php", dirName: "/second/path", isRecursive: true}
	wOpt3 := watchOpt{pattern: "*.php", dirName: "/third/path", isRecursive: true}
	watchOpts := []watchOpt{wOpt1,wOpt2,wOpt3}

	assert.True(t, fileMatchesPattern(fileName, watchOpts))
}

