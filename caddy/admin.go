package caddy

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/dunglas/frankenphp"
	"net/http"
	"fmt"
)

type FrankenPHPAdmin struct{}

// if the ID starts with admin.api, the module will register AdminRoutes via module.Routes()
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
    		Pattern: "/frankenphp/workers/add",
    		Handler: caddy.AdminHandlerFunc(admin.addWorker),
    	},
		{
        	Pattern: "/frankenphp/workers/remove",
        	Handler: caddy.AdminHandlerFunc(admin.removeWorker),
        },
	}
}

func (admin *FrankenPHPAdmin) restartWorkers(w http.ResponseWriter, r *http.Request) error {
	caddy.Log().Info("restarting workers from admin api")
	frankenphp.RestartWorkers()
	_, _ = w.Write([]byte("workers restarted successfully\n"))

	return nil
}

// experimental
func (admin *FrankenPHPAdmin) addWorker(w http.ResponseWriter, r *http.Request) error {
	caddy.Log().Info("adding workers from admin api")
	workerPattern := r.URL.Query().Get("filename")
	workerFilename, threadCount, err := frankenphp.AddWorkerThread(workerPattern)
	if err != nil {
		return err
	}
	message := fmt.Sprintf("New thread count: %d %s\n", threadCount, workerFilename)
    _, _ = w.Write([]byte(message))

	return nil
}

// experimental
func (admin *FrankenPHPAdmin) removeWorker(w http.ResponseWriter, r *http.Request) error {
	caddy.Log().Info("removing workers from admin api")
	workerPattern := r.URL.Query().Get("filename")
	workerFilename, threadCount, err := frankenphp.RemoveWorkerThread(workerPattern)
	if err != nil {
        return err
    }
	message := fmt.Sprintf("New thread count: %d %s\n", threadCount, workerFilename)
	_, _ = w.Write([]byte(message))

	return nil
}