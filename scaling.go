package frankenphp

import (
	"errors"
	"fmt"
)

// turn the first inactive/reserved thread into a regular thread
func AddRegularThread() (int, error) {
	thread := getInactivePHPThread()
	if thread == nil {
		return countRegularThreads(), fmt.Errorf("max amount of overall threads reached: %d", len(phpThreads))
	}
	convertToRegularThread(thread)
	return countRegularThreads(), nil
}

// remove the last regular thread
func RemoveRegularThread() (int, error) {
	regularThreadMu.RLock()
	if len(regularThreads) <= 1 {
		regularThreadMu.RUnlock()
		return 1, errors.New("cannot remove last thread")
	}
	thread := regularThreads[len(regularThreads)-1]
	regularThreadMu.RUnlock()
	thread.shutdown()
	return countRegularThreads(), nil
}

// turn the first inactive/reserved thread into a worker thread
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

// remove the last worker thread
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
	thread.shutdown()

	return worker.countThreads(), nil
}
