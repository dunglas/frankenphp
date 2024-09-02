package frankenphp

import (
	"go.uber.org/zap"
	"path/filepath"
	"strings"
)

type watchOpt struct {
	pattern	string
	dirName	 string
	isRecursive bool
}

func createWatchOption(fileName string) (watchOpt, error) {
	watchOpt := watchOpt{pattern: "", dirName: fileName, isRecursive: true}
	dirName, baseName := filepath.Split(watchOpt.dirName)
	if(strings.Contains(baseName, "*") || strings.Contains(baseName, ".")) {
		watchOpt.dirName = dirName
		watchOpt.pattern = baseName
		watchOpt.isRecursive = false
	}

	if(strings.Contains(fileName, "/**")) {
		watchOpt.dirName = strings.Split(fileName, "/**")[0]
		watchOpt.isRecursive = true
	}

	absName, err := filepath.Abs(watchOpt.dirName)
	if err != nil {
		logger.Error("directory could not be watched", zap.String("dir", watchOpt.dirName), zap.Error(err))
		return watchOpt, err
	}
	watchOpt.dirName = absName

	return watchOpt, nil
}

func fileMatchesPattern(fileName string, watchOpts []watchOpt) bool {
	for _, watchOpt := range watchOpts {
		if !strings.HasPrefix(fileName, watchOpt.dirName) {
			continue
		}
		if(watchOpt.isRecursive == false && filepath.Dir(fileName) != watchOpt.dirName) {
			continue
		}
		if watchOpt.pattern == "" {
			return true
		}
		baseName := filepath.Base(fileName)
		patternMatches, err := filepath.Match(watchOpt.pattern, baseName)
		if(err != nil) {
			logger.Error("failed to match filename", zap.String("file", fileName), zap.Error(err))
			continue
		}
		if(patternMatches){
			return true
		}
	}
	return false
}