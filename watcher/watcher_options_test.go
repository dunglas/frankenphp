package watcher

import (
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

func TestRecursiveDirectoryWithoutPattern(t *testing.T) {
	watchOpts, err := parseFilePatterns([]string {
		"/path/to/folder1",
		"/path/to/folder2/**/",
		"/path/to/folder3/**",
		"./",
		".",
		"",
	})

	assert.NoError(t, err)
	assert.Len(t, watchOpts, 6)
	assert.Equal(t, "/path/to/folder1", watchOpts[0].dir)
	assert.Equal(t, "/path/to/folder2", watchOpts[1].dir)
	assert.Equal(t, "/path/to/folder3", watchOpts[2].dir)
	assert.Equal(t, currentDir(t), watchOpts[3].dir)
	assert.Equal(t, currentDir(t), watchOpts[4].dir)
	assert.Equal(t, currentDir(t), watchOpts[5].dir)
	assertAllRecursive(t, watchOpts, true)
	assertAllPattern(t, watchOpts, "")
}

func TestRecursiveDirectoryWithPattern(t *testing.T) {
	watchOpts, err := parseFilePatterns([]string {
		"/path/to/folder1/**/*.php",
		"/path/to/folder2/**/.env",
		"/path/to/folder3/**/filename",
		"**/?.php",
	})

	assert.NoError(t, err)
	assert.Len(t, watchOpts, 4)
	assert.Equal(t, "/path/to/folder1", watchOpts[0].dir)
	assert.Equal(t, "/path/to/folder2", watchOpts[1].dir)
	assert.Equal(t, "/path/to/folder3", watchOpts[2].dir)
	assert.Equal(t, currentDir(t), watchOpts[3].dir)
	assert.Equal(t, "*.php", watchOpts[0].pattern)
	assert.Equal(t, ".env", watchOpts[1].pattern)
	assert.Equal(t, "filename", watchOpts[2].pattern)
	assert.Equal(t, "?.php", watchOpts[3].pattern)
	assertAllRecursive(t, watchOpts, true)
}

func TestNonRecursiveDirectoryWithPattern(t *testing.T) {
	watchOpts, err := parseFilePatterns([]string {
		"/path/to/folder1/*",
		"/path/to/folder2/*.php",
		"./*.php",
		"*.php",
	})

	assert.NoError(t, err)
	assert.Len(t, watchOpts, 4)
	assert.Equal(t, "/path/to/folder1", watchOpts[0].dir)
	assert.Equal(t, "/path/to/folder2", watchOpts[1].dir)
	assert.Equal(t, currentDir(t), watchOpts[2].dir)
	assert.Equal(t, currentDir(t), watchOpts[3].dir)
	assert.Equal(t, "*", watchOpts[0].pattern)
	assert.Equal(t, "*.php", watchOpts[1].pattern)
	assert.Equal(t, "*.php", watchOpts[2].pattern)
	assert.Equal(t, "*.php", watchOpts[2].pattern)
	assertAllRecursive(t, watchOpts, false)
}

func TestAllowReloadOnMatchingPattern(t *testing.T) {
	const fileName = "/some/path/watch-me.php"

	watchOpt, err := parseFilePattern("/some/path/*.php")

	assert.NoError(t, err)
	assert.True(t, watchOpt.allowReload(fileName, 0, 0))
}

func TestAllowReloadOnExactMatch(t *testing.T) {
	const fileName = "/some/path/watch-me.php"

	watchOpt, err := parseFilePattern("/some/path/watch-me.php")

	assert.NoError(t, err)
	assert.True(t, watchOpt.allowReload(fileName, 0, 0))
}

func TestDisallowOnDifferentFilename(t *testing.T) {
	const fileName = "/some/path/watch-me.php"

	watchOpt, err := parseFilePattern("/some/path/dont-watch.php")

	assert.NoError(t, err)
	assert.False(t, watchOpt.allowReload(fileName, 0, 0))
}

func TestAllowReloadOnRecursivePattern(t *testing.T) {
	const fileName = "/some/path/watch-me.php"

	watchOpt, err := parseFilePattern("/some/**/*.php")

	assert.NoError(t, err)
	assert.True(t, watchOpt.allowReload(fileName, 0, 0))
}

func TestAllowReloadWithRecursionAndNoPattern(t *testing.T) {
	const fileName = "/some/path/watch-me.php"

	watchOpt, err := parseFilePattern("/some/")

	assert.NoError(t, err)
	assert.True(t, watchOpt.allowReload(fileName, 0, 0))
}

func TestDisallowOnDifferentRecursivePattern(t *testing.T) {
	const fileName = "/some/path/watch-me.php"

	watchOpt, err := parseFilePattern("/some/**/*.html")

	assert.NoError(t, err)
	assert.False(t, watchOpt.allowReload(fileName, 0, 0))
}

func TestDisallowOnMissingRecursion(t *testing.T) {
	const fileName = "/some/path/watch-me.php"

	watchOpt, err := parseFilePattern("/some/*.php")

	assert.NoError(t, err)
	assert.False(t, watchOpt.allowReload(fileName, 0, 0))
}

func TestDisallowOnEventTypeBiggerThan3(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	const eventType = 4

	watchOpt, err := parseFilePattern("/some/path")

	assert.NoError(t, err)
	assert.False(t, watchOpt.allowReload(fileName, eventType, 0))
}

func TestDisallowOnPathTypeBiggerThan2(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	const pathType = 3

	watchOpt, err := parseFilePattern("/some/path")

	assert.NoError(t, err)
	assert.False(t, watchOpt.allowReload(fileName, 0, pathType))
}

func currentDir(t *testing.T) string {
	dir, err := filepath.Abs(".")
	assert.NoError(t, err)
	return dir
}

func assertAllRecursive(t *testing.T, watchOpts []*watchOpt, isRecursive bool) {
	for _, w := range watchOpts {
		assert.Equal(t, isRecursive, w.isRecursive)
	}
}

func assertAllPattern(t *testing.T, watchOpts []*watchOpt, pattern string) {
	for _, w := range watchOpts {
		assert.Equal(t, pattern, w.pattern)
	}
}

