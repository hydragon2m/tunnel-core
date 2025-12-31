package handshake

import "errors"

var (
	ErrInvalidFrameType        = errors.New("invalid frame type for auth")
	ErrAuthMustBeControlFrame   = errors.New("auth frame must be control frame")
	ErrInvalidAuthPayload       = errors.New("invalid auth payload")
	ErrNoTokenValidator         = errors.New("no token validator configured")
	ErrInvalidToken             = errors.New("invalid token")
	ErrTokenExpired             = errors.New("token expired")
	ErrUnauthorized             = errors.New("unauthorized")
)

