const API_BASE = '';

export async function fetchLiveIncidents() {
  const res = await fetch(`${API_BASE}/api/v1/incidents/live`);
  if (!res.ok) throw new Error('Failed to fetch live incidents');
  return res.json();
}

export async function fetchIncidentSignals(id) {
  const res = await fetch(`${API_BASE}/api/v1/incidents/${id}/signals`);
  if (!res.ok) throw new Error('Failed to fetch signals');
  return res.json();
}

export async function closeIncident(id, rcaNotes) {
  const res = await fetch(`${API_BASE}/api/v1/incidents/${id}/close`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ rca_notes: rcaNotes }),
  });
  if (!res.ok) {
    const data = await res.json();
    throw new Error(data.error || 'Failed to close incident');
  }
  return res.json();
}

export async function updateIncidentStatus(id, status) {
  const res = await fetch(`${API_BASE}/api/v1/incidents/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ status }),
  });
  if (!res.ok) throw new Error('Failed to update status');
  return res.json();
}

export async function fetchVitals() {
  const res = await fetch(`${API_BASE}/api/v1/analytics/vitals`);
  if (!res.ok) throw new Error('Failed to fetch vitals');
  return res.json();
}

export async function fetchHealth() {
  try {
    const res = await fetch(`${API_BASE}/health`);
    const data = await res.json();
    return data.services || { redis: 'down', mongodb: 'down', postgres: 'down' };
  } catch {
    return { redis: 'down', mongodb: 'down', postgres: 'down' };
  }
}
