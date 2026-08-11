package entities

import "strings"

// Order is a clean entity: stdlib imports only.
type Order struct {
	ID string
}

// Normalize canonicalizes the order id.
func (o *Order) Normalize() {
	o.ID = strings.TrimSpace(o.ID)
}
