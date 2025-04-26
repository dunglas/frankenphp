package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/dunglas/frankenphp"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := frankenphp.Init(frankenphp.WithLogger(logger)); err != nil {
		panic(err)
	}
	defer frankenphp.Shutdown()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		req, err := frankenphp.NewRequestWithContext(r)
		if err != nil {
			panic(err)
		}

		if err := frankenphp.ServeHTTP(w, req); err != nil {
			panic(err)
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.LogAttrs(nil, slog.LevelError, "server error", slog.Any("error", http.ListenAndServe(":"+port, nil)))
	os.Exit(1)
}
