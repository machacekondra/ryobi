import { useState, useEffect } from 'react'
import './App.css'

const API = '/api/v1'
const fetchJSON = async (url: string) => { const r = await fetch(url); const d = await r.json(); return (d.value || []).map((v: any) => typeof v === 'string' ? JSON.parse(v) : v) }

type View = 'catalog' | 'environments' | 'placements' | 'applications' | 'resource-types'

function App() {
  const [view, setView] = useState<View>('catalog')
  const [catalog, setCatalog] = useState<any[]>([])
  const [environments, setEnvironments] = useState<any[]>([])
  const [placements, setPlacements] = useState<any[]>([])
  const [apps, setApps] = useState<any[]>([])
  const [resourceTypes, setResourceTypes] = useState<any[]>([])
  const [editing, setEditing] = useState<any>(null)
  const [message, setMessage] = useState('')

  useEffect(() => { load() }, [view])
  const load = () => { loadCatalog(); loadEnvs(); loadPlacements(); loadApps(); loadResourceTypes() }
  const loadCatalog = async () => setCatalog(await fetchJSON(`${API}/catalog-items`))
  const loadEnvs = async () => setEnvironments(await fetchJSON(`${API}/environments`))
  const loadResourceTypes = async () => setResourceTypes(await fetchJSON(`${API}/resource-types`))
  const loadPlacements = async () => setPlacements(await fetchJSON(`${API}/placements`))
  const loadApps = async () => setApps(await fetchJSON(`${API}/applications`))

  const flash = (msg: string) => { setMessage(msg); setTimeout(() => setMessage(''), 3000) }

  async function deleteThing(path: string, name: string) {
    if (!confirm(`Delete "${name}"?`)) return
    await fetch(`${API}/${path}/${name}`, { method: 'DELETE' })
    flash(`Deleted "${name}"`); load()
  }

  async function saveCatalog(e: React.FormEvent) {
    e.preventDefault()
    const form = e.target as HTMLFormElement
    const fd = new FormData(form)
    const name = fd.get('name') as string
    let resources: any[] = []
    try { resources = JSON.parse(fd.get('resources') as string || '[]') } catch { }
    let parameters: any[] = []
    try { parameters = JSON.parse(fd.get('parameters') as string || '[]') } catch { }
    await fetch(`${API}/catalog-items/${name}`, {
      method: 'PUT', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, properties: { description: fd.get('description'), icon: fd.get('icon'), category: fd.get('category'), resources, parameters } })
    })
    flash(`Catalog item "${name}" saved`); setEditing(null); loadCatalog()
  }

  async function savePlacement(e: React.FormEvent) {
    e.preventDefault()
    const form = e.target as HTMLFormElement
    const fd = new FormData(form)
    const name = fd.get('name') as string
    await fetch(`${API}/placements/${name}`, {
      method: 'PUT', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, properties: { resourceType: fd.get('resourceType'), constraints: { region: fd.get('region'), sovereignty: fd.get('sovereignty') }, preferences: { cost: fd.get('cost'), availableResources: fd.get('availableResources') }, priority: parseInt(fd.get('priority') as string) || 0 } })
    })
    flash(`Placement "${name}" saved`); setEditing(null); loadPlacements()
  }

  async function saveResourceType(e: React.FormEvent) {
    e.preventDefault()
    const form = e.target as HTMLFormElement
    const fd = new FormData(form)
    const name = fd.get('name') as string
    let schema: any = {}
    try { schema = JSON.parse(fd.get('schema') as string || '{}') } catch { }
    await fetch(`${API}/resource-types/${name}`, {
      method: 'PUT', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, properties: { description: fd.get('description'), icon: fd.get('icon'), category: fd.get('category'), schema } })
    })
    flash(`Resource type "${name}" saved`); setEditing(null); loadResourceTypes()
  }

  return (
    <div className="app">
      <nav className="sidebar"><div className="logo">Ryobi Admin</div>
        {(['catalog', 'resource-types', 'environments', 'placements', 'applications'] as View[]).map(v => {
          const label = v === 'resource-types' ? 'Resource Types' : v.charAt(0).toUpperCase() + v.slice(1)
          return <button key={v} className={view === v ? 'active' : ''} onClick={() => { setView(v); setEditing(null) }}>{label}</button>
        })}
      </nav>
      <main className="main">
        {message && <div className="flash">{message}</div>}

        {view === 'catalog' && !editing && <>
          <div className="header"><h1>Catalog Items</h1><button className="primary" onClick={() => setEditing({ name: '', properties: {} })}>+ New</button></div>
          <table><thead><tr><th>Name</th><th>Description</th><th>Category</th><th>Resources</th><th></th></tr></thead><tbody>
            {catalog.map(c => <tr key={c.name}><td>{c.name}</td><td>{c.properties?.description}</td><td><span className="tag">{c.properties?.category}</span></td><td>{c.properties?.resources?.length || 0}</td><td><button className="sm" onClick={() => setEditing(c)}>Edit</button> <button className="sm danger" onClick={() => deleteThing('catalog-items', c.name)}>Delete</button></td></tr>)}
            {catalog.length === 0 && <tr><td colSpan={5} className="empty">No catalog items</td></tr>}
          </tbody></table>
        </>}
        {view === 'catalog' && editing && <>
          <h1>{editing.name ? 'Edit' : 'New'} Catalog Item</h1>
          <form className="form" onSubmit={saveCatalog}>
            <div className="field"><label>Name</label><input name="name" defaultValue={editing.name} required /></div>
            <div className="field"><label>Description</label><input name="description" defaultValue={editing.properties?.description} /></div>
            <div className="field"><label>Icon (emoji)</label><input name="icon" defaultValue={editing.properties?.icon} placeholder="📦" /></div>
            <div className="field"><label>Category</label><input name="category" defaultValue={editing.properties?.category} placeholder="web" /></div>
            <div className="field"><label>Resources (JSON)</label><textarea name="resources" rows={8} defaultValue={JSON.stringify(editing.properties?.resources || [], null, 2)} /></div>
            <div className="field"><label>Parameters (JSON)</label><textarea name="parameters" rows={6} defaultValue={JSON.stringify(editing.properties?.parameters || [], null, 2)} /></div>
            <div className="actions"><button className="secondary" type="button" onClick={() => setEditing(null)}>Cancel</button><button className="primary" type="submit">Save</button></div>
          </form>
        </>}

        {view === 'environments' && <>
          <h1>Environments</h1><p className="sub">Connected environment agents</p>
          <table><thead><tr><th>Name</th><th>Region</th><th>Sovereignty</th><th>Providers</th><th>Resource Types</th></tr></thead><tbody>
            {environments.map(e => {
              const p = e.properties || {}
              const providers = Object.keys(p.providers || {}).join(', ')
              const resTypes = Object.keys(p.recipes || {}).join(', ')
              const caps = p.recipeConfig?.terraform || {}
              return <tr key={e.name}><td>{e.name}</td><td>{caps.region || '—'}</td><td>{caps.sovereignty || '—'}</td><td>{providers || '—'}</td><td>{resTypes || '—'}</td></tr>
            })}
            {environments.length === 0 && <tr><td colSpan={5} className="empty">No environments connected</td></tr>}
          </tbody></table>
        </>}

        {view === 'placements' && !editing && <>
          <div className="header"><h1>Placement Rules</h1><button className="primary" onClick={() => setEditing({ name: '', properties: { constraints: {}, preferences: {} } })}>+ New</button></div>
          <table><thead><tr><th>Name</th><th>Resource Type</th><th>Region</th><th>Sovereignty</th><th>Cost</th><th>Priority</th><th></th></tr></thead><tbody>
            {placements.map(p => <tr key={p.name}><td>{p.name}</td><td>{p.properties?.resourceType}</td><td>{p.properties?.constraints?.region || '—'}</td><td>{p.properties?.constraints?.sovereignty || '—'}</td><td>{p.properties?.preferences?.cost || '—'}</td><td>{p.properties?.priority || 0}</td><td><button className="sm" onClick={() => setEditing(p)}>Edit</button> <button className="sm danger" onClick={() => deleteThing('placements', p.name)}>Delete</button></td></tr>)}
            {placements.length === 0 && <tr><td colSpan={7} className="empty">No placement rules</td></tr>}
          </tbody></table>
        </>}
        {view === 'placements' && editing && <>
          <h1>{editing.name ? 'Edit' : 'New'} Placement Rule</h1>
          <form className="form" onSubmit={savePlacement}>
            <div className="field"><label>Name</label><input name="name" defaultValue={editing.name} required /></div>
            <div className="field"><label>Resource Type</label><input name="resourceType" defaultValue={editing.properties?.resourceType} placeholder="Ryobi.Compute/containers" required /></div>
            <div className="field"><label>Region constraint</label><input name="region" defaultValue={editing.properties?.constraints?.region} /></div>
            <div className="field"><label>Sovereignty constraint</label><input name="sovereignty" defaultValue={editing.properties?.constraints?.sovereignty} /></div>
            <div className="field"><label>Cost preference</label><select name="cost" defaultValue={editing.properties?.preferences?.cost}><option value="">None</option><option value="minimize">Minimize</option></select></div>
            <div className="field"><label>Resources preference</label><select name="availableResources" defaultValue={editing.properties?.preferences?.availableResources}><option value="">None</option><option value="maximize">Maximize</option></select></div>
            <div className="field"><label>Priority</label><input name="priority" type="number" defaultValue={editing.properties?.priority || 0} /></div>
            <div className="actions"><button className="secondary" type="button" onClick={() => setEditing(null)}>Cancel</button><button className="primary" type="submit">Save</button></div>
          </form>
        </>}

        {view === 'resource-types' && !editing && <>
          <div className="header"><h1>Resource Types</h1><button className="primary" onClick={() => setEditing({ name: '', properties: {} })}>+ New</button></div>
          <table><thead><tr><th>Name</th><th>Description</th><th>Category</th><th></th></tr></thead><tbody>
            {resourceTypes.map(rt => <tr key={rt.name}><td>{rt.properties?.icon || '📦'} {rt.name}</td><td>{rt.properties?.description || '—'}</td><td><span className="tag">{rt.properties?.category || '—'}</span></td><td><button className="sm" onClick={() => setEditing(rt)}>Edit</button> <button className="sm danger" onClick={() => deleteThing('resource-types', rt.name)}>Delete</button></td></tr>)}
            {resourceTypes.length === 0 && <tr><td colSpan={4} className="empty">No resource types defined</td></tr>}
          </tbody></table>
        </>}
        {view === 'resource-types' && editing && <>
          <h1>{editing.name ? 'Edit' : 'New'} Resource Type</h1>
          <form className="form" onSubmit={saveResourceType}>
            <div className="field"><label>Name</label><input name="name" defaultValue={editing.name} placeholder="Ryobi.Compute/containers" required /></div>
            <div className="field"><label>Description</label><input name="description" defaultValue={editing.properties?.description} /></div>
            <div className="field"><label>Icon (emoji)</label><input name="icon" defaultValue={editing.properties?.icon} placeholder="📦" /></div>
            <div className="field"><label>Category</label><input name="category" defaultValue={editing.properties?.category} placeholder="compute" /></div>
            <div className="field"><label>Schema (JSON, optional)</label><textarea name="schema" rows={6} defaultValue={JSON.stringify(editing.properties?.schema || {}, null, 2)} /></div>
            <div className="actions"><button className="secondary" type="button" onClick={() => setEditing(null)}>Cancel</button><button className="primary" type="submit">Save</button></div>
          </form>
        </>}

        {view === 'applications' && <>
          <h1>Applications</h1>
          <table><thead><tr><th>Name</th><th>Catalog Item</th><th></th></tr></thead><tbody>
            {apps.map(a => <tr key={a.name}><td>{a.name}</td><td>{a.properties?.catalogItem || '—'}</td><td><button className="sm danger" onClick={() => deleteThing('applications', a.name)}>Delete</button></td></tr>)}
            {apps.length === 0 && <tr><td colSpan={3} className="empty">No applications</td></tr>}
          </tbody></table>
        </>}
      </main>
    </div>
  )
}
export default App
