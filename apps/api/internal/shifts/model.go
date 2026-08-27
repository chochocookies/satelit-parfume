// Package shifts tracks cashier till sessions for Phase 10's POS — when
// a cashier opened their drawer and with how much cash, and (once
// closed) how much was actually counted against what the system
// expected based on cash sales recorded during that window. See
// migration 000008_pos for the one-open-shift-per-user constraint this
// package leans on at the database level rather than re-implementing in
// Go.
package shifts

import (
	"errors"
	"time"
)

var (
	ErrNotFound      = errors.New("shift not found")
	ErrAlreadyOpen   = errors.New("you already have an open shift — close it before opening another")
	ErrAlreadyClosed = errors.New("this shift is already closed")
)

type Shift struct {
	ID              string     `json:"id"`
	BranchID        string     `json:"branch_id"`
	UserID          string     `json:"user_id"`
	OpeningBalance  int64      `json:"opening_balance"`
	ClosingBalance  *int64     `json:"closing_balance,omitempty"`
	ExpectedBalance *int64     `json:"expected_balance,omitempty"`
	Discrepancy     *int64     `json:"discrepancy,omitempty"`
	Status          string     `json:"status"`
	Notes           string     `json:"notes,omitempty"`
	OpenedAt        time.Time  `json:"opened_at"`
	ClosedAt        *time.Time `json:"closed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// OpenRequest's OpeningBalance deliberately has no `required` binding
// tag alongside `min=0` — Gin's validator treats a numeric zero value as
// "missing" under `required`, which would wrongly reject a cashier
// legitimately starting from an empty till.
type OpenRequest struct {
	OpeningBalance int64  `json:"opening_balance" binding:"min=0"`
	Notes          string `json:"notes"`
}

type CloseRequest struct {
	ClosingBalance int64  `json:"closing_balance" binding:"min=0"`
	Notes          string `json:"notes"`
}
