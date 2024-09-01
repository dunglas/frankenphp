package frankenphp

import (
	"go.uber.org/zap"
	"path/filepath"
	"strings"
)

type watchOpt struct {
	pattern    string
	dirName     string
	isRecursive bool
}

func fileNameToWatchOption(fileName string) (watchOpt, error) {
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