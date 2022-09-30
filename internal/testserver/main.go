package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dunglas/frankenphp"
)

func main() {
	if err := frankenphp.Init(); err != nil {
		panic(err)
	}
	defer frankenphp.Shutdown()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		cwd, err := os.Getwd()
		if err != nil {
			panic(err)
		}

		req := frankenphp.NewRequestWithContext(r, cwd)
		if err := frankenphp.ServeHTTP(w, req); err != nil {
			panic(err)
		}
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
