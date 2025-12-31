package handshake

import (
	"encoding/json"
	"time"

	"github.com/hydragon2m/tunnel-protocol/go/v1"
)

// Authenticator xử lý authentication handshake với agent
type Authenticator struct {
	// Token validator
	validateToken func(token string) (agentID string, err error)
	
	// Config
	authTimeout time.Duration
}

// AuthRequest là payload của FrameAuth từ agent
type AuthRequest struct {
	Token      string            `json:"token"`
	AgentID    string            `json:"agent_id,omitempty"`
	Version    string            `json:"version,omitempty"`
	Capabilities []string        `json:"capabilities,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// AuthResponse là payload của FrameAuth response từ server
type AuthResponse struct {
	Success    bool              `json:"success"`
	AgentID    string            `json:"agent_id,omitempty"`
	ServerTime int64             `json:"server_time,omitempty"`
	Config     map[string]interface{} `json:"config,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// NewAuthenticator tạo Authenticator mới
func NewAuthenticator(validateToken func(token string) (agentID string, err error), authTimeout time.Duration) *Authenticator {
	return &Authenticator{
		validateToken: validateToken,
		authTimeout:   authTimeout,
	}
}

// HandleAuth xử lý FrameAuth từ agent
// Returns: agentID, metadata, error
func (a *Authenticator) HandleAuth(frame *v1.Frame) (agentID string, metadata map[string]string, err error) {
	// Validate frame type
	if frame.Type != v1.FrameAuth {
		return "", nil, ErrInvalidFrameType
	}
	
	// Validate control frame
	if !frame.IsControlFrame() {
		return "", nil, ErrAuthMustBeControlFrame
	}
	
	// Parse auth request
	var req AuthRequest
	if err := json.Unmarshal(frame.Payload, &req); err != nil {
		return "", nil, ErrInvalidAuthPayload
	}
	
	// Validate token
	if a.validateToken == nil {
		return "", nil, ErrNoTokenValidator
	}
	
	validatedAgentID, err := a.validateToken(req.Token)
	if err != nil {
		return "", nil, err
	}
	
	// Use validated agent ID (server is source of truth)
	agentID = validatedAgentID
	
	// Build metadata
	metadata = make(map[string]string)
	if req.AgentID != "" {
		metadata["client_agent_id"] = req.AgentID
	}
	if req.Version != "" {
		metadata["client_version"] = req.Version
	}
	
	// Add capabilities to metadata
	if len(req.Capabilities) > 0 {
		capabilitiesJSON, _ := json.Marshal(req.Capabilities)
		metadata["capabilities"] = string(capabilitiesJSON)
	}
	
	// Merge additional metadata
	for k, v := range req.Metadata {
		metadata[k] = v
	}
	
	return agentID, metadata, nil
}

// CreateAuthResponse tạo FrameAuth response để gửi cho agent
func (a *Authenticator) CreateAuthResponse(success bool, agentID string, config map[string]interface{}, errMsg string) (*v1.Frame, error) {
	resp := AuthResponse{
		Success:    success,
		AgentID:    agentID,
		ServerTime: time.Now().Unix(),
		Config:     config,
	}
	
	if !success {
		resp.Error = errMsg
	}
	
	payload, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	
	return &v1.Frame{
		Version:  v1.Version,
		Type:     v1.FrameAuth,
		Flags:    v1.FlagAck, // Acknowledgment
		StreamID: v1.StreamIDControl,
		Payload:  payload,
	}, nil
}

// CreateAuthSuccessResponse tạo success response
func (a *Authenticator) CreateAuthSuccessResponse(agentID string, config map[string]interface{}) (*v1.Frame, error) {
	return a.CreateAuthResponse(true, agentID, config, "")
}

// CreateAuthErrorResponse tạo error response
func (a *Authenticator) CreateAuthErrorResponse(errMsg string) (*v1.Frame, error) {
	return a.CreateAuthResponse(false, "", nil, errMsg)
}

