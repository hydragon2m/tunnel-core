package registry

import (
	"sync"
	"time"
)

// Tunnel đại diện cho 1 tunnel mapping domain → connection
type Tunnel struct {
	Domain      string
	Subdomain   string
	FullDomain  string // subdomain + base domain
	ConnectionID string
	AgentID     string
	CreatedAt   time.Time
	LastAccess  time.Time
	Metadata    map[string]string
}

// Registry quản lý mapping domain → tunnel → connection
type Registry struct {
	// Domain → Tunnel mapping (read-heavy)
	tunnels map[string]*Tunnel // fullDomain -> Tunnel
	tunnelsMu sync.RWMutex
	
	// ConnectionID → []Tunnel (để cleanup khi connection close)
	connTunnels map[string]map[string]*Tunnel // connectionID -> fullDomain -> Tunnel
	connTunnelsMu sync.RWMutex
	
	// Base domain config
	baseDomain string
}

// NewRegistry tạo Registry mới
func NewRegistry(baseDomain string) *Registry {
	return &Registry{
		tunnels:     make(map[string]*Tunnel),
		connTunnels: make(map[string]map[string]*Tunnel),
		baseDomain:  baseDomain,
	}
}

// RegisterTunnel đăng ký tunnel mới
func (r *Registry) RegisterTunnel(domain, subdomain, connectionID, agentID string, metadata map[string]string) (*Tunnel, error) {
	// Build full domain
	fullDomain := r.buildFullDomain(subdomain)
	
	// Validate
	if domain != "" && domain != fullDomain {
		return nil, ErrDomainMismatch
	}
	
	r.tunnelsMu.Lock()
	defer r.tunnelsMu.Unlock()
	
	// Check duplicate
	if existing, exists := r.tunnels[fullDomain]; exists {
		if existing.ConnectionID != connectionID {
			return nil, ErrDomainAlreadyRegistered
		}
		// Same connection, update metadata
		existing.Metadata = metadata
		existing.LastAccess = time.Now()
		return existing, nil
	}
	
	// Create tunnel
	tunnel := &Tunnel{
		Domain:       domain,
		Subdomain:    subdomain,
		FullDomain:   fullDomain,
		ConnectionID: connectionID,
		AgentID:      agentID,
		CreatedAt:    time.Now(),
		LastAccess:   time.Now(),
		Metadata:     metadata,
	}
	
	r.tunnels[fullDomain] = tunnel
	
	// Track by connection
	r.connTunnelsMu.Lock()
	if r.connTunnels[connectionID] == nil {
		r.connTunnels[connectionID] = make(map[string]*Tunnel)
	}
	r.connTunnels[connectionID][fullDomain] = tunnel
	r.connTunnelsMu.Unlock()
	
	return tunnel, nil
}

// GetTunnel lấy tunnel theo domain
func (r *Registry) GetTunnel(domain string) (*Tunnel, bool) {
	r.tunnelsMu.RLock()
	defer r.tunnelsMu.RUnlock()
	
	tunnel, ok := r.tunnels[domain]
	if ok {
		// Update last access (async, không block)
		go func() {
			r.tunnelsMu.Lock()
			if t, exists := r.tunnels[domain]; exists {
				t.LastAccess = time.Now()
			}
			r.tunnelsMu.Unlock()
		}()
	}
	
	return tunnel, ok
}

// UnregisterTunnel xóa tunnel
func (r *Registry) UnregisterTunnel(domain string) error {
	r.tunnelsMu.Lock()
	tunnel, exists := r.tunnels[domain]
	if exists {
		delete(r.tunnels, domain)
	}
	r.tunnelsMu.Unlock()
	
	if !exists {
		return ErrTunnelNotFound
	}
	
	// Remove from connection tracking
	r.connTunnelsMu.Lock()
	if connTunnels, exists := r.connTunnels[tunnel.ConnectionID]; exists {
		delete(connTunnels, domain)
		if len(connTunnels) == 0 {
			delete(r.connTunnels, tunnel.ConnectionID)
		}
	}
	r.connTunnelsMu.Unlock()
	
	return nil
}

// UnregisterConnectionTunnels xóa tất cả tunnels của connection
func (r *Registry) UnregisterConnectionTunnels(connectionID string) {
	r.connTunnelsMu.RLock()
	connTunnels, exists := r.connTunnels[connectionID]
	if !exists {
		r.connTunnelsMu.RUnlock()
		return
	}
	
	// Copy domains để unlock sớm
	domains := make([]string, 0, len(connTunnels))
	for domain := range connTunnels {
		domains = append(domains, domain)
	}
	r.connTunnelsMu.RUnlock()
	
	// Unregister từng tunnel
	for _, domain := range domains {
		r.UnregisterTunnel(domain)
	}
}

// ListTunnels liệt kê tất cả tunnels (for admin/debug)
func (r *Registry) ListTunnels() []*Tunnel {
	r.tunnelsMu.RLock()
	defer r.tunnelsMu.RUnlock()
	
	tunnels := make([]*Tunnel, 0, len(r.tunnels))
	for _, tunnel := range r.tunnels {
		tunnels = append(tunnels, tunnel)
	}
	
	return tunnels
}

// GetConnectionTunnels lấy tất cả tunnels của connection
func (r *Registry) GetConnectionTunnels(connectionID string) []*Tunnel {
	r.connTunnelsMu.RLock()
	defer r.connTunnelsMu.RUnlock()
	
	connTunnels, exists := r.connTunnels[connectionID]
	if !exists {
		return nil
	}
	
	tunnels := make([]*Tunnel, 0, len(connTunnels))
	for _, tunnel := range connTunnels {
		tunnels = append(tunnels, tunnel)
	}
	
	return tunnels
}

// buildFullDomain build full domain từ subdomain
func (r *Registry) buildFullDomain(subdomain string) string {
	if subdomain == "" {
		return r.baseDomain
	}
	return subdomain + "." + r.baseDomain
}

// GetBaseDomain trả về base domain
func (r *Registry) GetBaseDomain() string {
	return r.baseDomain
}

