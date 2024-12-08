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
			Pattern: "/frankenphp/threads/status",
			Handler: caddy.AdminHandlerFunc(admin.showThreadStatus),
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
		return caddy.APIError{
			HTTPStatus: http.StatusMethodNotAllowed,
			Err:        fmt.Errorf("method not allowed"),
		}
	}

	frankenphp.RestartWorkers()
	caddy.Log().Info("workers restarted from admin api")
	admin.respond(w, http.StatusOK, "workers restarted successfully\n")

	return nil
}

func (admin *FrankenPHPAdmin) showThreadStatus(w http.ResponseWriter, r *http.Request) error {
	admin.respond(w, http.StatusOK, frankenphp.ThreadDebugStatus())

	return nil
}

func (admin *FrankenPHPAdmin) addWorkerThreads(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return caddy.APIError{
			HTTPStatus: http.StatusMethodNotAllowed,
			Err:        fmt.Errorf("method not allowed"),
		}
	}

	workerPattern := r.URL.Query().Get("file")
	message := ""
	for i := 0; i < admin.getCountFromRequest(r); i++ {
		workerFilename, threadCount, err := frankenphp.AddWorkerThread(workerPattern)
		if err != nil {
			return caddy.APIError{
				HTTPStatus: http.StatusBadRequest,
				Err:        err,
			}
		}
		message = fmt.Sprintf("New thread count: %d %s\n", threadCount, workerFilename)
	}

	caddy.Log().Debug(message)
	return admin.respond(w, http.StatusOK, message)
}

func (admin *FrankenPHPAdmin) removeWorkerThreads(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return caddy.APIError{
			HTTPStatus: http.StatusMethodNotAllowed,
			Err:        fmt.Errorf("method not allowed"),
		}
	}

	workerPattern := r.URL.Query().Get("file")
	message := ""
	for i := 0; i < admin.getCountFromRequest(r); i++ {
		workerFilename, threadCount, err := frankenphp.RemoveWorkerThread(workerPattern)
		if err != nil {
			return caddy.APIError{
				HTTPStatus: http.StatusBadRequest,
				Err:        err,
			}
		}
		message = fmt.Sprintf("New thread count: %d %s\n", threadCount, workerFilename)
	}

	caddy.Log().Debug(message)
	return admin.respond(w, http.StatusOK, message)
}

func (admin *FrankenPHPAdmin) respond(w http.ResponseWriter, statusCode int, message string) error {
	w.WriteHeader(statusCode)
	_, err := w.Write([]byte(message))
	return err
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
