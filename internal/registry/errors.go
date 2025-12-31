package registry

import "errors"

var (
	ErrDomainMismatch         = errors.New("domain mismatch")
	ErrDomainAlreadyRegistered = errors.New("domain already registered")
	ErrTunnelNotFound         = errors.New("tunnel not found")
)

