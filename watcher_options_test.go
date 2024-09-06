package frankenphp

import (
	"github.com/stretchr/testify/assert"
	"testing"
	fswatch "github.com/dunglas/go-fswatch"
	"path/filepath"
)

func TestSimpleRecursiveWatchOption(t *testing.T) {
	const shortForm = "/some/path"

	watchOpt := createFromShortForm(shortForm, t)

	assert.Empty(t, watchOpt.wildCardPattern)
	assert.Equal(t, "/some/path", watchOpt.dirs[0])
	assert.True(t, watchOpt.isRecursive)
}

func TestSingleFileWatchOption(t *testing.T) {
	const shortForm = "/some/path/watch-me.json"

	watchOpt := createFromShortForm(shortForm, t)

	assert.Equal(t, "watch-me.json", watchOpt.wildCardPattern)
	assert.Equal(t, "/some/path", watchOpt.dirs[0])
	assert.False(t, watchOpt.isRecursive)
}

func TestNonRecursivePatternWatchOption(t *testing.T) {
	const shortForm = "/some/path/*.json"

	watchOpt := createFromShortForm(shortForm, t)

	assert.Equal(t, "*.json", watchOpt.wildCardPattern)
	assert.Equal(t, "/some/path", watchOpt.dirs[0])
	assert.False(t, watchOpt.isRecursive)
}

func TestRecursivePatternWatchOption(t *testing.T) {
	const shortForm = "/some/path/**/*.json"

	watchOpt := createFromShortForm(shortForm, t)

	assert.Equal(t, "*.json", watchOpt.wildCardPattern)
	assert.Equal(t, "/some/path", watchOpt.dirs[0])
	assert.True(t, watchOpt.isRecursive)
}

func TestRelativePathname(t *testing.T) {
	const shortForm = "../testdata/**/*.txt"
	absPath, err := filepath.Abs("../testdata")

	watchOpt := createFromShortForm(shortForm, t)

	assert.NoError(t, err)
	assert.Equal(t, "*.txt", watchOpt.wildCardPattern)
	assert.Equal(t, absPath, watchOpt.dirs[0])
	assert.True(t, watchOpt.isRecursive)
}

func TestCurrentRelativePath(t *testing.T) {
	const shortForm = "."
	absPath, err := filepath.Abs(shortForm)

	watchOpt := createFromShortForm(shortForm, t)

	assert.NoError(t, err)
	assert.Equal(t, "", watchOpt.wildCardPattern)
	assert.Equal(t, absPath, watchOpt.dirs[0])
	assert.True(t, watchOpt.isRecursive)
}

func TestMatchPatternWithoutExtension(t *testing.T) {
	const shortForm = "/some/path/**/fileName"

	watchOpt := createFromShortForm(shortForm, t)

	assert.Equal(t, "fileName", watchOpt.wildCardPattern)
	assert.Equal(t, "/some/path", watchOpt.dirs[0])
	assert.True(t, watchOpt.isRecursive)
}

func TestAddingTwoFilePaths(t *testing.T) {
	watchOpt := getDefaultWatchOpt()
	applyFirstPath := WithWatcherDirs([]string{"/first/path"})
	applySecondPath := WithWatcherDirs([]string{"/second/path"})

	err1 := applyFirstPath(&watchOpt)
	err2 := applySecondPath(&watchOpt)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, []string{"/first/path","/second/path"}, watchOpt.dirs)
}

func TestAddingAnInclusionFilterWithDefaultForExclusion(t *testing.T) {
	expectedInclusionFilter := fswatch.Filter{
		Text: "\\.php$", 
		FilterType: fswatch.FilterInclude, 
		CaseSensitive: true, 
		Extended: true,
	}
	expectedExclusionFilter := fswatch.Filter{
		Text: "\\.", 
		FilterType: fswatch.FilterExclude, 
		CaseSensitive: true, 
		Extended: true,
	}

	watchOpt := createWithOption(WithWatcherFilters("\\.php$", "", true, true), t)

	assert.Equal(t, []fswatch.Filter{expectedInclusionFilter, expectedExclusionFilter}, watchOpt.filters)
}

func TestWithExclusionFilter(t *testing.T) {
	expectedExclusionFilter := fswatch.Filter{
		Text: "\\.php$", 
		FilterType: fswatch.FilterExclude, 
		CaseSensitive: false, 
		Extended: false,
	}

	watchOpt := createWithOption(WithWatcherFilters("", "\\.php$", false, false), t)

	assert.Equal(t, []fswatch.Filter{expectedExclusionFilter}, watchOpt.filters)
}

func TestWithPollMonitor(t *testing.T) {
	watchOpt := createWithOption(WithWatcherMonitorType("poll"), t)

	assert.Equal(t, (int)(fswatch.PollMonitor), (int)(watchOpt.monitorType))
}

func TestWithSymlinks(t *testing.T) {
	watchOpt := createWithOption(WithWatcherSymlinks(true), t)

	assert.True(t, watchOpt.followSymlinks)
}

func TestWithoutRecursion(t *testing.T) {
	watchOpt := createWithOption(WithWatcherRecursion(false), t)

	assert.False(t, watchOpt.isRecursive)
}

func TestWithLatency(t *testing.T) {
	watchOpt := createWithOption(WithWatcherLatency(500), t)

	assert.Equal(t, 0.5, watchOpt.latency)
}

func TestAllowReloadOnMatch(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	watchOpt := createFromShortForm("/some/path/**/*.php", t)

	assert.True(t, watchOpt.allowReload(fileName))
}

func TestAllowReloadOnExactMatch(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	watchOpt := createFromShortForm("/some/path/watch-me.php", t)

	assert.True(t, watchOpt.allowReload(fileName))
}

func TestDisallowReload(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	watchOpt := createFromShortForm("/some/path/dont-watch.php", t)

	assert.False(t, watchOpt.allowReload(fileName))
}

func TestAllowReloadOnRecursiveDirectory(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	watchOpt := createFromShortForm("/some", t)

	assert.True(t, watchOpt.allowReload(fileName))
}

func TestAllowReloadIfOptionIsNotAWildcard(t *testing.T) {
	const fileName = "/some/path/watch-me.php"
	watchOpt := getDefaultWatchOpt()

	assert.True(t, watchOpt.allowReload(fileName))
}

func createFromShortForm(shortForm string, t *testing.T) watchOpt {
	watchOpt := getDefaultWatchOpt()
    applyOptions := WithWatcherShortForm(shortForm)
    err := applyOptions(&watchOpt)
    assert.NoError(t, err)
    return watchOpt
}

func createWithOption(applyOptions WatchOption, t *testing.T) watchOpt {
	watchOpt := getDefaultWatchOpt()

	err := applyOptions(&watchOpt)

	assert.NoError(t, err)
	return watchOpt
}