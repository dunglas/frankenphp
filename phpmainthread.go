package frankenphp

// #cgo nocallback frankenphp_new_main_thread
// #cgo nocallback frankenphp_init_persistent_string
// #cgo noescape frankenphp_new_main_thread
// #cgo noescape frankenphp_init_persistent_string
// #include <php_variables.h>
// #include "frankenphp.h"
import "C"
import (
	"context"
	"github.com/dunglas/frankenphp/internal/memory"
	"github.com/dunglas/frankenphp/internal/phpheaders"
	"log/slog"
	"strings"
	"sync"
)

// represents the main PHP thread
// the thread needs to keep running as long as all other threads are running
type phpMainThread struct {
	state           *threadState
	done            chan struct{}
	numThreads      int
	maxThreads      int
	phpIni          map[string]string
	commonHeaders   map[string]*C.zend_string
	knownServerKeys map[string]*C.zend_string
	sandboxedEnv    map[string]*C.zend_string
}

var (
	phpThreads []*phpThread
	mainThread *phpMainThread
)

// initPHPThreads starts the main PHP thread,
// a fixed number of inactive PHP threads
// and reserves a fixed number of possible PHP threads
func initPHPThreads(numThreads int, numMaxThreads int, phpIni map[string]string) (*phpMainThread, error) {
	logger.Debug("Initializing PHP threads", slog.Int("numThreads", numThreads), slog.Int("numMaxThreads", numMaxThreads))
	mainThread = &phpMainThread{
		state:        newThreadState(),
		done:         make(chan struct{}),
		numThreads:   numThreads,
		maxThreads:   numMaxThreads,
		phpIni:       phpIni,
		sandboxedEnv: initializeEnv(),
	}

	// initialize the first thread
	// this needs to happen before starting the main thread
	// since some extensions access environment variables on startup
	// the threadIndex on the main thread defaults to 0 -> phpThreads[0].Pin(...)
	initialThread := newPHPThread(0)
	phpThreads = []*phpThread{initialThread}

	if err := mainThread.start(); err != nil {
		logger.Error("Failed to start main thread", "error", err)
		return nil, err
	}
	logger.Debug("Main thread started successfully")

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
			logger.Debug("Booting PHP thread", slog.Int("threadIndex", thread.threadIndex))
			thread.boot()
			logger.Debug("PHP thread booted", slog.Int("threadIndex", thread.threadIndex))
			ready.Done()
		}()
	}
	ready.Wait()
	logger.Debug("All PHP threads initialized")

	return mainThread, nil
}

func drainPHPThreads() {
	logger.Debug("Draining PHP threads")
	doneWG := sync.WaitGroup{}
	doneWG.Add(len(phpThreads))
	mainThread.state.set(stateShuttingDown)
	close(mainThread.done)
	for _, thread := range phpThreads {
		// shut down all reserved threads
		if thread.state.compareAndSwap(stateReserved, stateDone) {
			logger.Debug("Reserved PHP thread marked as done", slog.Int("threadIndex", thread.threadIndex))
			doneWG.Done()
			continue
		}
		// shut down all active threads
		go func(thread *phpThread) {
			logger.Debug("Shutting down PHP thread", slog.Int("threadIndex", thread.threadIndex))
			thread.shutdown()
			logger.Debug("PHP thread shut down", slog.Int("threadIndex", thread.threadIndex))
			doneWG.Done()
		}(thread)
	}

	doneWG.Wait()
	mainThread.state.set(stateDone)
	mainThread.state.waitFor(stateReserved)
	phpThreads = nil
	logger.Debug("All PHP threads drained")
}

//export go_wait_for_pending_threads
func go_wait_for_pending_threads() {
	logger.Debug("Waiting for pending threads")
	mainThread.state.waitFor(stateDone)
	logger.Debug("Pending threads finished")
}

func (mainThread *phpMainThread) start() error {
	logger.Debug("Calling frankenphp_new_main_thread", slog.Int("numThreads", mainThread.numThreads))
	if C.frankenphp_new_main_thread(C.int(mainThread.numThreads)) != 0 {
		return ErrMainThreadCreation
	}
	logger.Debug("frankenphp_new_main_thread returned successfully")

	mainThread.state.waitFor(stateReady)

	// cache common request headers as zend_strings (HTTP_ACCEPT, HTTP_USER_AGENT, etc.)
	mainThread.commonHeaders = make(map[string]*C.zend_string, len(phpheaders.CommonRequestHeaders))
	for key, phpKey := range phpheaders.CommonRequestHeaders {
		logger.Debug("Initializing persistent string for common header", slog.String("key", key))
		mainThread.commonHeaders[key] = C.frankenphp_init_persistent_string(C.CString(phpKey), C.size_t(len(phpKey)))
	}
	logger.Debug("Common headers initialized")

	// cache $_SERVER keys as zend_strings (SERVER_PROTOCOL, SERVER_SOFTWARE, etc.)
	mainThread.knownServerKeys = make(map[string]*C.zend_string, len(knownServerKeys))
	for _, phpKey := range knownServerKeys {
		logger.Debug("Initializing persistent string for known server key", slog.String("key", phpKey))
		mainThread.knownServerKeys[phpKey] = C.frankenphp_init_persistent_string(toUnsafeChar(phpKey), C.size_t(len(phpKey)))
	}
	logger.Debug("Known server keys initialized")

	return nil
}

func getInactivePHPThread() *phpThread {
	for _, thread := range phpThreads {
		if thread.state.is(stateInactive) {
			logger.Debug("Found inactive PHP thread", slog.Int("threadIndex", thread.threadIndex))
			return thread
		}
	}

	for _, thread := range phpThreads {
		if thread.state.compareAndSwap(stateReserved, stateBootRequested) {
			logger.Debug("Booting reserved PHP thread", slog.Int("threadIndex", thread.threadIndex))
			thread.boot()
			logger.Debug("Reserved PHP thread booted", slog.Int("threadIndex", thread.threadIndex))
			return thread
		}
	}

	logger.Debug("No inactive or reserved PHP thread found")
	return nil
}

//export go_frankenphp_main_thread_is_ready
func go_frankenphp_main_thread_is_ready() {
	logger.Debug("Main PHP thread is ready callback received")
	mainThread.setAutomaticMaxThreads()
	if mainThread.maxThreads < mainThread.numThreads {
		mainThread.maxThreads = mainThread.numThreads
	}

	mainThread.state.set(stateReady)
	mainThread.state.waitFor(stateDone)
	logger.Debug("Main PHP thread ready and waiting for done state")
}

// max_threads = auto
// setAutomaticMaxThreads estimates the amount of threads based on php.ini and system memory_limit
// If unable to get the system's memory limit, simply double num_threads
func (mainThread *phpMainThread) setAutomaticMaxThreads() {
	if mainThread.maxThreads >= 0 {
		return
	}
	perThreadMemoryLimit := int64(C.frankenphp_get_current_memory_limit())
	totalSysMemory := memory.TotalSysMemory()
	if perThreadMemoryLimit <= 0 || totalSysMemory == 0 {
		mainThread.maxThreads = mainThread.numThreads * 2
		logger.Debug("Automatic thread limit set (default)", slog.Int("maxThreads", mainThread.maxThreads))
		return
	}
	maxAllowedThreads := totalSysMemory / uint64(perThreadMemoryLimit)
	mainThread.maxThreads = int(maxAllowedThreads)

	logger.LogAttrs(context.Background(), slog.LevelDebug, "Automatic thread limit", slog.Int("perThreadMemoryLimitMB", int(perThreadMemoryLimit/1024/1024)), slog.Int("maxThreads", mainThread.maxThreads))
}

//export go_frankenphp_shutdown_main_thread
func go_frankenphp_shutdown_main_thread() {
	logger.Debug("Main PHP thread shutdown callback received")
	mainThread.state.set(stateReserved)
}

//export go_get_custom_php_ini
func go_get_custom_php_ini(disableTimeouts C.bool) *C.char {
	if mainThread.phpIni == nil {
		mainThread.phpIni = make(map[string]string)
	}

	// Timeouts are currently fundamentally broken
	// with ZTS except on Linux and FreeBSD: https://bugs.php.net/bug.php?id=79464
	// Disable timeouts if ZEND_MAX_EXECUTION_TIMERS is not supported
	if disableTimeouts {
		mainThread.phpIni["max_execution_time"] = "0"
		mainThread.phpIni["max_input_time"] = "-1"
	}

	// Pass the php.ini overrides to PHP before startup
	// TODO: if needed this would also be possible on a per-thread basis
	var overrides strings.Builder
	for k, v := range mainThread.phpIni {
		overrides.WriteString(k)
		overrides.WriteByte('=')
		overrides.WriteString(v)
		overrides.WriteByte('\n')
	}
	logger.Debug("Returning custom php.ini", slog.String("ini", overrides.String()))
	return C.CString(overrides.String())
}
