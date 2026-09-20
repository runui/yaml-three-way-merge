package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/runui/yaml-three-way-merge/web"
)

func main() {
	address := flag.String("addr", ":8080", "HTTP listen address")
	fixtures := flag.String("fixtures", "fixtures", "fixture corpus root")
	flag.Parse()

	service, err := NewService(*fixtures)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	server := &http.Server{
		Addr:              *address,
		Handler:           NewHandler(service, web.StaticFiles()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("YAML merge demo listening on %s", *address)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
