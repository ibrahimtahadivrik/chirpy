package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ibrahimtahadivrik/chirpy/internal/auth"
	"github.com/ibrahimtahadivrik/chirpy/internal/database"
)

func readinessHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
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

	nonBearedToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Println(err)
		data := map[string]string{"error": "Something went wrong"}
		jsonData, _ := json.Marshal(data)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write(jsonData)
		return
	}
	id, err := auth.ValidateJWT(nonBearedToken, cfg.jwtSecret)
	if err != nil {
		log.Println(err)
		data := map[string]string{"error": "Something went wrong"}
		jsonData, _ := json.Marshal(data)
		w.WriteHeader(http.StatusUnauthorized)
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
		id,
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

func (cfg *apiConfig) loginUserHandler(w http.ResponseWriter, r *http.Request) {
	type param struct {
		Password  string `json:"password"`
		Email     string `json:"email"`
		ExpiresIn int    `json:"expires_in_seconds"`
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

	if params.ExpiresIn == 0 {
		params.ExpiresIn = 3600
	} else if params.ExpiresIn > 3600 {
		params.ExpiresIn = 3600
	}
	token, err := auth.MakeJWT(user.ID, cfg.jwtSecret, time.Duration(params.ExpiresIn)*time.Second)
	if err != nil {
		log.Printf("Error creating token: %v", err)
		data := map[string]string{"error": "Something went wrong"}
		jsonData, _ := json.Marshal(data)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(jsonData)
		return
	}
	createRefreshToken := database.CreateRefreshTokenParams{
		Token:     auth.MakeRefreshToken(),
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(60 * 24 * time.Hour).UTC(),
	}

	refreshToken, err := cfg.database.CreateRefreshToken(r.Context(), createRefreshToken)
	if err != nil {
		log.Printf("Error creating refresh token: %v", err)
		data := map[string]string{"error": "Something went wrong"}
		jsonData, _ := json.Marshal(data)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(jsonData)
		return
	}
	respUser := User{
		ID:           user.ID,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		Email:        user.Email,
		Token:        token,
		RefreshToken: refreshToken.Token,
	}
	jsonData, _ := json.Marshal(respUser)
	w.WriteHeader(http.StatusOK)
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

func (cfg *apiConfig) validateRefreshToken(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		log.Printf("No authorization header found")
		jsonData, _ := json.Marshal(map[string]string{"error": "Authorization header missing"})
		w.WriteHeader(http.StatusUnauthorized)
		w.Write(jsonData)
		return
	}

	nonBearedToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("Error getting bearer token: %v", err)
		jsonData, _ := json.Marshal(map[string]string{"error": "Something went wrong"})
		w.WriteHeader(http.StatusUnauthorized)
		w.Write(jsonData)
		return
	}

	refreshTokenRow, err := cfg.database.GetUserFromRefreshToken(r.Context(), nonBearedToken)
	if err != nil {
		log.Printf("Error getting user from refresh token: %v", err)
		jsonData, _ := json.Marshal(map[string]string{"error": "Something went wrong"})
		w.WriteHeader(http.StatusUnauthorized)
		w.Write(jsonData)
		return
	}
	if refreshTokenRow.ExpiresAt.Before(time.Now().UTC()) || refreshTokenRow.RevokedAt.Valid != false {
		log.Printf("Refresh token expired")
		jsonData, _ := json.Marshal(map[string]string{"error": "Refresh token expired"})
		w.WriteHeader(http.StatusUnauthorized)
		w.Write(jsonData)
		return
	}

	newAccessToken, err := auth.MakeJWT(refreshTokenRow.UserID, cfg.jwtSecret, time.Duration(60)*time.Minute)
	if err != nil {
		log.Printf("Error creating token: %v", err)
		jsonData, _ := json.Marshal(map[string]string{"error": "Something went wrong"})
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(jsonData)
		return
	}

	data := map[string]string{"token": newAccessToken}
	jsonData, _ := json.Marshal(data)
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)
}

func (cfg *apiConfig) revokeRefreshToken(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		log.Printf("No authorization header found")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	nonBearedToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("Error getting bearer token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	errq := cfg.database.RevokeRefreshToken(r.Context(), nonBearedToken)
	if errq != nil {
		log.Printf("Error revoking refresh token: %v", errq)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
