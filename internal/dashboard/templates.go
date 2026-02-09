package dashboard

const adminHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Go-Tunnel [Admin Portal]</title>
    <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;600;700&family=JetBrains+Mono&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg-color: #020617;
            --glass-bg: rgba(30, 41, 59, 0.4);
            --glass-border: rgba(255, 255, 255, 0.08);
            --accent-primary: #38bdf8;
            --accent-secondary: #818cf8;
            --text-main: #f8fafc;
            --text-dim: #94a3b8;
            --success: #10b981;
            --danger: #ef4444;
            --warning: #f59e0b;
        }

        * { box-sizing: border-box; }
        body {
            font-family: 'Outfit', sans-serif;
            background-color: var(--bg-color);
            background-image: radial-gradient(circle at 50% 0%, rgba(56, 189, 248, 0.05) 0%, transparent 50%);
            color: var(--text-main);
            margin: 0;
            padding: 0;
            overflow-x: hidden;
            min-height: 100vh;
        }

        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 40px 20px;
        }

        header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 40px;
        }

        .brand {
            display: flex;
            align-items: center;
            gap: 12px;
        }

        .logo-box {
            width: 40px;
            height: 40px;
            background: linear-gradient(135deg, var(--accent-primary), var(--accent-secondary));
            border-radius: 10px;
            display: flex;
            align-items: center;
            justify-content: center;
            font-weight: 700;
            color: white;
            box-shadow: 0 0 20px rgba(56, 189, 248, 0.3);
        }

        h1 {
            font-size: 1.5rem;
            margin: 0;
            background: linear-gradient(to right, #fff, #94a3b8);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }

        .badge-admin {
            background: rgba(129, 140, 248, 0.1);
            color: var(--accent-secondary);
            border: 1px solid rgba(129, 140, 248, 0.2);
            padding: 4px 12px;
            border-radius: 6px;
            font-size: 0.75rem;
            font-weight: 600;
            text-transform: uppercase;
        }

        /* Stats Grid */
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
            gap: 24px;
            margin-bottom: 48px;
        }

        .card {
            background: var(--glass-bg);
            backdrop-filter: blur(12px);
            border: 1px solid var(--glass-border);
            border-radius: 20px;
            padding: 24px;
            transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
        }

        .card:hover {
            transform: translateY(-5px);
            border-color: rgba(56, 189, 248, 0.3);
            background: rgba(30, 41, 59, 0.6);
        }

        .stat-label {
            color: var(--text-dim);
            font-size: 0.875rem;
            margin-bottom: 8px;
            display: block;
        }

        .stat-value {
            font-size: 2.5rem;
            font-weight: 700;
            letter-spacing: -0.02em;
        }

        .accent-text { color: var(--accent-primary); }

        /* Tables */
        .section-header {
            display: flex;
            align-items: center;
            gap: 12px;
            margin-bottom: 24px;
        }

        .section-title {
            font-size: 1.25rem;
            font-weight: 600;
            margin: 0;
        }

        .table-wrap {
            background: var(--glass-bg);
            backdrop-filter: blur(12px);
            border: 1px solid var(--glass-border);
            border-radius: 24px;
            padding: 8px;
            overflow-x: auto;
        }

        table {
            width: 100%;
            border-collapse: separate;
            border-spacing: 0;
            text-align: left;
        }

        th {
            padding: 20px 24px;
            color: var(--text-dim);
            font-weight: 600;
            font-size: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            border-bottom: 1px solid var(--glass-border);
        }

        td {
            padding: 24px;
            border-bottom: 1px solid var(--glass-border);
            font-size: 0.9375rem;
        }

        tr:last-child td { border-bottom: none; }

        tr:hover td {
            background: rgba(255, 255, 255, 0.02);
        }

        .mono { font-family: 'JetBrains Mono', monospace; font-size: 0.85rem; }

        .status-dot {
            display: inline-block;
            width: 8px;
            height: 8px;
            border-radius: 50%;
            margin-right: 8px;
        }

        .online { background-color: var(--success); box-shadow: 0 0 10px var(--success); }

        .btn-kill {
            background: rgba(239, 68, 68, 0.1);
            color: var(--danger);
            border: 1px solid rgba(239, 68, 68, 0.2);
            padding: 6px 14px;
            border-radius: 8px;
            font-size: 0.75rem;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.2s;
        }

        .btn-kill:hover {
            background: var(--danger);
            color: white;
        }

        /* Responsive */
        @media (max-width: 768px) {
            .stats-grid { grid-template-columns: 1fr; }
            header { flex-direction: column; gap: 20px; align-items: flex-start; }
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <div class="brand">
                <div class="logo-box">G</div>
                <div>
                    <h1>Go-Tunnel</h1>
                    <span class="badge-admin">Infrastructure Control Agent Management</span>
                </div>
            </div>
            <div style="text-align: right">
                <div class="stat-label">Server Uptime</div>
                <div class="mono" style="font-size: 1.1rem">{{.Uptime}}</div>
            </div>
        </header>

        <div class="stats-grid">
            <div class="card">
                <span class="stat-label">Active Connections</span>
                <div class="stat-value accent-text">{{.Connections}}</div>
            </div>
            <div class="card">
                <span class="stat-label">Current Streams</span>
                <div class="stat-value">{{.Streams}}</div>
            </div>
            <div class="card">
                <span class="stat-label">Registered Tunnels</span>
                <div class="stat-value">{{.Tunnels}}</div>
            </div>
            <div class="card">
                <span class="stat-label">Global Quota</span>
                <div class="stat-value accent-text">98<span style="font-size: 1rem; color: var(--text-dim)">% Available</span></div>
            </div>
        </div>

        <div class="section-header">
            <div style="width: 8px; height: 24px; background: var(--accent-primary); border-radius: 4px;"></div>
            <h2 class="section-title">Active Connections</h2>
        </div>
        
        <div class="table-wrap">
            <table>
                <thead>
                    <tr>
                        <th>Agent Details</th>
                        <th>Account</th>
                        <th>Connection ID</th>
                        <th>Connected At</th>
                        <th>Operations</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .ConnList}}
                    <tr>
                        <td>
                            <div style="font-weight: 600">{{.AgentID}}</div>
                            <div style="font-size: 0.75rem; color: var(--text-dim)">{{.ID}}</div>
                        </td>
                        <td>
                            <span class="badge-admin" style="color: var(--accent-primary)">{{.AccountID}}</span>
                        </td>
                        <td class="mono">{{.ID}}</td>
                        <td>
                            <div style="color: var(--text-dim)">{{.CreatedAt.Format "15:04:05"}}</div>
                            <div style="font-size: 0.75rem">{{.CreatedAt.Format "Jan 02, 2006"}}</div>
                        </td>
                        <td>
                            <button class="btn-kill" onclick="confirmKill('{{.ID}}')">Terminate</button>
                        </td>
                    </tr>
                    {{else}}
                    <tr>
                        <td colspan="5" style="text-align: center; padding: 48px; color: var(--text-dim)">
                            <div style="font-size: 3rem; opacity: 0.2; margin-bottom: 12px">📡</div>
                            No active connections found
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>

        <div class="section-header" style="margin-top: 60px">
            <div style="width: 8px; height: 24px; background: var(--accent-secondary); border-radius: 4px;"></div>
            <h2 class="section-title">Network Registry</h2>
        </div>

        <div class="table-wrap">
            <table>
                <thead>
                    <tr>
                        <th>Tunnel Domain</th>
                        <th>Target Connection</th>
                        <th>Status</th>
                        <th>Last Access</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .TunnelList}}
                    <tr>
                        <td>
                            <div class="mono" style="color: var(--accent-primary); font-weight: 600; font-size: 1rem">{{.FullDomain}}</div>
                        </td>
                        <td class="mono">{{.ConnectionID}}</td>
                        <td>
                            <span class="status-dot online"></span>
                            <span style="font-size: 0.8rem; font-weight: 600; text-transform: uppercase">Active</span>
                        </td>
                        <td>
                            <div class="mono">{{if not .LastAccess.IsZero}}{{.LastAccess.Format "15:04:05"}}{{else}}Pending{{end}}</div>
                        </td>
                    </tr>
                    {{else}}
                    <tr>
                        <td colspan="4" style="text-align: center; padding: 48px; color: var(--text-dim)">
                            No tunnels registered
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
    </div>

    <script>
        function confirmKill(connId) {
            if (confirm('Are you sure you want to terminate connection ' + connId + '?')) {
                fetch('/api/connections/' + connId, { method: 'DELETE' })
                    .then(res => {
                        if (res.ok) location.reload();
                        else alert('Failed to terminate connection');
                    });
            }
        }
    </script>
</body>
</html>
`

const userHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Go-Tunnel [User Portal]</title>
    <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;600;700&family=JetBrains+Mono&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg-color: #0c0a09;
            --glass-bg: rgba(28, 25, 23, 0.7);
            --glass-border: rgba(255, 255, 255, 0.05);
            --accent: #f97316;
            --accent-glow: rgba(249, 115, 22, 0.3);
            --text-main: #fafaf9;
            --text-dim: #a8a29e;
            --success: #22c55e;
            --error: #ef4444;
            --offline: #78716c;
        }

        body {
            font-family: 'Outfit', sans-serif;
            background-color: var(--bg-color);
            background-image: 
                radial-gradient(at 0% 0%, rgba(249, 115, 22, 0.05) 0px, transparent 50%),
                radial-gradient(at 100% 100%, rgba(124, 58, 237, 0.05) 0px, transparent 50%);
            color: var(--text-main);
            margin: 0;
            padding: 0;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }

        .container {
            width: 100%;
            max-width: 600px;
            margin: 20px;
            display: none; /* Hidden by default */
        }

        .active { display: block; }

        .card {
            background: var(--glass-bg);
            backdrop-filter: blur(20px);
            border: 1px solid var(--glass-border);
            border-radius: 32px;
            padding: 40px;
            box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
        }

        .header { text-align: center; margin-bottom: 32px; }
        .logo {
            width: 64px; height: 64px;
            background: linear-gradient(135deg, #f97316, #ea580c);
            border-radius: 18px;
            margin: 0 auto 20px;
            display: flex; align-items: center; justify-content: center;
            font-size: 24px; font-weight: 800; color: white;
            box-shadow: 0 10px 20px rgba(249, 115, 22, 0.2);
        }

        h1 { font-size: 1.5rem; margin: 0; font-weight: 700; }
        .subtitle { color: var(--text-dim); font-size: 0.875rem; margin-top: 4px; }

        /* Forms */
        .form-group { margin-bottom: 20px; }
        label { display: block; margin-bottom: 8px; color: var(--text-dim); font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em; }
        input {
            width: 100%;
            background: rgba(255, 255, 255, 0.03);
            border: 1px solid var(--glass-border);
            border-radius: 12px;
            padding: 12px 16px;
            color: white;
            font-family: inherit;
            box-sizing: border-box;
            outline: none;
            transition: border-color 0.2s;
        }
        input:focus { border-color: var(--accent); }

        .btn {
            width: 100%;
            background: white;
            color: black;
            border: none;
            padding: 14px;
            border-radius: 14px;
            font-weight: 700;
            font-size: 0.9375rem;
            cursor: pointer;
            transition: all 0.2s;
            text-align: center;
            text-decoration: none;
            display: block;
        }
        .btn:hover { transform: translateY(-2px); box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.1); }
        .btn-secondary { background: rgba(255, 255, 255, 0.05); color: white; border: 1px solid var(--glass-border); margin-top: 12px; }
        .btn-accent { background: var(--accent); color: white; }

        /* Dashboard specific */
        .status-pill {
            display: inline-flex; align-items: center; gap: 8px;
            background: rgba(255, 255, 255, 0.03); border: 1px solid var(--glass-border);
            padding: 6px 16px; border-radius: 999px; font-size: 0.8125rem; font-weight: 600;
            margin: 0 auto 24px;
        }
        .dot { width: 8px; height: 8px; border-radius: 50%; }
        .dot-online { background-color: var(--success); box-shadow: 0 0 10px var(--success); }
        .dot-offline { background-color: var(--offline); }

        .mapping-item {
            background: rgba(255, 255, 255, 0.02);
            border: 1px solid var(--glass-border);
            border-radius: 16px;
            padding: 16px;
            margin-bottom: 12px;
            display: flex;
            align-items: center;
            gap: 12px;
        }
        .mapping-item input { flex: 1; margin: 0; }
        .mapping-item .remove-btn { color: var(--error); cursor: pointer; padding: 4px; font-weight: bold; }

        .token-card {
            background: rgba(249, 115, 22, 0.05);
            border: 1px solid rgba(249, 115, 22, 0.1);
            padding: 16px;
            border-radius: 16px;
            margin-top: 24px;
        }
        .token-value {
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.8125rem;
            color: var(--accent);
            word-break: break-all;
            margin-top: 8px;
            display: block;
        }
    </style>
</head>
<body>
    <!-- LOGIN SCREEN -->
    <div id="login-screen" class="container active">
        <div class="card">
            <div class="header">
                <div class="logo">G</div>
                <h1>Welcome Back</h1>
                <p class="subtitle">Enter your credentials to manage tunnels</p>
            </div>
            <div class="form-group">
                <label>Username</label>
                <input type="text" id="login-username" placeholder="your_username">
            </div>
            <div class="form-group">
                <label>Password</label>
                <input type="password" id="login-password" placeholder="••••••••">
            </div>
            <button class="btn btn-accent" onclick="login()">Login</button>
            <button class="btn btn-secondary" onclick="showScreen('register')">Create Account</button>
            <p id="login-error" style="color:var(--error); font-size:0.8rem; text-align:center; margin-top:10px;"></p>
        </div>
    </div>

    <!-- REGISTER SCREEN -->
    <div id="register-screen" class="container">
        <div class="card">
            <div class="header">
                <div class="logo">G</div>
                <h1>Create Account</h1>
                <p class="subtitle">Join Go-Tunnel and start proxying</p>
            </div>
            <div class="form-group">
                <label>Username</label>
                <input type="text" id="reg-username" placeholder="choose_username">
            </div>
            <div class="form-group">
                <label>Password</label>
                <input type="password" id="reg-password" placeholder="••••••••">
            </div>
            <button class="btn btn-accent" onclick="register()">Register</button>
            <button class="btn btn-secondary" onclick="showScreen('login')">Already have an account?</button>
            <p id="reg-error" style="color:var(--error); font-size:0.8rem; text-align:center; margin-top:10px;"></p>
        </div>
    </div>

    <!-- DASHBOARD SCREEN -->
    <div id="dashboard-screen" class="container">
        <div class="card">
            <div class="header">
                <div style="display:flex; justify-content:space-between; align-items:center;">
                    <span id="user-display" style="font-weight:600; color:var(--text-dim)">User</span>
                    <a href="#" onclick="logout()" style="color:var(--error); font-size:0.8rem">Sign Out</a>
                </div>
                <h1 style="margin-top:20px">Remote Config</h1>
                <p class="subtitle">Manage your tunnel mappings</p>
            </div>

            <div style="display: flex; justify-content: center">
                <div class="status-pill">
                    <span class="dot dot-offline" id="status-dot"></span>
                    <span id="status-text">OFFLINE</span>
                </div>
            </div>

            <div id="mappings-container">
                <label>Mapping Rules (Subdomain -> Target)</label>
                <!-- Dynamically filled -->
            </div>
            <button class="btn btn-secondary" onclick="addMapping()" style="margin-bottom:12px">+ Add New Rule</button>
            <button class="btn btn-accent" onclick="saveMappings()">Apply Configuration</button>

            <div class="token-card">
                <label>Your Agent Token</label>
                <span class="token-value" id="agent-token">Loading...</span>
                <p class="subtitle" style="font-size:0.7rem">Use this token with your <code>agent.exe</code></p>
            </div>
        </div>
    </div>

    <script>
        const API_BASE = '/api'; 
        let currentToken = localStorage.getItem('tunnel_admin_token');
        let currentMappings = [];

        function showScreen(id) {
            document.querySelectorAll('.container').forEach(c => c.classList.remove('active'));
            document.getElementById(id + '-screen').classList.add('active');
        }

        async function login() {
            const u = document.getElementById('login-username').value;
            const p = document.getElementById('login-password').value;
            const err = document.getElementById('login-error');
            
            try {
                const res = await fetch(API_BASE + '/auth/login', {
                    method: 'POST',
                    body: JSON.stringify({username: u, password: p})
                });
                if (!res.ok) throw new Error('Invalid credentials');
                const data = await res.json();
                localStorage.setItem('tunnel_admin_token', data.token);
                currentToken = data.token;
                initDashboard();
            } catch (e) { err.textContent = e.message; }
        }

        async function register() {
            const u = document.getElementById('reg-username').value;
            const p = document.getElementById('reg-password').value;
            const err = document.getElementById('reg-error');
            
            try {
                const res = await fetch(API_BASE + '/auth/register', {
                    method: 'POST',
                    body: JSON.stringify({username: u, password: p})
                });
                if (!res.ok) throw new Error('Registration failed');
                alert('Account created! Please login.');
                showScreen('login');
            } catch (e) { err.textContent = e.message; }
        }

        function logout() {
            localStorage.removeItem('tunnel_admin_token');
            location.reload();
        }

        async function initDashboard() {
            showScreen('dashboard');
            fetchStatus();
            fetchConfig();
            setInterval(fetchStatus, 5000);
        }

        async function fetchStatus() {
            const res = await fetch(API_BASE + '/user/status', {
                headers: { 'X-Tunnel-Token': currentToken }
            });
            if (res.status === 401) logout();
            const data = await res.json();
            
            document.getElementById('user-display').textContent = data.username;
            const dot = document.getElementById('status-dot');
            const text = document.getElementById('status-text');
            if (data.status === 'online') {
                dot.className = 'dot dot-online';
                text.textContent = 'AGENT ONLINE';
                text.style.color = 'var(--success)';
            } else {
                dot.className = 'dot dot-offline';
                text.textContent = 'AGENT OFFLINE';
                text.style.color = 'var(--text-dim)';
            }
        }

        async function fetchConfig() {
            const res = await fetch(API_BASE + '/user/config', {
                headers: { 'X-Tunnel-Token': currentToken }
            });
            const data = await res.json();
            document.getElementById('agent-token').textContent = data.agent_token;
            currentMappings = data.mappings || [];
            renderMappings();
        }

        function renderMappings() {
            const container = document.getElementById('mappings-container');
            container.innerHTML = '<label>Mapping Rules (Subdomain -> Target)</label>';
            currentMappings.forEach((m, idx) => {
                const div = document.createElement('div');
                div.className = 'mapping-item';
                div.innerHTML = ` + "`" + `
                    <input type="text" value="${m.subdomain}" placeholder="sub" onchange="updateMapping(${idx}, 'subdomain', this.value)">
                    <span style="color:var(--text-dim)">→</span>
                    <input type="text" value="${m.local_target}" placeholder="http://localhost:3000" onchange="updateMapping(${idx}, 'local_target', this.value)">
                    <span class="remove-btn" onclick="removeMapping(${idx})">×</span>
                ` + "`" + `;
                container.appendChild(div);
            });
        }

        function addMapping() {
            currentMappings.push({subdomain: '', local_target: 'http://localhost:8080'});
            renderMappings();
        }

        function updateMapping(idx, field, val) { 
            currentMappings[idx][field] = val; 
        }

        function removeMapping(idx) { 
            currentMappings.splice(idx, 1); 
            renderMappings(); 
        }

        async function saveMappings() {
            await fetch(API_BASE + '/user/mappings', {
                method: 'POST',
                headers: { 'X-Tunnel-Token': currentToken, 'Content-Type': 'application/json' },
                body: JSON.stringify(currentMappings)
            });
            alert('Settings applied! Restart your agent to take effect.');
        }

        if (currentToken) initDashboard();
    </script>
</body>
</html>
`
