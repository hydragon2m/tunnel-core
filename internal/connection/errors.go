package connection

import "errors"

var (
	ErrMaxConnections        = errors.New("max connections reached")
	ErrConnectionExists      = errors.New("connection already exists")
	ErrConnectionNotFound    = errors.New("connection not found")
	ErrConnectionClosed      = errors.New("connection closed")
	ErrConnectionClosedByAgent = errors.New("connection closed by agent")
	
	ErrStreamExists    = errors.New("stream already exists")
	ErrStreamNotFound  = errors.New("stream not found")
	ErrStreamClosed    = errors.New("stream closed")
	
	ErrInvalidControlFrame = errors.New("invalid control frame")
	ErrInvalidStreamFrame  = errors.New("invalid stream frame")
)

