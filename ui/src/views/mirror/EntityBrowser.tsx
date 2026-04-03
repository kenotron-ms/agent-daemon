import { useEffect, useState } from 'react'
import { Connector, Entity, listEntities } from '../../api/mirror'

function renderData(data: unknown): string {
  if (typeof data === 'string') {
    try { return JSON.stringify(JSON.parse(data), null, 2) } catch { return data }
  }
  return JSON.stringify(data, null, 2)
}

interface Props { connector: Connector }

export default function EntityBrowser({ connector }: Props) {
  const [entities, setEntities] = useState<Entity[]>([])
  const [selected, setSelected] = useState<Entity | null>(null)

  useEffect(() => {
    setEntities([])
    setSelected(null)
    listEntities(connector.id).then(setEntities).catch(console.error)
  }, [connector.id])

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', background: 'var(--bg-right)' }}>
      {/* Header */}
      <div style={{
        display: 'flex', alignItems: 'center', gap: 8,
        padding: '0 16px',
        height: 32,
        background: 'var(--bg-pane-title)',
        borderBottom: '1px solid var(--border)',
        flexShrink: 0,
      }}>
        <span style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-primary)' }}>
          {connector.name}
        </span>
        <span style={{ fontSize: 11, color: 'var(--text-very-muted)' }}>
          {entities.length} entities
          {connector.lastSyncAt && ` · ${new Date(connector.lastSyncAt).toLocaleTimeString()}`}
        </span>
      </div>

      <div style={{ display: 'flex', flex: 1, overflow: 'hidden' }}>
        {/* Entity list */}
        <div style={{
          width: 260, flexShrink: 0,
          borderRight: '1px solid var(--border)',
          overflowY: 'auto',
        }} className="canvas-scroll">
          {entities.map(e => (
            <button
              key={e.address}
              onClick={() => setSelected(e)}
              style={{
                width: '100%', textAlign: 'left',
                padding: '6px 12px 6px 14px',
                background: selected?.address === e.address ? 'var(--bg-sidebar-active)' : 'transparent',
                borderLeft: selected?.address === e.address ? '2px solid var(--amber)' : '2px solid transparent',
                borderBottom: '1px solid var(--border)',
                cursor: 'pointer',
                transition: 'background 0.12s ease',
              }}
              onMouseEnter={e => {
                if (selected?.address !== (e.currentTarget as HTMLElement).dataset.addr)
                  (e.currentTarget as HTMLElement).style.background = 'rgba(0,0,0,0.03)'
              }}
              onMouseLeave={ev => {
                if (selected?.address !== e.address)
                  (ev.currentTarget as HTMLElement).style.background = 'transparent'
              }}
            >
              <div style={{
                fontSize: 11.5, color: 'var(--text-primary)',
                overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
              }}>{e.address}</div>
              <div style={{ fontSize: 10, color: 'var(--text-very-muted)', marginTop: 2 }}>{e.type}</div>
            </button>
          ))}
          {entities.length === 0 && (
            <div style={{ padding: '16px 14px', fontSize: 11, color: 'var(--text-very-muted)' }}>No entities</div>
          )}
        </div>

        {/* JSON detail */}
        <div style={{ flex: 1, overflowY: 'auto', padding: 16 }} className="canvas-scroll">
          {selected ? (
            <pre style={{
              fontFamily: 'var(--font-mono)',
              fontSize: 11.5,
              color: 'var(--text-primary)',
              whiteSpace: 'pre-wrap',
              lineHeight: 1.6,
              margin: 0,
            }}>{renderData(selected.data)}</pre>
          ) : (
            <span style={{ fontSize: 12, color: 'var(--text-very-muted)' }}>Select an entity</span>
          )}
        </div>
      </div>
    </div>
  )
}
