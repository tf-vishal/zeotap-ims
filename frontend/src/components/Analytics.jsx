import { useState, useEffect } from "react";
import { fetchVitals, fetchLiveIncidents, fetchHealth } from "../api";

export default function Analytics() {
  const [vitals, setVitals] = useState(null);
  const [incidents, setIncidents] = useState([]);
  const [health, setHealth] = useState({
    redis: "down",
    mongodb: "down",
    postgres: "down",
  });

  useEffect(() => {
    const load = () => {
      fetchVitals()
        .then(setVitals)
        .catch(() => {});
      fetchLiveIncidents()
        .then((d) => setIncidents(d.incidents || []))
        .catch(() => {});
      fetchHealth()
        .then(setHealth)
        .catch(() => {});
    };
    load();
    const interval = setInterval(load, 10000);
    return () => clearInterval(interval);
  }, []);

  const mttr = vitals?.mttr_by_component || [];
  const openCount = incidents.filter((i) => i.status === "open").length;
  const totalSignals = incidents.reduce(
    (sum, i) => sum + (i.signal_count || 0),
    0,
  );
  const components = [...new Set(incidents.map((i) => i.component_id))];

  const [showAllServices, setShowAllServices] = useState(false);
  const [showAllMTTR, setShowAllMTTR] = useState(false);

  const displayedComponents = showAllServices
    ? components
    : components.slice(0, 4);
  const displayedMTTR = showAllMTTR ? mttr : mttr.slice(0, 4);

  const formatMTTR = (s) => {
    if (!s || s <= 0) return "--";
    if (s < 60) return `${s.toFixed(1)}s`;
    if (s < 3600) return `${(s / 60).toFixed(1)}m`;
    return `${(s / 3600).toFixed(1)}h`;
  };

  const healthServices = [
    { key: "redis", label: "Redis" },
    { key: "mongodb", label: "MongoDB" },
    { key: "postgres", label: "PostgreSQL" },
  ];

  return (
    <>
      {/* Panel header */}
      <div
        className="flex items-center shrink-0"
        style={{
          height: 44,
          padding: "0 16px",
          borderBottom: "1px solid var(--border-subtle)",
        }}
      >
        <span
          style={{
            fontSize: 11,
            fontWeight: 600,
            letterSpacing: "0.06em",
            textTransform: "uppercase",
            color: "var(--text-secondary)",
          }}
        >
          Analytics
        </span>
      </div>

      {/* Scrollable body */}
      <div className="flex-1 overflow-y-auto" style={{ padding: 16 }}>
        <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
          {/* ── Current State ───────────────────────────── */}
          <Section title="Current State">
            <div
              style={{
                display: "grid",
                gridTemplateColumns: "1fr 1fr",
                gap: 8,
              }}
            >
              <div className="stat-block">
                <div
                  style={{
                    fontSize: 10,
                    fontWeight: 500,
                    textTransform: "uppercase",
                    letterSpacing: "0.05em",
                    color: "var(--text-tertiary)",
                    marginBottom: 4,
                  }}
                >
                  Open
                </div>
                <div
                  className="font-mono"
                  style={{
                    fontSize: 20,
                    fontWeight: 700,
                    color:
                      openCount > 0 ? "var(--red-text)" : "var(--green-text)",
                  }}
                >
                  {openCount}
                </div>
              </div>
              <div className="stat-block">
                <div
                  style={{
                    fontSize: 10,
                    fontWeight: 500,
                    textTransform: "uppercase",
                    letterSpacing: "0.05em",
                    color: "var(--text-tertiary)",
                    marginBottom: 4,
                  }}
                >
                  Signals
                </div>
                <div
                  className="font-mono"
                  style={{
                    fontSize: 20,
                    fontWeight: 700,
                    color: "var(--text-primary)",
                  }}
                >
                  {totalSignals}
                </div>
              </div>
            </div>
          </Section>

          {/* ── System Health (REAL) ─────────────────────── */}
          <Section title="System Health">
            <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
              {healthServices.map(({ key, label }) => {
                const isUp = health[key] === "up";
                return (
                  <div key={key} className="health-row">
                    <span
                      style={{ fontSize: 11, color: "var(--text-secondary)" }}
                    >
                      {label}
                    </span>
                    <div className="flex items-center" style={{ gap: 6 }}>
                      <span
                        style={{
                          width: 6,
                          height: 6,
                          borderRadius: "50%",
                          background: isUp ? "var(--green)" : "var(--red)",
                        }}
                      />
                      <span
                        style={{
                          fontSize: 10,
                          fontWeight: 600,
                          color: isUp ? "var(--green-text)" : "var(--red-text)",
                        }}
                      >
                        {isUp ? "OK" : "DOWN"}
                      </span>
                    </div>
                  </div>
                );
              })}
            </div>
          </Section>

          {/* ── Affected Services ────────────────────────── */}
          <Section title="Affected Services">
            {components.length === 0 ? (
              <EmptyBlock>No affected services</EmptyBlock>
            ) : (
              <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                {displayedComponents.map((cid) => {
                  const inc = incidents.find((i) => i.component_id === cid);
                  return (
                    <div key={cid} className="health-row">
                      <div
                        className="flex items-center"
                        style={{ gap: 8, minWidth: 0 }}
                      >
                        <span
                          style={{
                            width: 7,
                            height: 7,
                            borderRadius: "50%",
                            background:
                              inc?.status === "open"
                                ? "var(--red)"
                                : "var(--green)",
                            flexShrink: 0,
                          }}
                        />
                        <span
                          className="font-mono"
                          style={{
                            fontSize: 11,
                            fontWeight: 500,
                            color: "var(--text-primary)",
                            overflow: "hidden",
                            textOverflow: "ellipsis",
                            whiteSpace: "nowrap",
                          }}
                        >
                          {cid}
                        </span>
                      </div>
                      <span
                        className="font-mono"
                        style={{
                          fontSize: 10,
                          color: "var(--text-faint)",
                          flexShrink: 0,
                          marginLeft: 8,
                        }}
                      >
                        {inc?.signal_count || 0} sig
                      </span>
                    </div>
                  );
                })}
                {components.length > 4 && (
                  <button
                    onClick={() => setShowAllServices(!showAllServices)}
                    className="btn btn-ghost"
                    style={{
                      fontSize: 10,
                      padding: "4px",
                      marginTop: 4,
                      width: "100%",
                    }}
                  >
                    {showAllServices
                      ? "Show Less"
                      : `Show All (${components.length})`}
                  </button>
                )}
              </div>
            )}
          </Section>

          {/* ── MTTR by Component ────────────────────────── */}
          <Section title="MTTR by Component">
            {mttr.length === 0 ? (
              <EmptyBlock>No resolved incidents yet</EmptyBlock>
            ) : (
              <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                {displayedMTTR.map((m) => {
                  const val = formatMTTR(m.avg_mttr_seconds);
                  const col =
                    m.avg_mttr_seconds < 300
                      ? "var(--green-text)"
                      : m.avg_mttr_seconds < 1800
                        ? "var(--yellow-text)"
                        : "var(--red-text)";
                  return (
                    <div key={m.component_id} className="stat-block">
                      <div
                        className="flex items-center justify-between"
                        style={{ marginBottom: 6 }}
                      >
                        <span
                          className="font-mono"
                          style={{
                            fontSize: 11,
                            fontWeight: 500,
                            color: "var(--text-primary)",
                          }}
                        >
                          {m.component_id}
                        </span>
                        <span
                          className="font-mono"
                          style={{ fontSize: 14, fontWeight: 700, color: col }}
                        >
                          {val}
                        </span>
                      </div>
                      <div
                        style={{
                          width: "100%",
                          height: 3,
                          borderRadius: 2,
                          background: "var(--bg-base)",
                          overflow: "hidden",
                        }}
                      >
                        <div
                          style={{
                            height: "100%",
                            borderRadius: 2,
                            width: `${Math.min(100, (m.avg_mttr_seconds / 3600) * 100)}%`,
                            background: col,
                            opacity: 0.5,
                            transition: "width 0.4s ease",
                          }}
                        />
                      </div>
                      <div
                        className="flex items-center justify-between"
                        style={{ marginTop: 4 }}
                      >
                        <span
                          style={{ fontSize: 10, color: "var(--text-faint)" }}
                        >
                          {m.total_incidents} closed
                        </span>
                        <span
                          style={{
                            fontSize: 9,
                            fontWeight: 500,
                            textTransform: "uppercase",
                            letterSpacing: "0.06em",
                            color: "var(--text-faint)",
                          }}
                        >
                          Avg MTTR
                        </span>
                      </div>
                    </div>
                  );
                })}
                {mttr.length > 4 && (
                  <button
                    onClick={() => setShowAllMTTR(!showAllMTTR)}
                    className="btn btn-ghost"
                    style={{
                      fontSize: 10,
                      padding: "4px",
                      marginTop: 4,
                      width: "100%",
                    }}
                  >
                    {showAllMTTR ? "Show Less" : `Show All (${mttr.length})`}
                  </button>
                )}
              </div>
            )}
          </Section>
        </div>
      </div>
    </>
  );
}

function Section({ title, children }) {
  return (
    <div>
      <div className="section-title">{title}</div>
      {children}
    </div>
  );
}

function EmptyBlock({ children }) {
  return (
    <div
      className="flex items-center justify-center"
      style={{
        padding: "12px 0",
        fontSize: 11,
        color: "var(--text-faint)",
        background: "var(--bg-raised)",
        border: "1px solid var(--border-faint)",
        borderRadius: 5,
        textAlign: "center",
      }}
    >
      {children}
    </div>
  );
}
