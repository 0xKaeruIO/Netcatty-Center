package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"

	"netcatty-center/internal/config"
	"netcatty-center/internal/httpapi"
	"netcatty-center/internal/store"
	"netcatty-center/internal/tlscert"
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

	handler := httpapi.New(st, cfg).Engine()
	server := &http.Server{
		Addr:    cfg.Addr(),
		Handler: handler,
	}
	if cfg.HTTPS {
		cert, err := tlscert.Generate(cfg.Host)
		if err != nil {
			log.Fatalf("generate self-signed certificate: %v", err)
		}
		server.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
		log.Printf("Netcatty Center listening on %s (in-memory self-signed TLS)", cfg.PublicURL())
		if err := server.ListenAndServeTLS("", ""); err != nil {
			log.Fatal(err)
		}
		return
	}
	log.Printf("Netcatty Center listening on %s", cfg.PublicURL())
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
