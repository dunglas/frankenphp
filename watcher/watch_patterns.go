package watcher

import (
	"go.uber.org/zap"
	"path/filepath"
	"strings"
)

type watchPattern struct {
	dir         string
	isRecursive bool
	patterns    []string
	trigger     chan struct{}
}

func parseFilePatterns(filePatterns []string) ([]*watchPattern, error) {
	watchPatterns := make([]*watchPattern, 0, len(filePatterns))
	for _, filePattern := range filePatterns {
		watchPattern, err := parseFilePattern(filePattern)
		if err != nil {
			return nil, err
		}
		watchPatterns = append(watchPatterns, watchPattern)
	}
	return watchPatterns, nil
}

// this method prepares the watchPattern struct for a single file pattern (aka /path/*pattern)
// TODO: using '/' is more efficient than filepath functions, but does not work on windows
// if windows is ever supported this needs to be adjusted
func parseFilePattern(filePattern string) (*watchPattern, error) {
	absPattern, err := filepath.Abs(filePattern)
	if err != nil {
		return nil, err
	}

	w := &watchPattern{isRecursive: true, dir: absPattern}

	// first we try to split the pattern to determine
	// where the directory ends and the pattern starts
	splitPattern := strings.Split(absPattern, "/")
	patternWithoutDir := ""
	for i, part := range splitPattern {
		// we found a pattern if it contains a glob character [*?
		// if it contains a '.' it is also likely a filename
		// TODO: directories with a '.' in the name are currently not allowed
		if strings.ContainsAny(part, "[*?.") {
			patternWithoutDir = filepath.Join(splitPattern[i:]...)
			w.dir = filepath.Join(splitPattern[:i]...)
			break
		}
	}

	// now we split the pattern into multiple patterns according to the supported '**' syntax
	w.patterns = strings.Split(patternWithoutDir, "**")
	for i, pattern := range w.patterns {
		w.patterns[i] = strings.Trim(pattern, "/")
	}

	// remove trailing slash and add leading slash
	w.dir = "/" + strings.Trim(w.dir, "/")

	return w, nil
}

func (watchPattern *watchPattern) allowReload(fileName string, eventType int, pathType int) bool {
	if !isValidEventType(eventType) || !isValidPathType(pathType) {
		return false
	}

	return isValidPattern(fileName, watchPattern.dir, watchPattern.patterns)
}

// 0:rename,1:modify,2:create,3:destroy,4:owner,5:other,
func isValidEventType(eventType int) bool {
	return eventType <= 3
}

// 0:dir,1:file,2:hard_link,3:sym_link,4:watcher,5:other,
func isValidPathType(eventType int) bool {
	return eventType <= 2
}

func isValidPattern(fileName string, dir string, patterns []string) bool {

	// first we remove the dir from the pattern
	if !strings.HasPrefix(fileName, dir) {
		return false
	}
	fileNameWithoutDir := strings.TrimLeft(fileName, dir+"/")

	// if the pattern has size 1 we can match it directly against the filename
	if len(patterns) == 1 {
		return matchPattern(patterns[0], fileNameWithoutDir)
	}

	return matchPatterns(patterns, fileNameWithoutDir)
}

// TODO: does this need performance optimization?
func matchPatterns(patterns []string, fileName string) bool {
	partsToMatch := strings.Split(fileName, "/")
	cursor := 0

	// if there are multiple patterns due to '**' we need to match them individually
	for i, pattern := range patterns {
		patternSize := strings.Count(pattern, "/") + 1

		// if we are at the last pattern we will start matching from the end of the filename
		if i == len(patterns)-1 {
			cursor = len(partsToMatch) - patternSize
		}

		// the cursor will move through the fileName until the pattern matches
		for j := cursor; j < len(partsToMatch); j++ {
			cursor = j
			subPattern := strings.Join(partsToMatch[j:j+patternSize], "/")
			if matchPattern(pattern, subPattern) {
				cursor = j + patternSize - 1
				break
			}
			if cursor > len(partsToMatch)-patternSize-1 {

				return false
			}
		}
	}

	return true
}

func matchPattern(pattern string, fileName string) bool {
	if pattern == "" {
		return true
	}
	patternMatches, err := filepath.Match(pattern, fileName)
	if err != nil {
		logger.Error("failed to match filename", zap.String("file", fileName), zap.Error(err))
		return false
	}

	return patternMatches
}
