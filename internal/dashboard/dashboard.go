package dashboard

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/hydragon2m/tunnel-core/connection"
	"github.com/hydragon2m/tunnel-core/internal/account"
	"github.com/hydragon2m/tunnel-core/internal/registry"
)

type Dashboard struct {
	connManager  *connection.Manager
	registry     *registry.Registry
	accountStore account.Store
	startTime    time.Time
}

func NewDashboard(connManager *connection.Manager, reg *registry.Registry, store account.Store) *Dashboard {
	return &Dashboard{
		connManager:  connManager,
		registry:     reg,
		accountStore: store,
		startTime:    time.Now(),
	}
}

func (d *Dashboard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Simple CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Tunnel-Token")

	if r.Method == http.MethodOptions {
		return
	}

	if strings.HasPrefix(r.URL.Path, "/api") {
		d.handleAPI(w, r)
		return
	}

	d.handleView(w, r)
}

func (d *Dashboard) handleView(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/admin" {
		conns := d.connManager.GetAllConnections()
		tunnels := d.registry.ListTunnels()

		data := struct {
			Uptime      string
			Connections int
			Streams     int
			Tunnels     int
			ConnList    []*connection.Connection
			TunnelList  []*registry.Tunnel
		}{
			Uptime:      time.Since(d.startTime).Round(time.Second).String(),
			Connections: len(conns),
			Streams:     d.getTotalStreams(),
			Tunnels:     len(tunnels),
			ConnList:    conns,
			TunnelList:  tunnels,
		}

		tmpl, _ := template.New("admin").Parse(adminHTML)
		tmpl.Execute(w, data)
		return
	}

	// Default to User Portal
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(userHTML))
}

func (d *Dashboard) getTotalStreams() int {
	conns := d.connManager.GetAllConnections()
	count := 0
	for _, c := range conns {
		count += len(c.GetAllStreams())
	}
	return count
}

func (d *Dashboard) handleAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api")

	// --- AUTH ENDPOINTS ---

	if path == "/auth/register" && r.Method == http.MethodPost {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if req.Username == "" || req.Password == "" {
			http.Error(w, "Username and password required", http.StatusBadRequest)
			return
		}

		if _, err := d.accountStore.GetByUsername(req.Username); err == nil {
			http.Error(w, "Username already exists", http.StatusConflict)
			return
		}

		acc := &account.Account{
			ID:         fmt.Sprintf("user-%d", time.Now().Unix()),
			Username:   req.Username,
			Token:      fmt.Sprintf("tok-%d", time.Now().UnixNano()),
			AdminToken: fmt.Sprintf("adm-%d", time.Now().UnixNano()),
			MaxConns:   5,
		}
		acc.SetPassword(req.Password)

		if err := d.accountStore.Save(acc); err != nil {
			http.Error(w, "Failed to save account", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": acc.ID, "token": acc.AdminToken})
		return
	}

	if path == "/auth/login" && r.Method == http.MethodPost {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		acc, err := d.accountStore.GetByUsername(req.Username)
		if err != nil || !acc.CheckPassword(req.Password) {
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{
			"id":          acc.ID,
			"token":       acc.AdminToken,
			"agent_token": acc.Token,
		})
		return
	}

	// --- PROTECTED ENDPOINTS ---
	token := r.Header.Get("X-Tunnel-Token")
	var currentAcc *account.Account
	if token != "" {
		accs, _ := d.accountStore.List()
		for _, a := range accs {
			if a.AdminToken == token {
				currentAcc = a
				break
			}
		}
		if currentAcc == nil {
			currentAcc, _ = d.accountStore.GetByToken(token)
		}
	}

	if currentAcc == nil && !strings.HasPrefix(path, "/auth/") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if path == "/user/status" && r.Method == http.MethodGet {
		conns := d.connManager.GetAllConnections()
		var myConn *connection.Connection
		for _, c := range conns {
			if c.AgentID == currentAcc.ID {
				myConn = c
				break
			}
		}

		status := "offline"
		if myConn != nil {
			status = "online"
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     status,
			"account_id": currentAcc.ID,
			"username":   currentAcc.Username,
		})
		return
	}

	if path == "/user/config" && r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"account_id":  currentAcc.ID,
			"subdomains":  currentAcc.Subdomains,
			"max_conns":   currentAcc.MaxConns,
			"mappings":    currentAcc.Mappings,
			"agent_token": currentAcc.Token,
		})
		return
	}

	if path == "/user/mappings" && r.Method == http.MethodPost {
		var req []account.Mapping
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		currentAcc.Mappings = req
		if err := d.accountStore.Save(currentAcc); err != nil {
			http.Error(w, "Failed to save mappings", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		return
	}

	if strings.HasPrefix(path, "/connections/") && r.Method == http.MethodDelete {
		if currentAcc == nil || currentAcc.ID != "default" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		connID := strings.TrimPrefix(path, "/connections/")
		if err := d.connManager.CloseConnection(connID); err == nil {
			w.WriteHeader(http.StatusNoContent)
		} else {
			http.Error(w, "Connection not found", http.StatusNotFound)
		}
		return
	}

	http.NotFound(w, r)
}
