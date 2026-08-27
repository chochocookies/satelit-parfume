// Package users holds internal staff accounts — the RBAC side of auth.
// See internal/customers for the separate shop-account side.
package users

import "time"

// User has no json tags on purpose: before Phase 9 it was only ever
// used internally (auth reads Roles/Name/Email off it to build a
// Subject, which has its own tags and never carries PasswordHash out
// over the wire). Phase 9 is the first thing that returns staff
// accounts from an API response directly — see StaffAccount below for
// the shape that actually gets serialized, so PasswordHash can never
// accidentally leak into a JSON response the way it would if a handler
// serialized *User straight.
type User struct {
	ID           string
	Name         string
	Email        string
	PasswordHash string
	Status       string
	Roles        []string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// IsActive reports whether this account is allowed to authenticate.
func (u *User) IsActive() bool {
	return u.Status == "active"
}

// StaffAccount is the public-facing shape for GET/POST/PUT
// /admin/users (Phase 9) — every field of User except PasswordHash.
type StaffAccount struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Status    string    `json:"status"`
	Roles     []string  `json:"roles"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ToStaffAccount strips PasswordHash for API responses — see the User
// doc comment above for why this exists rather than serializing *User
// directly.
func (u *User) ToStaffAccount() StaffAccount {
	return StaffAccount{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		Status:    u.Status,
		Roles:     u.Roles,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

// CreateRequest backs POST /admin/users. Roles are role *names*
// (SUPER_ADMIN, ADMIN, BRANCH_MANAGER, CASHIER, INVENTORY_STAFF, per
// migration 000002_auth's seed data) — see Handler.Create for the extra
// check on granting SUPER_ADMIN itself.
type CreateRequest struct {
	Name     string   `json:"name" binding:"required,min=2,max=150"`
	Email    string   `json:"email" binding:"required,email"`
	Password string   `json:"password" binding:"required,min=8"`
	Roles    []string `json:"roles" binding:"required,min=1"`
}

// UpdateRequest backs PUT /admin/users/:id — full replace, same
// shape/reasoning as branches.UpsertRequest and
// products.ProductUpsertRequest. There's deliberately no password field:
// resetting a staff member's own password is a different, more sensitive
// flow (proving it's really them) that this phase doesn't build — see
// the root README's "Deliberately not in Phase 9" note.
type UpdateRequest struct {
	Name   string   `json:"name" binding:"required,min=2,max=150"`
	Status string   `json:"status" binding:"required,oneof=active suspended"`
	Roles  []string `json:"roles" binding:"required,min=1"`
}
