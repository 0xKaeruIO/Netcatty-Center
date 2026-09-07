package main

import (
	"log"
	"net/http"
	"os"

	"netcatty-center/internal/config"
	"netcatty-center/internal/httpapi"
	"netcatty-center/internal/store"
)

func main() {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	if cfg.HasBootstrapAdmin() {
		log.Printf("admin login uses --admin-user / NCC_ADMIN_USER (not stored in the database)")
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer st.Close()

	srv := httpapi.New(st, cfg)
	log.Printf("Netcatty Center listening on http://%s", cfg.Addr())
	if err := http.ListenAndServe(cfg.Addr(), srv.Engine()); err != nil {
		log.Fatal(err)
	}
}
