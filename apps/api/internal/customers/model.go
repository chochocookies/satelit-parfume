// Package customers holds shop-customer accounts — separate from internal
// staff accounts (see internal/users). A customer's authorization is
// "is this their own resource", not a role/permission lookup.
package customers

import "time"

type Customer struct {
	ID           string
	Name         string
	Email        string
	Phone        string
	PasswordHash string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (c *Customer) IsActive() bool {
	return c.Status == "active"
}
