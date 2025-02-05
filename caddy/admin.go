package caddy

import (
	"fmt"
	"github.com/caddyserver/caddy/v2"
	"github.com/dunglas/frankenphp"
	"net/http"
)

type FrankenPHPAdmin struct{}

// if the id starts with "admin.api" the module will register AdminRoutes via module.Routes()
func (FrankenPHPAdmin) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "admin.api.frankenphp",
		New: func() caddy.Module { return new(FrankenPHPAdmin) },
	}
}

// EXPERIMENTAL: These routes are not yet stable and may change in the future.
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
	return admin.success(w, frankenphp.ThreadDebugStatus())
}

func (admin *FrankenPHPAdmin) success(w http.ResponseWriter, message string) error {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(message))
	return err
}

func (admin *FrankenPHPAdmin) error(statusCode int, err error) error {
	return caddy.APIError{HTTPStatus: statusCode, Err: err}
}
