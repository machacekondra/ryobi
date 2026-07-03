package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/ryobi-project/ryobi/pkg/grpcapi"
)

// managementServer provides an HTTP UI for managing the environment agent.
type managementServer struct {
	mu          sync.RWMutex
	cfg         *EnvConfig
	watcher     *statusWatcher
	grpcClient  grpcapi.EnvironmentServiceClient
	logger      logr.Logger
}

func newManagementServer(cfg *EnvConfig, watcher *statusWatcher, grpcClient grpcapi.EnvironmentServiceClient, logger logr.Logger) *managementServer {
	return &managementServer{
		cfg:        cfg,
		watcher:    watcher,
		grpcClient: grpcClient,
		logger:     logger.WithName("management"),
	}
}

func (m *managementServer) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", m.handleUI)
	mux.HandleFunc("/api/config", m.handleConfig)
	mux.HandleFunc("/api/capabilities", m.handleCapabilities)
	mux.HandleFunc("/api/resources", m.handleResources)
	mux.HandleFunc("/api/recipes", m.handleRecipes)

	server := &http.Server{Addr: m.cfg.Management.Address, Handler: mux}
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()

	m.logger.Info("Management UI started", "address", m.cfg.Management.Address)
	err := server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (m *managementServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		m.mu.RLock()
		defer m.mu.RUnlock()
		writeJSON(w, map[string]any{
			"name":         m.cfg.Name,
			"server":       m.cfg.Server,
			"capabilities": m.cfg.Capabilities,
			"recipes":      m.cfg.Recipes,
		})
		return
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (m *managementServer) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		m.mu.RLock()
		defer m.mu.RUnlock()
		writeJSON(w, m.cfg.Capabilities)
		return
	}

	if r.Method == http.MethodPut {
		var caps CapabilitiesCfg
		if err := json.NewDecoder(r.Body).Decode(&caps); err != nil {
			http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
			return
		}

		m.mu.Lock()
		m.cfg.Capabilities = caps
		m.mu.Unlock()

		// Re-register with updated capabilities
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := registerEnvironment(ctx, m.grpcClient, m.cfg, m.logger); err != nil {
				m.logger.Error(err, "Failed to re-register after capability update")
			} else {
				m.logger.Info("Re-registered with updated capabilities")
			}
		}()

		writeJSON(w, map[string]string{"status": "updated"})
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (m *managementServer) handleResources(w http.ResponseWriter, r *http.Request) {
	m.watcher.mu.Lock()
	resources := make([]map[string]any, 0, len(m.watcher.resources))
	for _, wr := range m.watcher.resources {
		resources = append(resources, map[string]any{
			"resourceId":     wr.ResourceID,
			"resourceName":   wr.ResourceName,
			"resourceType":   wr.ResourceType,
			"deploymentName": wr.DeploymentName,
			"namespace":      wr.Namespace,
			"lastHealth":     wr.lastState,
		})
	}
	m.watcher.mu.Unlock()
	writeJSON(w, resources)
}

func (m *managementServer) handleRecipes(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	writeJSON(w, m.cfg.Recipes)
}

func (m *managementServer) handleUI(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	envName := m.cfg.Name
	addr := m.cfg.Management.Address
	m.mu.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := uiTemplate
	html = strings.ReplaceAll(html, "{{ENV_NAME}}", envName)
	html = strings.ReplaceAll(html, "{{ADDR}}", addr)
	w.Write([]byte(html))
}

const uiTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Ryobi Environment: {{ENV_NAME}}</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#111827;color:#f3f4f6;padding:2rem}
h1{font-size:1.5rem;margin-bottom:.5rem;color:#60a5fa}
h2{font-size:1.1rem;margin:1.5rem 0 .75rem;color:#93c5fd}
.badge{display:inline-block;padding:.15rem .5rem;border-radius:4px;font-size:.75rem;font-weight:600}
.badge-green{background:#065f46;color:#6ee7b7}
.badge-yellow{background:#78350f;color:#fcd34d}
.badge-red{background:#7f1d1d;color:#fca5a5}
.badge-gray{background:#374151;color:#9ca3af}
.subtitle{color:#9ca3af;margin-bottom:1.5rem}
.card{background:#1f2937;border:1px solid #374151;border-radius:8px;padding:1.25rem;margin-bottom:1rem}
table{width:100%;border-collapse:collapse;font-size:.875rem}
th,td{text-align:left;padding:.5rem .75rem;border-bottom:1px solid #374151}
th{color:#9ca3af;font-weight:600}
.form-row{display:flex;gap:.75rem;margin-bottom:.75rem;align-items:center}
.form-row label{min-width:120px;color:#9ca3af;font-size:.875rem}
.form-row input,.form-row select{background:#374151;border:1px solid #4b5563;color:#f3f4f6;padding:.4rem .6rem;border-radius:4px;flex:1}
button{background:#2563eb;color:#fff;border:none;padding:.5rem 1.25rem;border-radius:6px;cursor:pointer;font-weight:600}
button:hover{background:#1d4ed8}
.refresh{background:#374151;font-size:.75rem;padding:.3rem .75rem}
.refresh:hover{background:#4b5563}
#status-msg{margin-top:.5rem;font-size:.85rem;color:#6ee7b7}
</style>
</head>
<body>
<h1>Environment: {{ENV_NAME}}</h1>
<p class="subtitle">Management UI &middot; {{ADDR}}</p>

<h2>Capabilities</h2>
<div class="card" id="caps-card">Loading...</div>

<h2>Watched Resources <button class="refresh" onclick="loadResources()">Refresh</button></h2>
<div class="card" id="resources-card">Loading...</div>

<h2>Registered Recipes</h2>
<div class="card" id="recipes-card">Loading...</div>

<script>
const API = '';

async function loadCapabilities() {
  const res = await fetch(API + '/api/capabilities');
  const caps = await res.json();
  const el = document.getElementById('caps-card');
  el.innerHTML = '<form id="caps-form">' +
    '<div class="form-row"><label>Region</label><input name="region" value="' + (caps.region||'') + '"></div>' +
    '<div class="form-row"><label>Sovereignty</label><input name="sovereignty" value="' + (caps.sovereignty||'') + '"></div>' +
    '<div class="form-row"><label>Capabilities</label><input name="capabilities" value="' + (caps.capabilities||[]).join(', ') + '"></div>' +
    '<div class="form-row"><label>Cost / Hour</label><input name="costPerHour" type="number" step="0.01" value="' + (caps.costPerHour||0) + '"></div>' +
    '<div class="form-row"><label>Max Replicas</label><input name="maxReplicas" type="number" value="' + (caps.maxReplicas||0) + '"></div>' +
    '<button type="submit">Save &amp; Re-register</button> <span id="status-msg"></span>' +
    '</form>';
  document.getElementById('caps-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const fd = new FormData(e.target);
    const body = {
      region: fd.get('region'),
      sovereignty: fd.get('sovereignty'),
      capabilities: fd.get('capabilities').split(',').map(s=>s.trim()).filter(Boolean),
      costPerHour: parseFloat(fd.get('costPerHour'))||0,
      maxReplicas: parseInt(fd.get('maxReplicas'))||0,
    };
    await fetch(API + '/api/capabilities', {method:'PUT', headers:{'Content-Type':'application/json'}, body: JSON.stringify(body)});
    document.getElementById('status-msg').textContent = 'Saved and re-registered!';
    setTimeout(()=>{document.getElementById('status-msg').textContent='';}, 3000);
  });
}

async function loadResources() {
  const res = await fetch(API + '/api/resources');
  const resources = await res.json();
  if (!resources.length) {
    document.getElementById('resources-card').innerHTML = '<p style="color:#9ca3af">No resources being watched</p>';
    return;
  }
  let html = '<table><thead><tr><th>Name</th><th>Type</th><th>Deployment</th><th>Namespace</th><th>Health</th></tr></thead><tbody>';
  for (const r of resources) {
    let badge = '<span class="badge badge-gray">Unknown</span>';
    if (r.lastHealth) {
      const state = r.lastHealth.split(':')[0];
      const cls = state === 'Running' ? 'badge-green' : state === 'Degraded' ? 'badge-yellow' : state === 'Down' ? 'badge-red' : 'badge-gray';
      badge = '<span class="badge ' + cls + '">' + r.lastHealth + '</span>';
    }
    html += '<tr><td>'+r.resourceName+'</td><td>'+r.resourceType+'</td><td>'+r.deploymentName+'</td><td>'+r.namespace+'</td><td>'+badge+'</td></tr>';
  }
  html += '</tbody></table>';
  document.getElementById('resources-card').innerHTML = html;
}

async function loadRecipes() {
  const res = await fetch(API + '/api/recipes');
  const recipes = await res.json();
  if (!recipes.length) {
    document.getElementById('recipes-card').innerHTML = '<p style="color:#9ca3af">No recipes registered</p>';
    return;
  }
  let html = '<table><thead><tr><th>Resource Type</th><th>Recipe</th><th>Template Path</th></tr></thead><tbody>';
  for (const r of recipes) {
    html += '<tr><td>'+r.resourceType+'</td><td>'+r.recipeName+'</td><td><code>'+r.templatePath+'</code></td></tr>';
  }
  html += '</tbody></table>';
  document.getElementById('recipes-card').innerHTML = html;
}

loadCapabilities();
loadResources();
loadRecipes();
setInterval(loadResources, 10000);
</script>
</body>
</html>`

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
