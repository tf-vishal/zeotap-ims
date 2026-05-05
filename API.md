# API Documentation

The API runs by default on `http://localhost:8080`.

## 1. Health Check
`GET /health`

Checks the connectivity of all underlying database dependencies (Redis, MongoDB, PostgreSQL).

**Response (200 OK):**
```json
{
  "services": {
    "mongodb": "up",
    "postgres": "up",
    "redis": "up"
  },
  "status": "healthy",
  "timestamp": "2026-05-05T08:00:00Z"
}
```

## 2. Signal Ingestion
`POST /api/v1/signals`

Ingests raw incident signals. Supports both single JSON objects and **JSON Arrays** for high-throughput batching.

**Request Body (Batch Array):**
```json
[
  {
    "component_id": "svc-payments",
    "signal_type": "timeout",
    "severity": "critical",
    "timestamp": "2026-05-05T08:01:00Z",
    "metadata": { "endpoint": "/charge", "latency_ms": 5005 }
  },
  {
    "component_id": "svc-auth",
    "signal_type": "latency_spike",
    "severity": "warning",
    "timestamp": "2026-05-05T08:01:01Z",
    "metadata": { "region": "us-east" }
  }
]
```

**Response (202 Accepted):**
```json
{
  "accepted": 2,
  "message": "signals buffered for processing",
  "status": "accepted"
}
```

## 3. Live Dashboard Feed
`GET /api/v1/incidents/live`

Fetches the Top 100 most critical active incidents directly from the Redis Hot-Path cache.

**Response (200 OK):**
```json
{
  "count": 1,
  "incidents": [
    {
      "id": "e7fe4c7f-5472-4426",
      "component_id": "svc-payments",
      "status": "open",
      "signal_count": 150,
      "first_seen_at": "2026-05-05T08:00:00Z",
      "last_seen_at": "2026-05-05T08:00:05Z"
    }
  ]
}
```

## 4. Get Forensic Signals
`GET /api/v1/incidents/:id/signals`

Retrieves the raw, unstructured JSON payloads associated with a specific debounced Work Item ID from MongoDB.

## 5. Update Status
`PATCH /api/v1/incidents/:id`

Updates the state of an incident (`open` or `investigating` or `resolved`).
*Note: Cannot transition to `closed` via this endpoint.*

**Request Body:**
```json
{ "status": "investigating" }
```

## 6. Close Incident (RCA)
`POST /api/v1/incidents/:id/close`

Transitions an incident to `closed`, enforcing mandatory Root Cause Analysis (RCA) notes. Upon success, removes the incident from the Redis Hot-Path and records the resolution timestamp in PostgreSQL for MTTR calculation.

**Request Body:**
```json
{
  "rca_notes": "Root cause: Expired DB Cert. Impact: Payments down for 5 mins. Fix: Rotated cert."
}
```

## 7. System Vitals (Analytics)
`GET /api/v1/analytics/vitals`

Retrieves global ingestion throughput, consumer lag, MTTR per component, and historical hourly metrics.
