package quota

import "errors"

var (
	ErrAgentStreamLimitExceeded  = errors.New("agent stream limit exceeded")
	ErrDomainStreamLimitExceeded = errors.New("domain stream limit exceeded")
	ErrAgentRateLimitExceeded     = errors.New("agent rate limit exceeded")
	ErrDomainRateLimitExceeded    = errors.New("domain rate limit exceeded")
	ErrGlobalStreamLimitExceeded  = errors.New("global stream limit exceeded")
	ErrGlobalConnectionLimitExceeded = errors.New("global connection limit exceeded")
)

