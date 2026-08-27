// Package branches covers branch CRUD, staff assignment, and branch-scoped
// access control. Branch-specific stock lives in internal/inventory —
// kept separate since inventory is also meaningfully consumed by the
// products package (availability-by-branch), not just by branches.
package branches

import "time"

type Branch struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Code        string    `json:"code"`
	Slug        string    `json:"slug"`
	Address     string    `json:"address,omitempty"`
	City        string    `json:"city,omitempty"`
	Province    string    `json:"province,omitempty"`
	PostalCode  string    `json:"postal_code,omitempty"`
	Latitude    *float64  `json:"latitude,omitempty"`
	Longitude   *float64  `json:"longitude,omitempty"`
	Phone       string    `json:"phone,omitempty"`
	WhatsApp    string    `json:"whatsapp,omitempty"`
	OpeningTime string    `json:"opening_time,omitempty"`
	ClosingTime string    `json:"closing_time,omitempty"`
	Status      string    `json:"status"`
	DistanceKM  *float64  `json:"distance_km,omitempty"` // only set when ?lat=&lng= was given
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UpsertRequest is shared by create and update — every field is a full
// replace on update, there's no separate PATCH-style partial shape.
type UpsertRequest struct {
	Name        string   `json:"name" binding:"required,min=2,max=150"`
	Code        string   `json:"code" binding:"required,min=2,max=20"`
	Address     string   `json:"address"`
	City        string   `json:"city"`
	Province    string   `json:"province"`
	PostalCode  string   `json:"postal_code"`
	Latitude    *float64 `json:"latitude" binding:"omitempty,min=-90,max=90"`
	Longitude   *float64 `json:"longitude" binding:"omitempty,min=-180,max=180"`
	Phone       string   `json:"phone"`
	WhatsApp    string   `json:"whatsapp"`
	OpeningTime string   `json:"opening_time" binding:"omitempty,len=5"`
	ClosingTime string   `json:"closing_time" binding:"omitempty,len=5"`
}

type StaffMember struct {
	UserID string   `json:"user_id"`
	Name   string   `json:"name"`
	Email  string   `json:"email"`
	Roles  []string `json:"roles,omitempty"`
}

type AssignStaffRequest struct {
	UserID string `json:"user_id" binding:"required,uuid"`
}
