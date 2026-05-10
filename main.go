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
	"github.com/ibrahimtahadivrik/chirpy/internal/auth"
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

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	User_id   uuid.UUID `json:"user_id"`
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

func (cfg *apiConfig) createChirpHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	type param struct {
		Body   string    `json:"body"`
		UserID uuid.UUID `json:"user_id"`
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

	newChirp := database.CreateChirpParams{
		newBody,
		params.UserID,
	}
	chirp, err := cfg.database.CreateChirp(r.Context(), newChirp)
	if err != nil {
		fmt.Println(err)
		data := map[string]string{"error": "Something went wrong"}
		jsonData, _ := json.Marshal(data)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(jsonData)
		return
	}

	respChirp := Chirp{
		chirp.ID,
		chirp.CreatedAt,
		chirp.UpdatedAt,
		chirp.Body,
		chirp.UserID,
	}

	jsonData, _ := json.Marshal(respChirp)
	w.WriteHeader(http.StatusCreated)
	w.Write(jsonData)

}

func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {
	type param struct {
		Password string `json:"password"`
		Email    string `json:"email"`
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

	dbparam := database.CreateUserParams{}
	hashed, err := auth.HashPassword(params.Password)
	if err != nil {
		log.Printf("error hashing password: %v", err)
		data := map[string]string{"error": "Something went wrong"}
		jsonData, _ := json.Marshal(data)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(jsonData)
		return
	}
	dbparam.HashedPassword = hashed
	dbparam.Email = params.Email
	user, err := cfg.database.CreateUser(r.Context(), dbparam)
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

func (cfg *apiConfig) loginUserHandler(w http.ResponseWriter, r *http.Request) {
	type param struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	decoder := json.NewDecoder(r.Body)
	params := param{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("error decoding body: %v", err)
		data := map[string]string{"error": "Something went wrong"}
		jsonData, _ := json.Marshal(data)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(jsonData)
		return
	}
	user, err := cfg.database.ValidateUser(r.Context(), params.Email)
	if err != nil {
		log.Printf("Error validating user: %v", err)
		data := map[string]string{"error": "Something went wrong"}
		jsonData, _ := json.Marshal(data)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(jsonData)
		return
	}
	check, err := auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if err != nil {
		log.Printf("Error validating user: %v", err)
		data := map[string]string{"error": "Something went wrong"}
		jsonData, _ := json.Marshal(data)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(jsonData)
		return
	}

	if !check {
		log.Printf("Error validating user")
		data := map[string]string{"error": "Something went wrong"}
		jsonData, _ := json.Marshal(data)
		w.WriteHeader(http.StatusUnauthorized)
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
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)
}

func (cfg *apiConfig) getAllChirpsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	dbChips, err := cfg.database.GetAllChirps(r.Context())
	if err != nil {
		log.Printf("Error getting all chirps: %v", err)
		data := map[string]string{"error": "Something went wrong"}
		jsonData, _ := json.Marshal(data)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(jsonData)
		return
	}
	chips := make([]Chirp, len(dbChips))
	for i, dbChip := range dbChips {
		chips[i] = Chirp{
			dbChip.ID,
			dbChip.CreatedAt,
			dbChip.UpdatedAt,
			dbChip.Body,
			dbChip.UserID,
		}
	}
	jsonData, _ := json.Marshal(chips)
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)
}

func (cfg *apiConfig) getOneChirpHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	chirpID, err := uuid.Parse(r.PathValue("chirpID"))
	if err != nil {
		log.Printf("Error parsing chirp ID: %v", err)
		data := map[string]string{"error": "Something went wrong"}
		jsonData, _ := json.Marshal(data)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(jsonData)
		return
	}
	dbChirp, err := cfg.database.GetOneChirp(r.Context(), chirpID)
	if err != nil {
		log.Printf("Error getting chirp: %v", err)
		data := map[string]string{"error": "Something went wrong"}
		jsonData, _ := json.Marshal(data)
		w.WriteHeader(http.StatusNotFound)
		w.Write(jsonData)
		return
	}
	respChirp := Chirp{
		dbChirp.ID,
		dbChirp.CreatedAt,
		dbChirp.UpdatedAt,
		dbChirp.Body,
		dbChirp.UserID,
	}
	jsonData, _ := json.Marshal(respChirp)
	w.WriteHeader(http.StatusOK)
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
	mux.HandleFunc("POST /api/chirps", apiCfg.createChirpHandler)
	mux.HandleFunc("GET /api/chirps", apiCfg.getAllChirpsHandler)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.getOneChirpHandler)
	mux.HandleFunc("POST /api/users", apiCfg.createUserHandler)
	mux.HandleFunc("POST /api/login", apiCfg.loginUserHandler)

	s.Addr = ":8080"
	s.Handler = mux
	err := http.ListenAndServe(s.Addr, s.Handler)
	if err != nil {
		panic(err)
	}
}
