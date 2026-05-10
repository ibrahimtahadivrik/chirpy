package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMakeJWT(t *testing.T) {
	tests := []struct {
		name           string
		userID         uuid.UUID
		signSecret     string
		validateSecret string
		expiresIn      time.Duration
		wantErr        bool
	}{
		{
			name:           "valid secret",
			userID:         uuid.New(),
			signSecret:     "secret1",
			validateSecret: "secret1",
			expiresIn:      time.Hour,
			wantErr:        false,
		}, {
			name:           "invalid secret",
			userID:         uuid.New(),
			signSecret:     "secret2",
			validateSecret: "secret3",
			expiresIn:      time.Hour,
			wantErr:        true,
		}, {
			name:           "expired token",
			userID:         uuid.New(),
			signSecret:     "secret4",
			validateSecret: "secret4",
			expiresIn:      -time.Hour,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := MakeJWT(tt.userID, tt.signSecret, tt.expiresIn)
			if err != nil {
				t.Fatal("MakeJWT (unexpected error):", err)
			}

			validID, err := ValidateJWT(token, tt.validateSecret)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateJWT() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if validID != tt.userID {
					t.Errorf("ValidateJWT() validID = %v, want %v", validID, tt.userID)
				}
			}
		})
	}

}
