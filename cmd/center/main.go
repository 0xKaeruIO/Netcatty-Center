package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

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
	if !filepath.IsAbs(cfg.PublicDir) {
		if wd, err := os.Getwd(); err == nil {
			cfg.PublicDir = filepath.Join(wd, cfg.PublicDir)
		}
	}
	if !httpapi.PublicDirExists(cfg.PublicDir) {
		log.Fatalf("public UI not found at %s", cfg.PublicDir)
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
