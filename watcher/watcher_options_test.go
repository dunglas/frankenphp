package watcher

import (
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

func TestWithRecursion(t *testing.T) {
	watchOpt := createWithOptions(t, WithWatcherRecursion(true))

	assert.True(t, watchOpt.isRecursive)
}

func TestWithWatcherPattern(t *testing.T) {
	watchOpt := createWithOptions(t, WithWatcherPattern("*php"))

	assert.Equal(t, "*php", watchOpt.pattern)
}

func TestWithWatcherDir(t *testing.T) {
	watchOpt := createWithOptions(t, WithWatcherDirs([]string{"/path/to/app"}))

	assert.Equal(t, "/path/to/app", watchOpt.dirs[0])
}

func TestWithRelativeWatcherDir(t *testing.T) {
	absoluteDir, err := filepath.Abs(".")

	watchOpt := createWithOptions(t, WithWatcherDirs([]string{"."}))

	assert.NoError(t, err)
	assert.Equal(t, absoluteDir, watchOpt.dirs[0])
}

func TestAllowReloadOnMatchingPattern(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	watchOpt := createWithOptions(
		t,
		WithWatcherDirs([]string{"/some/path"}),
		WithWatcherPattern("*.php"),
	)

	assert.True(t, watchOpt.allowReload(fileName, 0, 0))
}

func TestAllowReloadOnExactMatch(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	watchOpt := createWithOptions(
		t,
		WithWatcherDirs([]string{"/some/path"}),
		WithWatcherPattern("watch-me.php"),
	)

	assert.True(t, watchOpt.allowReload(fileName, 0, 0))
}

func TestDisallowOnDifferentFilename(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	watchOpt := createWithOptions(
		t,
		WithWatcherDirs([]string{"/some/path"}),
		WithWatcherPattern("dont-watch.php"),
	)

	assert.False(t, watchOpt.allowReload(fileName, 0, 0))
}

func TestAllowReloadOnRecursiveDirectory(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	watchOpt := createWithOptions(
		t,
		WithWatcherDirs([]string{"/some"}),
		WithWatcherRecursion(true),
		WithWatcherPattern("*.php"),
	)

	assert.True(t, watchOpt.allowReload(fileName, 0, 0))
}

func TestAllowReloadWithRecursionAndNoPattern(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	watchOpt := createWithOptions(
		t,
		WithWatcherDirs([]string{"/some"}),
		WithWatcherRecursion(true),
	)

	assert.True(t, watchOpt.allowReload(fileName, 0, 0))
}

func TestDisallowOnDifferentPatterns(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	watchOpt := createWithOptions(
		t,
		WithWatcherDirs([]string{"/some"}),
		WithWatcherRecursion(true),
		WithWatcherPattern(".txt"),
	)

	assert.False(t, watchOpt.allowReload(fileName, 0, 0))
}

func TestDisallowOnMissingRecursion(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	watchOpt := createWithOptions(
		t,
		WithWatcherDirs([]string{"/some"}),
		WithWatcherRecursion(false),
		WithWatcherPattern(".php"),
	)

	assert.False(t, watchOpt.allowReload(fileName, 0, 0))
}

func TestDisallowOnEventTypeBiggerThan3(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	const eventType = 4
	watchOpt := createWithOptions(
		t,
		WithWatcherDirs([]string{"/some/path"}),
		WithWatcherPattern("watch-me.php"),
	)

	assert.False(t, watchOpt.allowReload(fileName, eventType, 0))
}

func TestDisallowOnPathTypeBiggerThan2(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	const pathType = 3
	watchOpt := createWithOptions(
		t,
		WithWatcherDirs([]string{"/some/path"}),
		WithWatcherPattern("watch-me.php"),
	)

	assert.False(t, watchOpt.allowReload(fileName, 0, pathType))
}

func createWithOptions(t *testing.T, applyOptions ...WatchOption) WatchOpt {
	watchOpt := WatchOpt{}

	for _, applyOption := range applyOptions {
		err := applyOption(&watchOpt)
		assert.NoError(t, err)
	}
	return watchOpt
}
