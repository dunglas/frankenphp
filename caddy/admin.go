package caddy

import (
	"fmt"
	"github.com/caddyserver/caddy/v2"
	"github.com/dunglas/frankenphp"
	"net/http"
	"strconv"
	"strings"
)

type FrankenPHPAdmin struct{}

// if the id starts with "admin.api" the module will register AdminRoutes via module.Routes()
func (FrankenPHPAdmin) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "admin.api.frankenphp",
		New: func() caddy.Module { return new(FrankenPHPAdmin) },
	}
}

func (admin FrankenPHPAdmin) Routes() []caddy.AdminRoute {
	return []caddy.AdminRoute{
		{
			Pattern: "/frankenphp/workers/restart",
			Handler: caddy.AdminHandlerFunc(admin.restartWorkers),
		},
		{
			Pattern: "/frankenphp/threads",
			Handler: caddy.AdminHandlerFunc(admin.threads),
		},
	}
}

func (admin *FrankenPHPAdmin) restartWorkers(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return admin.error(http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
	}

	frankenphp.RestartWorkers()
	caddy.Log().Info("workers restarted from admin api")
	admin.success(w, "workers restarted successfully\n")

	return nil
}

func (admin *FrankenPHPAdmin) threads(w http.ResponseWriter, r *http.Request) error {
	if r.Method == http.MethodPut {
		return admin.changeThreads(w, r, admin.getCountFromRequest(r))
	}
	if r.Method == http.MethodDelete {
		return admin.changeThreads(w, r, -admin.getCountFromRequest(r))
	}
	if r.Method == http.MethodGet {
		return admin.success(w, frankenphp.ThreadDebugStatus())
	}

	return admin.error(http.StatusMethodNotAllowed, fmt.Errorf("method not allowed, try: GET,PUT,DELETE"))
}

func (admin *FrankenPHPAdmin) changeThreads(w http.ResponseWriter, r *http.Request, count int) error {
	if !r.URL.Query().Has("worker") {
		return admin.changeRegularThreads(w, count)
	}
	workerFilename := admin.getWorkerByPattern(r.URL.Query().Get("worker"))

	return admin.changeWorkerThreads(w, count, workerFilename)
}

func (admin *FrankenPHPAdmin) changeWorkerThreads(w http.ResponseWriter, num int, workerFilename string) error {
	method := frankenphp.AddWorkerThread
	if num < 0 {
		num = -num
		method = frankenphp.RemoveWorkerThread
	}
	message := ""
	for i := 0; i < num; i++ {
		threadCount, err := method(workerFilename)
		if err != nil {
			return admin.error(http.StatusBadRequest, err)
		}
		message = fmt.Sprintf("New thread count: %d %s\n", threadCount, workerFilename)
	}
	return admin.success(w, message)
}

func (admin *FrankenPHPAdmin) changeRegularThreads(w http.ResponseWriter, num int) error {
	method := frankenphp.AddRegularThread
	if num < 0 {
		num = -num
		method = frankenphp.RemoveRegularThread
	}
	message := ""
	for i := 0; i < num; i++ {
		threadCount, err := method()
		if err != nil {
			return admin.error(http.StatusBadRequest, err)
		}
		message = fmt.Sprintf("New thread count: %d Regular Threads\n", threadCount)
	}
	return admin.success(w, message)
}

func (admin *FrankenPHPAdmin) success(w http.ResponseWriter, message string) error {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(message))
	return err
}

func (admin *FrankenPHPAdmin) error(statusCode int, err error) error {
	return caddy.APIError{HTTPStatus: statusCode, Err: err}
}

func (admin *FrankenPHPAdmin) getCountFromRequest(r *http.Request) int {
	value := r.URL.Query().Get("count")
	if value == "" {
		return 1
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return 1
	}
	return i
}

func (admin *FrankenPHPAdmin) getWorkerByPattern(pattern string) string {
	for _, workerFilename := range frankenphp.WorkerFileNames() {
		if strings.HasSuffix(workerFilename, pattern) {
			return workerFilename
		}
	}
	return ""
}
