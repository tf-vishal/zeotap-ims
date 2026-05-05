import { useState, useEffect } from 'react';
import { fetchIncidentSignals, updateIncidentStatus } from '../api';

export default function IncidentDetail({ incident, onClose, onOpenRCA }) {
  const [signals, setSignals] = useState([]);
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState('overview');
  const [expanded, setExpanded] = useState(null);
  const [status, setStatus] = useState(incident?.status);

  useEffect(() => {
    if (!incident) return;
    setStatus(incident.status);
    setActiveTab('overview');
    setExpanded(null);
    setLoading(true);
    fetchIncidentSignals(incident.id)
      .then(data => { setSignals(data.signals || []); setLoading(false); })
      .catch(() => setLoading(false));
  }, [incident?.id]);

  /* ── Empty state ──────────────────────────────────────── */
  if (!incident) {
    return (
      <div className="flex flex-col items-center justify-center h-full" style={{ padding: 32 }}>
        {/* Container card for the empty state */}
        <div
          className="flex flex-col items-center justify-center"
          style={{
            width: '100%',
            maxWidth: 360,
            padding: '48px 32px',
            background: 'var(--bg-surface)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 10,
          }}
        >
          <svg className="mb-4" style={{ width: 40, height: 40, color: 'var(--text-faint)', opacity: 0.4 }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M15 15l-2 5L9 9l11 4-5 2zm0 0l5 5M7.188 2.239l.777 2.897M5.136 7.965l-2.898-.777M13.95 4.05l-2.122 2.122m-5.657 5.656l-2.12 2.122" />
          </svg>
          <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--text-secondary)', marginBottom: 4 }}>
            No incident selected
          </span>
          <span style={{ fontSize: 12, color: 'var(--text-faint)', textAlign: 'center' }}>
            Select an incident from the left panel to begin investigation
          </span>
        </div>
      </div>
    );
  }

  const handleStatusChange = async (s) => {
    try {
      await updateIncidentStatus(incident.id, s);
      setStatus(s);
      incident.status = s;
    } catch (e) { alert(e.message); }
  };

  const timeStr = (ts) => ts ? new Date(ts).toLocaleString() : '--';
  const sevColor = (s) => s === 'critical' ? 'var(--red)' : s === 'warning' ? 'var(--yellow)' : 'var(--accent)';
  const sevBg = (s) => s === 'critical' ? 'var(--red-dim)' : s === 'warning' ? 'var(--yellow-dim)' : 'var(--accent-dim)';
  const sevText = (s) => s === 'critical' ? 'var(--red-text)' : s === 'warning' ? 'var(--yellow-text)' : 'var(--text-secondary)';

  return (
    <>
      {/* ── Fixed header ────────────────────────────────── */}
      <div className="shrink-0" style={{ borderBottom: '1px solid var(--border-subtle)' }}>
        {/* Row 1: Title + Actions */}
        <div className="flex items-center justify-between" style={{ height: 44, padding: '0 20px' }}>
          <div className="flex items-center gap-2.5">
            <button
              onClick={onClose}
              className="flex items-center justify-center"
              style={{
                width: 24, height: 24, borderRadius: 4,
                color: 'var(--text-faint)',
                transition: 'color 0.12s, background 0.12s',
              }}
              onMouseEnter={e => { e.currentTarget.style.background = 'var(--bg-hover)'; e.currentTarget.style.color = 'var(--text-primary)'; }}
              onMouseLeave={e => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = 'var(--text-faint)'; }}
            >
              <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
            <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-primary)' }}>
              {incident.component_id}
            </span>
            <span className={`badge badge-${status}`}>{status}</span>
          </div>

          <div className="flex items-center gap-1.5">
            {status === 'open' && (
              <button className="btn btn-warning" onClick={() => handleStatusChange('investigating')}>Investigate</button>
            )}
            {status === 'investigating' && (
              <button className="btn btn-success" onClick={() => handleStatusChange('resolved')}>Resolve</button>
            )}
            {status !== 'closed' && (
              <button className="btn btn-danger" onClick={() => onOpenRCA(incident)}>Close + RCA</button>
            )}
          </div>
        </div>

        {/* Row 2: Tabs */}
        <div className="flex items-end" style={{ padding: '0 20px' }}>
          <button onClick={() => setActiveTab('overview')} className={`tab ${activeTab === 'overview' ? 'active' : ''}`}>
            Overview
          </button>
          <button onClick={() => setActiveTab('signals')} className={`tab ${activeTab === 'signals' ? 'active' : ''}`}>
            Signals ({signals.length})
          </button>
        </div>
      </div>

      {/* ── Scrollable body ─────────────────────────────── */}
      <div className="flex-1 overflow-y-auto" style={{ padding: 20 }}>
        {activeTab === 'overview' && (
          <div className="animate-fade-in" style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            {/* Stat cards */}
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 8 }}>
              <Stat label="Incident ID" value={incident.id.slice(0, 8) + '…'} mono />
              <Stat label="Signal Count" value={incident.signal_count} />
              <Stat label="Status" value={status} badge />
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
              <Stat label="First Seen" value={timeStr(incident.first_seen_at)} />
              <Stat label="Last Seen" value={timeStr(incident.last_seen_at)} />
            </div>

            {/* Activity */}
            <div>
              <div className="section-title">Recent Activity</div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                {signals.slice(0, 8).map((sig, i) => (
                  <div
                    key={i}
                    className="flex items-center"
                    style={{ gap: 8, padding: '6px 10px', background: 'var(--bg-raised)', borderRadius: 5, border: '1px solid var(--border-faint)' }}
                  >
                    <span style={{ width: 5, height: 5, borderRadius: '50%', background: sevColor(sig.severity), flexShrink: 0 }} />
                    <span className="font-mono" style={{ fontSize: 11, color: 'var(--text-secondary)', fontWeight: 500 }}>
                      {sig.signal_type}
                    </span>
                    <span style={{ fontSize: 10, padding: '0 5px', borderRadius: 3, background: sevBg(sig.severity), color: sevText(sig.severity) }}>
                      {sig.severity}
                    </span>
                    <span className="font-mono" style={{ fontSize: 10, color: 'var(--text-faint)', marginLeft: 'auto' }}>
                      {sig.timestamp?.split('T')[1]?.slice(0, 12) || '--'}
                    </span>
                  </div>
                ))}
                {signals.length > 8 && (
                  <button
                    onClick={() => setActiveTab('signals')}
                    style={{ fontSize: 11, fontWeight: 500, color: 'var(--accent)', padding: '4px 0', textAlign: 'left', background: 'none', border: 'none', cursor: 'pointer' }}
                  >
                    View all {signals.length} signals →
                  </button>
                )}
                {signals.length === 0 && !loading && (
                  <span style={{ fontSize: 11, color: 'var(--text-faint)', padding: '8px 0' }}>No signals recorded.</span>
                )}
              </div>
            </div>
          </div>
        )}

        {activeTab === 'signals' && (
          <div className="animate-fade-in" style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
            {loading ? (
              <div className="flex items-center justify-center" style={{ padding: 48 }}>
                <span style={{ fontSize: 12, color: 'var(--text-tertiary)' }}>Loading signals…</span>
              </div>
            ) : signals.length === 0 ? (
              <div className="flex items-center justify-center" style={{ padding: 48 }}>
                <span style={{ fontSize: 12, color: 'var(--text-faint)' }}>No signals found.</span>
              </div>
            ) : (
              signals.map((sig, i) => (
                <div key={i} className="signal-row" onClick={() => setExpanded(expanded === i ? null : i)}>
                  <div className="flex items-center justify-between">
                    <div className="flex items-center" style={{ gap: 8 }}>
                      <span style={{ width: 5, height: 5, borderRadius: '50%', background: sevColor(sig.severity), flexShrink: 0 }} />
                      <span className="font-mono" style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-primary)' }}>
                        {sig.signal_type}
                      </span>
                      <span style={{ fontSize: 10, padding: '0 5px', borderRadius: 3, background: sevBg(sig.severity), color: sevText(sig.severity), fontWeight: 500 }}>
                        {sig.severity}
                      </span>
                    </div>
                    <span className="font-mono" style={{ fontSize: 10, color: 'var(--text-faint)' }}>
                      {sig.timestamp}
                    </span>
                  </div>
                  {expanded === i && (
                    <pre
                      className="font-mono"
                      style={{
                        marginTop: 8,
                        padding: 10,
                        borderRadius: 4,
                        fontSize: 11,
                        background: 'var(--bg-base)',
                        color: 'var(--green-text)',
                        border: '1px solid var(--border-faint)',
                        overflowX: 'auto',
                      }}
                    >
                      {JSON.stringify(sig.metadata, null, 2)}
                    </pre>
                  )}
                </div>
              ))
            )}
          </div>
        )}
      </div>
    </>
  );
}

function Stat({ label, value, mono, badge }) {
  return (
    <div className="stat-block">
      <div style={{ fontSize: 10, fontWeight: 500, textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--text-tertiary)', marginBottom: 4 }}>
        {label}
      </div>
      {badge ? (
        <span className={`badge badge-${value}`}>{value}</span>
      ) : (
        <div className={mono ? 'font-mono' : ''} style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-primary)' }}>
          {value}
        </div>
      )}
    </div>
  );
}
