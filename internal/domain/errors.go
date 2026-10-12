package domain

import "errors"

var (
	// ErrNotFound — requested entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict — uniqueness violation (e.g. channel already connected).
	ErrConflict = errors.New("conflict")
	// ErrAlreadyHandled — conversation was already taken/rejected/qualified.
	ErrAlreadyHandled = errors.New("already handled")
	// ErrLeadRejected — the conversation was rejected by the owner.
	ErrLeadRejected = errors.New("lead rejected")
	// ErrNoForwardMapping — owner replied to a message not mapped to a customer.
	ErrNoForwardMapping = errors.New("no forward mapping")
)
