package main

import (
	"embed"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/L480/vaillant-ui/internal/catalog"
	"github.com/L480/vaillant-ui/internal/ebusd"
	"github.com/L480/vaillant-ui/internal/server"
)

//go:embed web
var webFS embed.FS

var version = "dev"

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func healthcheck(addr string) int {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 1
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	c := http.Client{Timeout: 3 * time.Second}
	r, err := c.Get("http://" + net.JoinHostPort(host, port) + "/healthz")
	if err != nil {
		return 1
	}
	r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func main() {
	addr := env("LISTEN", ":80")
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		os.Exit(healthcheck(addr))
	}
	static, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}
	if d := os.Getenv("WEB_DIR"); d != "" {
		static = os.DirFS(d)
	}
	s := &server.Server{
		Bus:          ebusd.New(env("EBUSD_ADDR", "host.docker.internal:8888")),
		Catalog:      catalog.Build(),
		ReadOnly:     os.Getenv("READONLY") == "1",
		Version:      version,
		Static:       static,
		AllowedHosts: splitList(os.Getenv("ALLOWED_HOSTS")),
	}
	log.Printf("vaillant-ui %s listening on %s (ebusd %s, readonly=%v)", version, addr, s.Bus.Addr, s.ReadOnly)
	srv := &http.Server{Addr: addr, Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second, WriteTimeout: 10 * time.Minute}
	log.Fatal(srv.ListenAndServe())
}
