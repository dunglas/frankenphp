package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// represents the main PHP thread
// the thread needs to keep running as long as all other threads are running
type phpMainThread struct {
	state           *threadState
	done            chan struct{}
	numThreads      int
	maxThreads      int
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
		state:           newThreadState(),
		done:            make(chan struct{}),
		numThreads:      numThreads,
		maxThreads:      numMaxThreads,
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

	// overwrite php.ini config (if necessary)

	mainThread.state.waitFor(stateReady)
	return nil
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
func go_frankenphp_main_thread_is_ready() {
	mainThread.overridePHPIni()
	mainThread.setAutomaticMaxThreads()
	if mainThread.maxThreads < mainThread.numThreads {
		mainThread.maxThreads = mainThread.numThreads
	}

	mainThread.state.set(stateReady)
	mainThread.state.waitFor(stateDone)
}

// override php.ini directives with those set in the Caddy config
// this needs to happen on each thread and before script execution
func (mainThread *phpMainThread) overridePHPIni() {
	if meinThread.phpIniOverrides == nil {
		return
	}
	for k, v := range mainThread.phpIniOverrides {
		C.frankenphp_overwrite_ini_configuraton(
			C.go_string{C.ulong(len(k)), toUnsafeChar(k)},
			C.go_string{C.ulong(len(v)), toUnsafeChar(v)},
		)
	}
}

// figure out how many threads can be started based on memory_limit from php.ini
func (mainThread *phpMainThread) setAutomaticMaxThreads() {
	if mainThread.maxThreads >= 0 {
		return
	}
	perThreadMemoryLimit := uint64(C.frankenphp_get_current_memory_limit())
	if perThreadMemoryLimit <= 0 {
		return
	}
	maxAllowedThreads := getProcessAvailableMemory() / perThreadMemoryLimit
	mainThread.maxThreads = int(maxAllowedThreads)
	logger.Info("Automatic thread limit", zap.Int("phpMemoryLimit MB", int(perThreadMemoryLimit/1024/1024)), zap.Int("maxThreads", mainThread.maxThreads))
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
