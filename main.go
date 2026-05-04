package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

type server struct {
	Addr    string
	Handler http.Handler
}
type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}
func readinessHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Hits: %d", cfg.fileserverHits.Load())))
}
func (cfg *apiConfig) resetMetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	cfg.fileserverHits.Store(0)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("metrics were reset"))
}

func main() {
	mux := http.NewServeMux()

	apiCfg := &apiConfig{}
	s := &server{}

	mux.Handle("/app/", http.StripPrefix("/app", apiCfg.middlewareMetricsInc(http.FileServer(http.Dir(".")))))
	mux.HandleFunc("GET /healthz", readinessHandler)
	mux.HandleFunc("GET /metrics", apiCfg.metricsHandler)
	mux.HandleFunc("POST /reset", apiCfg.resetMetricsHandler)

	s.Addr = ":8080"
	s.Handler = mux
	err := http.ListenAndServe(s.Addr, s.Handler)
	if err != nil {
		panic(err)
	}
}
