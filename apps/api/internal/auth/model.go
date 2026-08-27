// Package auth handles registration, login (customer and staff, kept
// separate — see internal/users vs internal/customers), refresh-token
// rotation, logout, and the RequireAuth/RequireRole middleware that
// everything else in the app protects its routes with.
package auth

import "time"

type RegisterRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=150"`
	Email    string `json:"email" binding:"required,email"`
	Phone    string `json:"phone" binding:"omitempty,max=30"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type TokenPair struct {
	AccessToken           string    `json:"access_token"`
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshToken          string    `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
}

// Subject is the "who is this" shape returned by login/register/me,
// deliberately the same for staff and customers so the frontend doesn't
// need two different response shapes to render "who's logged in".
type Subject struct {
	ID    string   `json:"id"`
	Type  string   `json:"type"` // "user" (staff) | "customer"
	Name  string   `json:"name"`
	Email string   `json:"email"`
	Roles []string `json:"roles,omitempty"`
}

type AuthResponse struct {
	Subject Subject   `json:"subject"`
	Tokens  TokenPair `json:"tokens"`
}
