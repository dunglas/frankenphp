package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"strconv"
	"sync"

	"go.uber.org/zap"
)

// represents the main PHP thread
// the thread needs to keep running as long as all other threads are running
type phpMainThread struct {
	state      *threadState
	done       chan struct{}
	numThreads int
	maxThreads int
	phpIniOverrides map[string]string
}

var (
	phpThreads []*phpThread
	mainThread *phpMainThread
)

// start the main PHP thread
// start a fixed number of inactive PHP threads
// reserve a fixed number of possible PHP threads
func initPHPThreads(numThreads int, numMaxThreads int, phpIniOverrides map[string]string) error {
	mainThread = &phpMainThread{
		state:      newThreadState(),
		done:       make(chan struct{}),
		numThreads: numThreads,
		maxThreads: numMaxThreads,
		phpIniOverrides: phpIniOverrides,
	}

	// initialize the first thread
	// this needs to happen before starting the main thread
	// since some extensions access environment variables on startup
	// the threadIndex on the main thread defaults to 0 -> phpThreads[0].Pin(...)
	initialThread := newPHPThread(0)
	phpThreads = []*phpThread{initialThread}

	if err := mainThread.start(); err != nil {
		return err
	}

	// initialize all other threads
	phpThreads = make([]*phpThread, mainThread.maxThreads)
	phpThreads[0] = initialThread
	for i := 1; i < mainThread.maxThreads; i++ {
		phpThreads[i] = newPHPThread(i)
	}

	// start the underlying C threads
	ready := sync.WaitGroup{}
	ready.Add(numThreads)
	for i := 0; i < numThreads; i++ {
		thread := phpThreads[i]
		go func() {
			thread.boot()
			ready.Done()
		}()
	}
	ready.Wait()

	return nil
}

func ThreadDebugStatus() string {
	statusMessage := ""
	reservedThreadCount := 0
	for _, thread := range phpThreads {
		if thread.state.is(stateReserved) {
			reservedThreadCount++
			continue
		}
		statusMessage += thread.debugStatus() + "\n"
	}
	statusMessage += fmt.Sprintf("%d additional threads can be started at runtime\n", reservedThreadCount)
	return statusMessage
}

func drainPHPThreads() {
	doneWG := sync.WaitGroup{}
	doneWG.Add(len(phpThreads))
	mainThread.state.set(stateShuttingDown)
	close(mainThread.done)
	for _, thread := range phpThreads {
		// shut down all reserved threads
		if thread.state.compareAndSwap(stateReserved, stateDone) {
			doneWG.Done()
			continue
		}
		// shut down all active threads
		go func(thread *phpThread) {
			thread.shutdown()
			doneWG.Done()
		}(thread)
	}

	doneWG.Wait()
	mainThread.state.set(stateDone)
	mainThread.state.waitFor(stateReserved)
	phpThreads = nil
}

func (mainThread *phpMainThread) start() error {
	if C.frankenphp_new_main_thread(C.int(mainThread.numThreads)) != 0 {
		return MainThreadCreationError
	}
	for k,v := range mainThread.phpIniOverrides {
		C.frankenphp_set_ini_override(C.CString(k), C.CString(v))
	}

	mainThread.state.waitFor(stateReady)
	return nil
}

func (mainThread *phpMainThread) start() error {

}

func getInactivePHPThread() *phpThread {
	thread := getPHPThreadAtState(stateInactive)
	if thread != nil {
		return thread
	}
	thread = getPHPThreadAtState(stateReserved)
	if thread == nil {
		return nil
	}
	thread.boot()
	return thread
}

func getPHPThreadAtState(state stateID) *phpThread {
	for _, thread := range phpThreads {
		if thread.state.is(state) {
			return thread
		}
	}
	return nil
}

//export go_frankenphp_main_thread_is_ready
func go_frankenphp_main_thread_is_ready(memory_limit *C.char) {
	if mainThread.maxThreads == -1 && memory_limit != nil {
		mainThread.setAutomaticThreadLimit(C.GoString(memory_limit))
	}

	if mainThread.maxThreads < mainThread.numThreads {
		mainThread.maxThreads = mainThread.numThreads
	}

	mainThread.state.set(stateReady)
	mainThread.state.waitFor(stateDone)
}

// figure out how many threads can be started based on memory_limit from php.ini
func (mainThread *phpMainThread) setAutomaticThreadLimit(phpMemoryLimit string) {
	perThreadMemoryLimit := parsePHPMemoryLimit(phpMemoryLimit)
	if perThreadMemoryLimit <= 0 {
		return
	}
	maxAllowedThreads := getProcessAvailableMemory() / perThreadMemoryLimit
	mainThread.maxThreads = int(maxAllowedThreads)
	logger.Info("Automatic thread limit", zap.String("phpMemoryLimit", phpMemoryLimit), zap.Int("maxThreads", mainThread.maxThreads))
}

// Convert the memory limit from php.ini to bytes
// The memory limit in PHP is either post-fixed with an M or G
// Without postfix it's in bytes, -1 means no limit
func parsePHPMemoryLimit(memoryLimit string) uint64 {
	multiplier := 1
	lastChar := memoryLimit[len(memoryLimit)-1]
	if lastChar == 'M' {
		multiplier = 1024 * 1024
		memoryLimit = memoryLimit[:len(memoryLimit)-1]
	} else if lastChar == 'G' {
		multiplier = 1024 * 1024 * 1024
		memoryLimit = memoryLimit[:len(memoryLimit)-1]
	}

	bytes, err := strconv.Atoi(memoryLimit)
	if err != nil {
		logger.Warn("Could not parse PHP memory limit (assuming unlimited)", zap.String("memoryLimit", memoryLimit), zap.Error(err))
		return 0
	}
	if bytes < 0 {
		return 0
	}
	return uint64(bytes * multiplier)
}

// Gets all available memory in bytes
// Should be unix compatible - TODO: verify that it is on all important platforms
// On potential Windows support this would need to be done differently
func getProcessAvailableMemory() uint64 {
	return uint64(C.sysconf(C._SC_PHYS_PAGES) * C.sysconf(C._SC_PAGE_SIZE))
}

//export go_frankenphp_shutdown_main_thread
func go_frankenphp_shutdown_main_thread() {
	mainThread.state.set(stateReserved)
}
