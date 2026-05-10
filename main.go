package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/ibrahimtahadivrik/chirpy/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type server struct {
	Addr    string
	Handler http.Handler
}
type apiConfig struct {
	database       *database.Queries
	platform       string
	fileserverHits atomic.Int32
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
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
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	body := fmt.Sprintf("<html>\n  <body>\n    <h1>Welcome, Chirpy Admin</h1>\n    <p>Chirpy has been visited %d times!</p>\n  </body>\n</html>", cfg.fileserverHits.Load())
	w.Write([]byte(body))
}

func (cfg *apiConfig) resetMetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	if cfg.platform != "dev" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	cfg.database.DeleteUsers(r.Context())
	cfg.fileserverHits.Store(0)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("metrics were reset"))
}

func (cfg *apiConfig) validateChirpHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	type param struct {
		Body string `json:"body"`
	}
	badWords := []string{"kerfuffle", "sharbert", "fornax"}

	decoder := json.NewDecoder(r.Body)
	params := param{}
	err := decoder.Decode(&params)
	if err != nil {
		data := map[string]string{"error": "Something went wrong"}
		jsonData, _ := json.Marshal(data)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(jsonData)
		return
	}
	if len(params.Body) > 140 {
		data := map[string]string{"error": "Chirp is too long"}
		jsonData, _ := json.Marshal(data)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(jsonData)
		return
	}

	words := strings.Fields(params.Body)

	for i, _ := range words {
		for j, _ := range badWords {
			if strings.ToLower(words[i]) == badWords[j] {
				words[i] = "****"
			}
		}
	}

	newBody := strings.Join(words, " ")
	data := map[string]string{"cleaned_body": newBody}
	jsonData, _ := json.Marshal(data)
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)

}

func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {
	type param struct {
		Email string `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	params := param{}
	err := decoder.Decode(&params)
	if err != nil {
		data := map[string]string{"error": "Something went wrong"}
		jsonData, _ := json.Marshal(data)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(jsonData)
		return
	}

	user, err := cfg.database.CreateUser(r.Context(), params.Email)
	if err != nil {
		log.Printf("Error creating user: %v", err)
		data := map[string]string{"error": "Something went wrong"}
		jsonData, _ := json.Marshal(data)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(jsonData)
		return
	}

	respUser := User{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	}

	jsonData, _ := json.Marshal(respUser)
	w.WriteHeader(http.StatusCreated)
	w.Write(jsonData)
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	db, _ := sql.Open("postgres", dbURL)
	dbQueries := database.New(db)

	mux := http.NewServeMux()

	apiCfg := &apiConfig{}
	apiCfg.database = dbQueries
	apiCfg.platform = platform
	s := &server{}

	mux.Handle("/app/", http.StripPrefix("/app", apiCfg.middlewareMetricsInc(http.FileServer(http.Dir(".")))))
	mux.HandleFunc("GET /api/healthz", readinessHandler)
	mux.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)
	mux.HandleFunc("POST /admin/reset", apiCfg.resetMetricsHandler)
	mux.HandleFunc("POST /api/validate_chirp", apiCfg.validateChirpHandler)
	mux.HandleFunc("POST /api/users", apiCfg.createUserHandler)

	s.Addr = ":8080"
	s.Handler = mux
	err := http.ListenAndServe(s.Addr, s.Handler)
	if err != nil {
		panic(err)
	}
}
