import { useState, useEffect } from 'react';
import { fetchLiveIncidents } from '../api';

export default function LiveFeed({ selectedId, onSelectIncident }) {
  const [incidents, setIncidents] = useState([]);
  const [loading, setLoading] = useState(true);
  const [filterSeverity, setFilterSeverity] = useState('all');

  const getSeverity = (count) => {
    if (count >= 5000) return 'critical';
    if (count >= 1000) return 'warning';
    return 'info';
  };

  const filteredIncidents = incidents.filter(inc => {
    if (filterSeverity === 'all') return true;
    return getSeverity(inc.signal_count) === filterSeverity;
  });

  useEffect(() => {
    const load = () => {
      fetchLiveIncidents()
        .then(data => { setIncidents(data.incidents || []); setLoading(false); })
        .catch(() => setLoading(false));
    };
    load();
    const interval = setInterval(load, 3000);
    return () => clearInterval(interval);
  }, []);

  const timeAgo = (ts) => {
    if (!ts) return '--';
    const diff = Math.floor((Date.now() - new Date(ts).getTime()) / 1000);
    if (diff < 60) return `${diff}s ago`;
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    return `${Math.floor(diff / 3600)}h ago`;
  };

  return (
    <>
      {/* Panel header */}
      <div
        className="flex items-center justify-between shrink-0"
        style={{ height: 44, padding: '0 16px', borderBottom: '1px solid var(--border-subtle)' }}
      >
        <div className="flex items-center gap-2">
          <span style={{ width: 6, height: 6, borderRadius: '50%', background: 'var(--red)' }} />
          <span style={{ fontSize: 11, fontWeight: 600, letterSpacing: '0.06em', textTransform: 'uppercase', color: 'var(--text-secondary)' }}>
            Incidents
          </span>
        </div>
        <div className="flex items-center gap-3">
          <select 
            value={filterSeverity} 
            onChange={e => setFilterSeverity(e.target.value)}
            style={{
              fontSize: 10,
              padding: '2px 4px',
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-subtle)',
              borderRadius: 4,
              color: 'var(--text-secondary)',
              outline: 'none',
              cursor: 'pointer'
            }}
          >
            <option value="all">All Severities</option>
            <option value="critical">Critical (5k+)</option>
            <option value="warning">Warning (1k+)</option>
            <option value="info">Info</option>
          </select>
          <span
            className="font-mono"
            style={{
              fontSize: 11,
              fontWeight: 600,
              color: 'var(--text-faint)',
              background: 'var(--bg-elevated)',
              padding: '2px 8px',
              borderRadius: 4,
            }}
          >
            {filteredIncidents.length}
          </span>
        </div>
      </div>

      {/* Body */}
      <div className="flex-1 overflow-y-auto">
        {loading ? (
          <EmptyCenter>
            <span style={{ color: 'var(--text-tertiary)', fontSize: 12 }}>Loading…</span>
          </EmptyCenter>
        ) : filteredIncidents.length === 0 ? (
          <EmptyCenter>
            <svg className="w-8 h-8 mb-2" style={{ color: 'var(--text-faint)', opacity: 0.5 }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-tertiary)' }}>No matches</span>
            <span style={{ fontSize: 11, color: 'var(--text-faint)', marginTop: 2 }}>No active incidents found</span>
          </EmptyCenter>
        ) : (
          filteredIncidents.map((inc) => {
            const sev = getSeverity(inc.signal_count);
            const sevColor = sev === 'critical' ? 'var(--red)' : sev === 'warning' ? 'var(--orange)' : 'var(--blue)';
            return (
            <div
              key={inc.id}
              onClick={() => onSelectIncident(inc)}
              className={`incident-row ${selectedId === inc.id ? 'active' : ''}`}
            >
              <div className="flex items-center justify-between" style={{ marginBottom: 4 }}>
                <div className="flex items-center gap-2">
                  <span style={{ width: 6, height: 6, borderRadius: '50%', background: sevColor, flexShrink: 0 }} />
                  <span className="font-mono" style={{ fontSize: 12, fontWeight: 600, color: 'var(--text-primary)' }}>
                    {inc.component_id}
                  </span>
                </div>
                <span className={`badge badge-${inc.status}`}>{inc.status}</span>
              </div>
              <div className="flex items-center" style={{ gap: 8, paddingLeft: 14 }}>
                <span style={{ fontSize: 11, color: 'var(--text-tertiary)' }}>
                  <span className="font-mono" style={{ fontWeight: 600, color: 'var(--text-secondary)' }}>{inc.signal_count}</span> signals
                </span>
                <span style={{ fontSize: 10, color: 'var(--text-faint)' }}>{timeAgo(inc.last_seen_at)}</span>
              </div>
            </div>
          )})
        )}
      </div>
    </>
  );
}

function EmptyCenter({ children }) {
  return (
    <div className="flex flex-col items-center justify-center" style={{ height: '100%', minHeight: 200 }}>
      {children}
    </div>
  );
}
