// Package categories holds product categories — simple reference data
// (name + slug), auto-created on demand during product import (see
// internal/products) rather than requiring a separate admin step before
// the first import can run.
package categories

type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}
