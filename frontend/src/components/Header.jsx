import { useState, useEffect } from 'react';
import { fetchVitals } from '../api';

export default function Header() {
  const [vitals, setVitals] = useState(null);

  useEffect(() => {
    const load = () => fetchVitals().then(setVitals).catch(() => {});
    load();
    const interval = setInterval(load, 5000);
    return () => clearInterval(interval);
  }, []);

  const sps = vitals?.signals_per_sec || '0.00';
  const pps = vitals?.processing_per_sec || '0.00';
  const lag = vitals?.consumer_lag ?? '--';
  const cache = vitals?.cache_status || '--';

  return (
    <header
      className="flex items-center justify-between shrink-0"
      style={{
        height: 48,
        padding: '0 20px',
        background: 'var(--bg-surface)',
        borderBottom: '1px solid var(--border-subtle)',
      }}
    >
      {/* Left — Brand */}
      <div className="flex items-center gap-3">
        <div
          className="flex items-center justify-center shrink-0"
          style={{
            width: 28,
            height: 28,
            borderRadius: 6,
            background: 'linear-gradient(135deg, #4c8dff 0%, #7c5cfc 100%)',
          }}
        >
          <svg className="w-4 h-4 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
        </div>

        <span className="text-[13px] font-semibold tracking-tight" style={{ color: 'var(--text-primary)' }}>
          IMS Command Center
        </span>

        <div className="flex items-center gap-1.5 ml-0.5">
          <span
            className="animate-pulse-live"
            style={{ width: 6, height: 6, borderRadius: '50%', background: 'var(--green)' }}
          />
          <span style={{ fontSize: 10, fontWeight: 600, letterSpacing: '0.08em', color: 'var(--green)' }}>
            LIVE
          </span>
        </div>
      </div>

      {/* Right — Metrics row */}
      <div className="flex items-center gap-2">
        <MetricChip label="SIG/S" value={sps} />
        <MetricChip label="PROC/S" value={pps} />
        <MetricChip
          label="LAG"
          value={lag}
          valueColor={lag > 100 ? 'var(--red-text)' : lag > 10 ? 'var(--yellow-text)' : 'var(--green-text)'}
        />
        <MetricChip
          label="CACHE"
          value={cache}
          valueColor={cache === 'hot' ? 'var(--green-text)' : 'var(--yellow-text)'}
        />
      </div>
    </header>
  );
}

function MetricChip({ label, value, valueColor }) {
  return (
    <div
      className="flex items-center gap-1.5"
      style={{
        height: 28,
        padding: '0 10px',
        borderRadius: 5,
        background: 'var(--bg-raised)',
        border: '1px solid var(--border-subtle)',
      }}
    >
      <span style={{ fontSize: 9, fontWeight: 600, letterSpacing: '0.08em', color: 'var(--text-faint)' }}>
        {label}
      </span>
      <span
        className="font-mono"
        style={{ fontSize: 12, fontWeight: 600, color: valueColor || 'var(--text-primary)' }}
      >
        {value}
      </span>
    </div>
  );
}
