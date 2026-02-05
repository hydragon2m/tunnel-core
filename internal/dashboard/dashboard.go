package dashboard

import (
	"html/template"
	"net/http"
	"time"

	"github.com/hydragon2m/tunnel-core/connection"
	"github.com/hydragon2m/tunnel-core/internal/registry"
)

// Dashboard handles the monitoring UI
type Dashboard struct {
	connManager *connection.Manager
	registry    *registry.Registry
	startTime   time.Time
}

// NewDashboard creates a new dashboard handler
func NewDashboard(cm *connection.Manager, reg *registry.Registry) *Dashboard {
	return &Dashboard{
		connManager: cm,
		registry:    reg,
		startTime:   time.Now(),
	}
}

// ServeHTTP implements http.Handler
func (d *Dashboard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	conns := d.connManager.GetAllConnections()
	tunnels := d.registry.ListTunnels()

	data := struct {
		StartTime   string
		Uptime      string
		Connections int
		Streams     int
		Tunnels     int
		ConnList    []*connection.Connection
		TunnelList  []*registry.Tunnel
	}{
		StartTime:   d.startTime.Format(time.RFC3339),
		Uptime:      time.Since(d.startTime).Round(time.Second).String(),
		Connections: len(conns),
		Tunnels:     len(tunnels),
		ConnList:    conns,
		TunnelList:  tunnels,
	}

	// Calculate total streams
	for _, c := range conns {
		data.Streams += len(c.GetAllStreams())
	}

	tmpl := template.Must(template.New("dashboard").Parse(dashboardHTML))
	tmpl.Execute(w, data)
}

// Helper to get all streams from a connection (I'll need to add this to Connection struct)
// Actually, I can just use a helper function here for now or add it to Connection.

const dashboardHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Go-Tunnel Dashboard</title>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;600;700&family=JetBrains+Mono&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg-color: #0f172a;
            --card-bg: rgba(30, 41, 59, 0.7);
            --accent-color: #38bdf8;
            --text-primary: #f1f5f9;
            --text-secondary: #94a3b8;
            --border-color: rgba(255, 255, 255, 0.1);
            --success-color: #10b981;
        }

        body {
            font-family: 'Inter', sans-serif;
            background-color: var(--bg-color);
            color: var(--text-primary);
            margin: 0;
            padding: 20px;
            line-height: 1.5;
        }

        .container {
            max-width: 1200px;
            margin: 0 auto;
        }

        header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 40px;
            padding-bottom: 20px;
            border-bottom: 1px solid var(--border-color);
        }

        h1 {
            margin: 0;
            font-size: 1.5rem;
            font-weight: 700;
            background: linear-gradient(to right, #38bdf8, #818cf8);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }

        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            margin-bottom: 40px;
        }

        .stat-card {
            background: var(--card-bg);
            backdrop-filter: blur(10px);
            padding: 24px;
            border-radius: 16px;
            border: 1px solid var(--border-color);
            transition: transform 0.2s;
        }

        .stat-card:hover {
            transform: translateY(-4px);
        }

        .stat-label {
            display: block;
            color: var(--text-secondary);
            font-size: 0.875rem;
            margin-bottom: 8px;
        }

        .stat-value {
            font-size: 2rem;
            font-weight: 700;
            color: var(--accent-color);
        }

        .section-title {
            font-size: 1.25rem;
            margin-bottom: 20px;
            color: var(--text-primary);
        }

        .table-container {
            background: var(--card-bg);
            border-radius: 16px;
            border: 1px solid var(--border-color);
            overflow: hidden;
            margin-bottom: 40px;
        }

        table {
            width: 100%;
            border-collapse: collapse;
            text-align: left;
        }

        th {
            padding: 16px;
            background: rgba(255, 255, 255, 0.05);
            font-size: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            color: var(--text-secondary);
        }

        td {
            padding: 16px;
            border-bottom: 1px solid var(--border-color);
            font-size: 0.875rem;
        }

        tr:last-child td {
            border-bottom: none;
        }

        .mono {
            font-family: 'JetBrains Mono', monospace;
        }

        .badge {
            display: inline-block;
            padding: 2px 8px;
            border-radius: 9999px;
            font-size: 0.75rem;
            font-weight: 600;
            background: rgba(16, 185, 129, 0.1);
            color: var(--success-color);
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>Go-Tunnel Dashboard</h1>
            <div style="font-size: 0.875rem; color: var(--text-secondary)">
                Uptime: <span class="mono">{{.Uptime}}</span>
            </div>
        </header>

        <div class="stats-grid">
            <div class="stat-card">
                <span class="stat-label">Active Connections</span>
                <span class="stat-value">{{.Connections}}</span>
            </div>
            <div class="stat-card">
                <span class="stat-label">Active Streams</span>
                <span class="stat-value">{{.Streams}}</span>
            </div>
            <div class="stat-card">
                <span class="stat-label">Registered Tunnels</span>
                <span class="stat-value">{{.Tunnels}}</span>
            </div>
        </div>

        <h2 class="section-title">Active Connections</h2>
        <div class="table-container">
            <table>
                <thead>
                    <tr>
                        <th>Connection ID</th>
                        <th>Agent ID</th>
                        <th>Account ID</th>
                        <th>Connected At</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .ConnList}}
                    <tr>
                        <td class="mono">{{.ID}}</td>
                        <td>{{.AgentID}}</td>
                        <td>{{.AccountID}}</td>
                        <td>{{.CreatedAt.Format "2006-01-02 15:04:05"}}</td>
                    </tr>
                    {{else}}
                    <tr>
                        <td colspan="4" style="text-align: center; color: var(--text-secondary)">No active connections</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>

        <h2 class="section-title">Registered Tunnels</h2>
        <div class="table-container">
            <table>
                <thead>
                    <tr>
                        <th>Domain</th>
                        <th>Connection ID</th>
                        <th>Created At</th>
                        <th>Last Access</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .TunnelList}}
                    <tr>
                        <td class="mono" style="color: var(--accent-color)">{{.FullDomain}}</td>
                        <td class="mono">{{.ConnectionID}}</td>
                        <td>{{.CreatedAt.Format "15:04:05"}}</td>
                        <td>{{if not .LastAccess.IsZero}}{{.LastAccess.Format "15:04:05"}}{{else}}Never{{end}}</td>
                    </tr>
                    {{else}}
                    <tr>
                        <td colspan="4" style="text-align: center; color: var(--text-secondary)">No registered tunnels</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
    </div>
</body>
</html>
`
