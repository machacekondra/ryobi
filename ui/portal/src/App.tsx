import { useState, useEffect } from 'react'
import './App.css'

const API = '/api/v1'

interface CatalogItem { name: string; properties: { description?: string; icon?: string; category?: string; resources?: any[]; parameters?: { name: string; description?: string; type?: string; default?: any; required?: boolean }[] } }
interface Application { name: string; properties: { catalogItem?: string } }
interface Resource { name: string; properties: { resourceType?: string; status?: { state?: string; environment?: string; health?: { state?: string; readyReplicas?: number; desiredReplicas?: number } } } }

function App() {
  const [view, setView] = useState<'catalog' | 'apps' | 'deploy'>('catalog')
  const [catalog, setCatalog] = useState<CatalogItem[]>([])
  const [apps, setApps] = useState<Application[]>([])
  const [selectedCatalog, setSelectedCatalog] = useState<CatalogItem | null>(null)
  const [selectedApp, setSelectedApp] = useState<string | null>(null)
  const [resources, setResources] = useState<Resource[]>([])
  const [deployName, setDeployName] = useState('')
  const [deployParams, setDeployParams] = useState<Record<string, string>>({})
  const [deploying, setDeploying] = useState(false)
  const [message, setMessage] = useState('')

  useEffect(() => { loadCatalog(); loadApps() }, [])
  useEffect(() => { if (selectedApp) loadResources(selectedApp) }, [selectedApp])
  useEffect(() => { const i = setInterval(() => { if (selectedApp) loadResources(selectedApp) }, 5000); return () => clearInterval(i) }, [selectedApp])

  const fetchJSON = async (url: string) => { const r = await fetch(url); const d = await r.json(); return (d.value || []).map((v: any) => typeof v === 'string' ? JSON.parse(v) : v) }
  const loadCatalog = async () => setCatalog(await fetchJSON(`${API}/catalog-items`))
  const loadApps = async () => setApps(await fetchJSON(`${API}/applications`))
  const loadResources = async (n: string) => setResources(await fetchJSON(`${API}/applications/${n}/resources`))

  function startDeploy(item: CatalogItem) { setSelectedCatalog(item); setDeployName(item.name); setDeployParams({}); setView('deploy') }

  async function executeDeploy() {
    if (!selectedCatalog || !deployName) return
    setDeploying(true); setMessage('')
    try {
      await fetch(`${API}/applications/${deployName}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name: deployName, properties: { catalogItem: selectedCatalog.name } }) })
      for (const res of selectedCatalog.properties.resources || []) {
        const params = { ...res.parameters }; for (const [k, v] of Object.entries(deployParams)) { if (v) params[k] = isNaN(Number(v)) ? v : Number(v) }
        await fetch(`${API}/applications/${deployName}/resources/${res.name}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name: res.name, properties: { resourceType: res.type, parameters: params } }) })
      }
      setMessage('Deployed!'); loadApps(); setTimeout(() => { setView('apps'); setSelectedApp(deployName); setMessage('') }, 1200)
    } catch (e: any) { setMessage('Error: ' + e.message) } finally { setDeploying(false) }
  }

  async function deleteApp(name: string) {
    if (!confirm('Delete "' + name + '" and all its resources?')) return
    const items = await fetchJSON(`${API}/applications/${name}/resources`)
    for (const r of items) await fetch(`${API}/applications/${name}/resources/${r.name}`, { method: 'DELETE' })
    await fetch(`${API}/applications/${name}`, { method: 'DELETE' })
    setSelectedApp(null); loadApps()
  }

  const healthBadge = (s?: string) => { const cls = s === 'Running' ? 'green' : s === 'Degraded' ? 'yellow' : s === 'Down' ? 'red' : 'gray'; return <span className={'badge ' + cls}>{s || '—'}</span> }
  const stateBadge = (s?: string) => { const cls = s === 'Succeeded' ? 'green' : s === 'Failed' ? 'red' : s === 'Deploying' ? 'blue' : 'gray'; return <span className={'badge ' + cls}>{s || 'Pending'}</span> }

  return (
    <div className="app">
      <nav className="sidebar"><div className="logo">Ryobi Portal</div>
        <button className={view === 'catalog' ? 'active' : ''} onClick={() => setView('catalog')}>Catalog</button>
        <button className={view === 'apps' ? 'active' : ''} onClick={() => { setView('apps'); setSelectedApp(null) }}>My Apps</button>
      </nav>
      <main className="main">
        {view === 'catalog' && <>
          <h1>Service Catalog</h1><p className="sub">Deploy applications from templates</p>
          <div className="grid">{catalog.map(i => (
            <div key={i.name} className="card">
              <div className="card-head"><span className="icon">{i.properties.icon || '📦'}</span><h3>{i.name}</h3><span className="cat">{i.properties.category}</span></div>
              <p>{i.properties.description}</p>
              <div className="meta">{i.properties.resources?.length || 0} resources · {i.properties.parameters?.length || 0} params</div>
              <button className="primary" onClick={() => startDeploy(i)}>Deploy</button>
            </div>
          ))}{catalog.length === 0 && <p className="empty">No catalog items</p>}</div>
        </>}
        {view === 'deploy' && selectedCatalog && <>
          <h1>Deploy: {selectedCatalog.name}</h1><p className="sub">{selectedCatalog.properties.description}</p>
          <div className="form"><div className="field"><label>Application Name</label><input value={deployName} onChange={e => setDeployName(e.target.value)} /></div>
            {selectedCatalog.properties.parameters?.map(p => (<div key={p.name} className="field"><label>{p.name}{p.required && <span className="req">*</span>}</label>{p.description && <span className="hint">{p.description}</span>}<input placeholder={String(p.default ?? '')} value={deployParams[p.name] || ''} onChange={e => setDeployParams({ ...deployParams, [p.name]: e.target.value })} /></div>))}
            <div className="actions"><button className="secondary" onClick={() => setView('catalog')}>Cancel</button><button className="primary" onClick={executeDeploy} disabled={deploying || !deployName}>{deploying ? 'Deploying...' : 'Deploy'}</button></div>
            {message && <div className={message.startsWith('Error') ? 'msg err' : 'msg ok'}>{message}</div>}
          </div>
        </>}
        {view === 'apps' && !selectedApp && <>
          <h1>My Applications</h1>
          <table><thead><tr><th>Name</th><th>Catalog</th><th></th></tr></thead><tbody>
            {apps.map(a => <tr key={a.name}><td><a href="#" onClick={e => { e.preventDefault(); setSelectedApp(a.name) }}>{a.name}</a></td><td>{a.properties.catalogItem || '—'}</td><td><button className="sm danger" onClick={() => deleteApp(a.name)}>Delete</button></td></tr>)}
            {apps.length === 0 && <tr><td colSpan={3} className="empty">No applications</td></tr>}
          </tbody></table>
        </>}
        {view === 'apps' && selectedApp && <>
          <p className="crumb"><a href="#" onClick={e => { e.preventDefault(); setSelectedApp(null) }}>Apps</a> / {selectedApp}</p>
          <h1>{selectedApp}</h1>
          <table><thead><tr><th>Resource</th><th>Type</th><th>Environment</th><th>State</th><th>Health</th><th>Ready</th></tr></thead><tbody>
            {resources.map(r => <tr key={r.name}><td>{r.name}</td><td>{r.properties.resourceType}</td><td>{r.properties.status?.environment || '—'}</td><td>{stateBadge(r.properties.status?.state)}</td><td>{healthBadge(r.properties.status?.health?.state)}</td><td>{r.properties.status?.health?.desiredReplicas ? `${r.properties.status?.health?.readyReplicas || 0}/${r.properties.status?.health?.desiredReplicas}` : '—'}</td></tr>)}
            {resources.length === 0 && <tr><td colSpan={6} className="empty">No resources</td></tr>}
          </tbody></table>
          <button className="danger" onClick={() => deleteApp(selectedApp)}>Delete Application</button>
        </>}
      </main>
    </div>
  )
}
export default App
