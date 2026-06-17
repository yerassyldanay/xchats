// Package domain holds the brain's core types. It has no external dependencies
// (Decision 10): the core is built and tested before any integration exists.
package domain

import "time"

// Role of a message author within the window transcript.
type Role string

const (
	RoleCustomer Role = "customer" // incoming
	RoleAgent    Role = "agent"    // outgoing human reply
)

// Message is the neutral form of a conversation message used in the window.
type Message struct {
	ID        string
	Role      Role
	Content   string
	CreatedAt time.Time
}
