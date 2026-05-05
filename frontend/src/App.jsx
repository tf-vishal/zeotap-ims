import { useState } from 'react';
import Header from './components/Header';
import LiveFeed from './components/LiveFeed';
import IncidentDetail from './components/IncidentDetail';
import RCAModal from './components/RCAModal';
import Analytics from './components/Analytics';

export default function App() {
  const [selectedIncident, setSelectedIncident] = useState(null);
  const [rcaIncident, setRcaIncident] = useState(null);
  const [refreshKey, setRefreshKey] = useState(0);

  const handleIncidentClosed = () => {
    setRcaIncident(null);
    setSelectedIncident(null);
    setRefreshKey(k => k + 1);
  };

  return (
    <div className="h-screen flex flex-col overflow-hidden" style={{ background: 'var(--bg-base)' }}>
      <Header />

      {/* 3-column grid with 1px dividers */}
      <div className="flex flex-1 min-h-0">
        {/* Left — 280px fixed */}
        <div
          className="flex flex-col h-full shrink-0 overflow-hidden"
          style={{ width: 280, background: 'var(--bg-surface)', borderRight: '1px solid var(--border-subtle)' }}
        >
          <LiveFeed
            key={refreshKey}
            selectedId={selectedIncident?.id}
            onSelectIncident={setSelectedIncident}
          />
        </div>

        {/* Center — flexible */}
        <div
          className="flex flex-col h-full flex-1 overflow-hidden"
          style={{ background: 'var(--bg-base)', borderRight: '1px solid var(--border-subtle)' }}
        >
          <IncidentDetail
            incident={selectedIncident}
            onClose={() => setSelectedIncident(null)}
            onOpenRCA={setRcaIncident}
          />
        </div>

        {/* Right — 300px fixed */}
        <div
          className="flex flex-col h-full shrink-0 overflow-hidden"
          style={{ width: 300, background: 'var(--bg-surface)' }}
        >
          <Analytics />
        </div>
      </div>

      {rcaIncident && (
        <RCAModal
          incident={rcaIncident}
          onClose={() => setRcaIncident(null)}
          onClosed={handleIncidentClosed}
        />
      )}
    </div>
  );
}
