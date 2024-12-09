package frankenphp

import (
	"errors"
	"fmt"
	"strings"
)

// exposed logic for safely scaling threads

func AddRegularThread() (int, error) {
	thread := getInactivePHPThread()
	if thread == nil {
		return countRegularThreads(), fmt.Errorf("max amount of threads reached: %d", len(phpThreads))
	}
	convertToRegularThread(thread)
	return countRegularThreads(), nil
}

func RemoveRegularThread() (int, error) {
	regularThreadMu.RLock()
	if len(regularThreads) <= 1 {
		regularThreadMu.RUnlock()
		return 1, errors.New("cannot remove last thread")
	}
	thread := regularThreads[len(regularThreads)-1]
	regularThreadMu.RUnlock()
	convertToInactiveThread(thread)
	return countRegularThreads(), nil
}

func AddWorkerThread(workerFileName string) (string, int, error) {
	worker := getWorkerByFilePattern(workerFileName)
	if worker == nil {
		return "", 0, errors.New("worker not found")
	}
	thread := getInactivePHPThread()
	if thread == nil {
		return "", 0, fmt.Errorf("max amount of threads reached: %d", len(phpThreads))
	}
	convertToWorkerThread(thread, worker)
	return worker.fileName, worker.countThreads(), nil
}

func RemoveWorkerThread(workerFileName string) (string, int, error) {
	worker := getWorkerByFilePattern(workerFileName)
	if worker == nil {
		return "", 0, errors.New("worker not found")
	}

	worker.threadMutex.RLock()
	if len(worker.threads) <= 1 {
		worker.threadMutex.RUnlock()
		return worker.fileName, 0, errors.New("cannot remove last thread")
	}
	thread := worker.threads[len(worker.threads)-1]
	worker.threadMutex.RUnlock()
	convertToInactiveThread(thread)

	return worker.fileName, worker.countThreads(), nil
}

// get the first worker ending in the given pattern
func getWorkerByFilePattern(pattern string) *worker {
	for _, worker := range workers {
		if pattern == "" || strings.HasSuffix(worker.fileName, pattern) {
			return worker
		}
	}

	return nil
}
