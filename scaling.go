package frankenphp

import (
	"errors"
	"fmt"
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

func AddWorkerThread(workerFileName string) (int, error) {
	worker, ok := workers[workerFileName]
	if !ok {
		return 0, errors.New("worker not found")
	}
	thread := getInactivePHPThread()
	if thread == nil {
		count := worker.countThreads()
		return count, fmt.Errorf("max amount of threads reached: %d", count)
	}
	convertToWorkerThread(thread, worker)
	return worker.countThreads(), nil
}

func RemoveWorkerThread(workerFileName string) (int, error) {
	worker, ok := workers[workerFileName]
	if !ok {
		return 0, errors.New("worker not found")
	}

	worker.threadMutex.RLock()
	if len(worker.threads) <= 1 {
		worker.threadMutex.RUnlock()
		return 1, errors.New("cannot remove last thread")
	}
	thread := worker.threads[len(worker.threads)-1]
	worker.threadMutex.RUnlock()
	convertToInactiveThread(thread)

	return worker.countThreads(), nil
}
