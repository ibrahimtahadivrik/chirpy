package main

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/ibrahimtahadivrik/chirpy/internal/database"
)

type server struct {
	Addr    string
	Handler http.Handler
}
type apiConfig struct {
	database       *database.Queries
	platform       string
	jwtSecret      string
	fileserverHits atomic.Int32
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
	Token     string    `json:"token"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	User_id   uuid.UUID `json:"user_id"`
}
