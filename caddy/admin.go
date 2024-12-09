package caddy

import (
	"fmt"
	"github.com/caddyserver/caddy/v2"
	"github.com/dunglas/frankenphp"
	"net/http"
	"strconv"
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
			Handler: caddy.AdminHandlerFunc(admin.showThreadStatus),
		},
		{
			Pattern: "/frankenphp/threads/remove",
			Handler: caddy.AdminHandlerFunc(admin.removeRegularThreads),
		},
		{
			Pattern: "/frankenphp/threads/add",
			Handler: caddy.AdminHandlerFunc(admin.addRegularThreads),
		},
		{
			Pattern: "/frankenphp/workers/add",
			Handler: caddy.AdminHandlerFunc(admin.addWorkerThreads),
		},
		{
			Pattern: "/frankenphp/workers/remove",
			Handler: caddy.AdminHandlerFunc(admin.removeWorkerThreads),
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

func (admin *FrankenPHPAdmin) showThreadStatus(w http.ResponseWriter, r *http.Request) error {
	admin.success(w, frankenphp.ThreadDebugStatus())

	return nil
}

func (admin *FrankenPHPAdmin) addWorkerThreads(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return admin.error(http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
	}

	workerPattern := r.URL.Query().Get("file")
	message := ""
	for i := 0; i < admin.getCountFromRequest(r); i++ {
		workerFilename, threadCount, err := frankenphp.AddWorkerThread(workerPattern)
		if err != nil {
			return admin.error(http.StatusBadRequest, err)
		}
		message = fmt.Sprintf("New thread count: %d %s\n", threadCount, workerFilename)
	}

	caddy.Log().Debug(message)
	return admin.success(w, message)
}

func (admin *FrankenPHPAdmin) removeWorkerThreads(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return admin.error(http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
	}

	workerPattern := r.URL.Query().Get("file")
	message := ""
	for i := 0; i < admin.getCountFromRequest(r); i++ {
		workerFilename, threadCount, err := frankenphp.RemoveWorkerThread(workerPattern)
		if err != nil {
			return admin.error(http.StatusBadRequest, err)
		}
		message = fmt.Sprintf("New thread count: %d %s\n", threadCount, workerFilename)
	}

	caddy.Log().Debug(message)
	return admin.success(w, message)
}

func (admin *FrankenPHPAdmin) addRegularThreads(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return admin.error(http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
	}

	message := ""
	for i := 0; i < admin.getCountFromRequest(r); i++ {
		threadCount, err := frankenphp.AddRegularThread()
		if err != nil {
			return admin.error(http.StatusBadRequest, err)
		}
		message = fmt.Sprintf("New thread count: %d \n", threadCount)
	}

	caddy.Log().Debug(message)
	return admin.success(w, message)
}

func (admin *FrankenPHPAdmin) removeRegularThreads(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return admin.error(http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
	}

	message := ""
	for i := 0; i < admin.getCountFromRequest(r); i++ {
		threadCount, err := frankenphp.RemoveRegularThread()
		if err != nil {
			return admin.error(http.StatusBadRequest, err)
		}
		message = fmt.Sprintf("New thread count: %d \n", threadCount)
	}

	caddy.Log().Debug(message)
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
