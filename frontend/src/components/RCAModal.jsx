import { useState } from 'react';
import { closeIncident } from '../api';

export default function RCAModal({ incident, onClose, onClosed }) {
  const [notes, setNotes] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  if (!incident) return null;

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!notes.trim()) { setError('RCA notes are mandatory for incident closure.'); return; }
    setSubmitting(true);
    setError('');
    try { await closeIncident(incident.id, notes.trim()); onClosed(); }
    catch (err) { setError(err.message); }
    finally { setSubmitting(false); }
  };

  return (
    <div className="modal-backdrop animate-fade-in">
      <div className="modal-box">
        {/* Header */}
        <div className="flex items-center justify-between" style={{ marginBottom: 16 }}>
          <div>
            <div style={{ fontSize: 14, fontWeight: 700, color: 'var(--text-primary)' }}>Close Incident</div>
            <div className="font-mono" style={{ fontSize: 11, color: 'var(--text-faint)', marginTop: 2 }}>{incident.id}</div>
          </div>
          <button
            onClick={onClose}
            className="flex items-center justify-center"
            style={{ width: 28, height: 28, borderRadius: 5, border: 'none', background: 'transparent', color: 'var(--text-faint)', cursor: 'pointer', transition: 'background 0.12s' }}
            onMouseEnter={e => e.currentTarget.style.background = 'var(--bg-hover)'}
            onMouseLeave={e => e.currentTarget.style.background = 'transparent'}
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Warning banner */}
        <div
          className="flex items-start"
          style={{ gap: 8, padding: 12, borderRadius: 6, marginBottom: 16, background: 'var(--orange-dim)', border: '1px solid rgba(237,137,54,0.15)' }}
        >
          <svg style={{ width: 16, height: 16, color: 'var(--orange)', flexShrink: 0, marginTop: 1 }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z" />
          </svg>
          <span style={{ fontSize: 11, lineHeight: 1.5, color: 'var(--orange-text)' }}>
            Root Cause Analysis is <strong>required</strong>. Include root cause, impact, and remediation.
          </span>
        </div>

        {/* Info */}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, marginBottom: 16 }}>
          <div className="stat-block">
            <div style={{ fontSize: 10, fontWeight: 500, textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--text-tertiary)', marginBottom: 4 }}>Component</div>
            <div className="font-mono" style={{ fontSize: 12, fontWeight: 600, color: 'var(--text-primary)' }}>{incident.component_id}</div>
          </div>
          <div className="stat-block">
            <div style={{ fontSize: 10, fontWeight: 500, textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--text-tertiary)', marginBottom: 4 }}>Signals</div>
            <div className="font-mono" style={{ fontSize: 12, fontWeight: 600, color: 'var(--text-primary)' }}>{incident.signal_count}</div>
          </div>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit}>
          <label style={{ display: 'block', fontSize: 11, fontWeight: 600, color: 'var(--text-secondary)', marginBottom: 6 }}>
            RCA Notes
          </label>
          <textarea
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            placeholder="Root cause: ...&#10;Impact: ...&#10;Remediation: ..."
            rows={5}
            style={{
              width: '100%', padding: 12, borderRadius: 6, resize: 'none',
              fontSize: 12, lineHeight: 1.6,
              background: 'var(--bg-raised)', border: '1px solid var(--border-default)',
              color: 'var(--text-primary)', outline: 'none',
              transition: 'border-color 0.15s',
            }}
            onFocus={e => e.target.style.borderColor = 'var(--accent)'}
            onBlur={e => e.target.style.borderColor = 'var(--border-default)'}
          />
          {error && <p style={{ marginTop: 8, fontSize: 11, color: 'var(--red-text)' }}>{error}</p>}
          <div className="flex justify-end" style={{ gap: 8, marginTop: 16 }}>
            <button type="button" onClick={onClose} className="btn btn-ghost">Cancel</button>
            <button type="submit" disabled={submitting} className="btn btn-primary">
              {submitting ? 'Closing…' : 'Close Incident'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
