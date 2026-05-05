# Incident Management System (IMS) — Prompts, Execution Overviews & Plans

> This document captures every prompt, execution overview, and implementation plan used across all stages of building the Incident Management System — from raw signal ingestion to the full real-time dashboard and high-concurrency tuning.

---

## Table of Contents

1. [Stages 1 & 2 — Ingestion & Buffering Layer](#stages-1--2--ingestion--buffering-layer)
2. [Stages 3 & 4 — Processing Layer](#stages-3--4--processing-layer)
3. [Stages 5 & 6 — Workflow Engine, Analytics & UI](#stages-5--6--workflow-engine-analytics--ui)
4. [Stage 7 — High-Concurrency Tuning & Network Optimizations](#stage-7--high-concurrency-tuning--network-optimizations)

---

## Stages 1 & 2 — Ingestion & Buffering Layer

### Prompt

> **Role:** You are a senior SRE designing a high-throughput ingestion system.

**System Overview:**
We are building the ingestion and buffering layer of an Incident Management System (IMS). This system receives high volumes of signals (errors, alerts, events, etc.) from distributed systems. It must handle bursts of 10,000+ signals per second without blocking or crashing. The system must be asynchronous, resilient, and optimized for throughput.

**Scope (Strict) — Implement only:**

- Signal ingestion API
- Rate limiting
- Redis Stream buffering
- Basic observability

**Do NOT implement:** workers or consumers, database logic, debouncing, or anything beyond this scope.

---

**Core Requirements:**

**1. Ingestion API**

- Expose an HTTP endpoint to receive signals
- Accept structured JSON with: `component_id`, `signal_type`, `severity`, `timestamp`, `metadata`
- The API must be lightweight and non-blocking

**2. Rate Limiting**

- Apply a global rate limiter
- System must protect itself under extreme load
- Target capacity: 10,000–12,000 req/sec
- Requests beyond the limit must be rejected safely

**3. Buffering (Critical)**

- All accepted signals must be pushed to a Redis Stream
- This acts as a buffer to decouple ingestion from processing
- The ingestion layer must never depend on downstream systems

**4. Performance Constraints**

- Do not perform any database operations in the request path
- Minimize latency per request
- Avoid unnecessary allocations or blocking calls

**Observability Requirements:**

**1. Throughput Tracking**

- Maintain a thread-safe counter of ingested signals every 5 seconds
- Calculate signals/sec, log the throughput, and reset the counter

**2. Health Endpoint**

- Provide a health check endpoint
- It must verify: service is running, Redis connection is healthy
- Response should reflect system status clearly

**Configuration:**

- All config must come from environment variables — no hardcoded values
- Must include: server port, rate limit, Redis connection details, Redis stream name

**Architecture Constraints:**

- Use a clean, modular structure
- Keep ingestion, configuration, and Redis logic isolated
- Use idiomatic Go concurrency
- Ensure thread safety and avoid race conditions

**Engineering Expectations:**

- Code should be production-grade and readable
- Add comments explaining non-trivial logic
- Keep implementation simple but correct
- Do not over-engineer or introduce unnecessary abstraction

**Goal:** Deliver a robust ingestion layer capable of safely accepting and buffering high-volume signals while maintaining visibility into system health and throughput.

---

### Execution Overview

Stages 1 and 2 focus on building the **ingestion and buffering layer** of the Incident Management System.

The system implements a **high-throughput, non-blocking HTTP ingestion API** capable of handling burst traffic in the range of **10,000–12,000 requests per second**. Incoming signals are accepted as structured JSON and processed with minimal overhead to ensure low latency.

To maintain system stability under heavy load, a **global rate limiter** is applied. Requests exceeding the configured threshold are safely rejected, preventing resource exhaustion.

All accepted signals are asynchronously written to a **Redis Stream**, which acts as a buffering layer. This ensures complete **decoupling of ingestion from downstream processing**, allowing the system to remain reliable even when consumers are unavailable.

Basic observability is included through **throughput tracking** and a **health check endpoint**, providing visibility into system performance and Redis connectivity.

The scope is intentionally limited to ingestion, rate limiting, buffering, and observability. No downstream processing, database interaction, or worker logic is included in these stages.

---

### Implementation Plan

#### 1. Configuration Setup

- Load all required parameters from environment variables:
  - Server port
  - Rate limit threshold
  - Redis connection details
  - Redis stream name
- Ensure no hardcoded values are used

#### 2. Ingestion API Implementation

- Develop a lightweight HTTP server
- Implement an endpoint to receive signal payloads
- Parse and validate structured JSON input
- Ensure the request path remains non-blocking and minimal

#### 3. Rate Limiting

- Introduce a global rate limiter to control incoming traffic
- Enforce system limits (10k–12k req/sec)
- Reject excess requests safely without impacting system stability

#### 4. Redis Stream Buffering

- Establish a Redis connection
- Push all accepted signals to a Redis Stream
- Use the stream as a buffer to decouple ingestion from processing
- Ensure ingestion does not depend on downstream systems

#### 5. Observability

- Implement a thread-safe counter for ingested signals
- Calculate and log throughput every 5 seconds
- Reset metrics after each interval

#### 6. Health Endpoint

- Provide a health check endpoint
- Validate: service availability and Redis connectivity
- Return a clear system status response

#### 7. Concurrency & Performance

- Use idiomatic Go concurrency patterns
- Ensure thread safety for shared resources
- Avoid blocking operations and unnecessary allocations in the request path

---

## Stages 3 & 4 — Processing Layer

### Prompt

> **Role:** You are a senior SRE building the processing layer of a high-throughput Incident Management System.

**System Overview:**
We are building the processing layer of an Incident Management System (IMS). The ingestion layer already pushes signals into a Redis Stream. Your job is to:

- Consume signals asynchronously
- Deduplicate and aggregate them correctly
- Persist both raw and processed data across multiple storage systems

The system must be concurrent, fault-tolerant, and consistent under load.

**Scope (Strict) — Implement only:**

- Redis Stream consumption
- Worker pool using goroutines
- Debouncing and aggregation logic
- Multi-sink persistence (MongoDB, PostgreSQL, Redis state)
- Processing-layer observability

---

**Core Requirements:**

**1. Consumer Group Processing**

- Use Redis Streams consumer groups
- Support multiple concurrent workers (config: `WORKER_COUNT`)
- Each message must be processed by only one worker

**2. Worker Pool**

- Implement a configurable goroutine worker pool
- Workers must:
  - Continuously read from Redis
  - Process messages concurrently
- Worker count must come from environment variables
- Avoid blocking operations in the processing loop

**3. Debouncing & Aggregation (Critical)**

Each signal contains a `component_id`. Rules:

- Apply a 10-second sliding window per `component_id`
- If multiple signals arrive within this window: only one Work Item should be created; all subsequent signals must be aggregated into the same Work Item

**4. Multi-Sink Persistence**

After processing each signal:

- **MongoDB (Data Lake)**
  - Store every raw signal payload
  - Collection: `signal_audits`
  - Append-only

- **PostgreSQL (Transactional Store)**
  - Store aggregated Work Items
  - Schema: `id`, `component_id`, `status` (open / investigating / resolved / closed), `signal_count`, `first_seen_at`, `last_seen_at`

- **Redis (Hot State)**
  - Maintain active debounce windows
  - Counters / metadata for fast access

**Observability Requirements:**

**1. Throughput Logging**

- Every 5 seconds, log: `Processed_Signals/sec` and `Work_Items_Created/sec`
- Use thread-safe counters and reset after logging

**2. Health Endpoint**

- Extend `/health` to verify:
  - Service status
  - Redis connectivity
  - MongoDB connectivity
  - PostgreSQL connectivity
  - Consumer lag (pending messages in stream)

**Configuration — all from environment variables:**

- Redis (host, port, stream, consumer group)
- MongoDB URI
- PostgreSQL URI
- Worker count
- Debounce window (default: 10 seconds)

**Architecture Constraints:**

- Follow a clean, modular structure
- Separate concerns (Redis, MongoDB, PostgreSQL)
- Use idiomatic Go concurrency
- Ensure thread safety throughout

**Infrastructure:** Start all databases from Docker Compose.

**Engineering Expectations:**

- Write production-grade, readable Go code
- Handle edge cases: duplicate deliveries, worker crashes, partial failures
- Add comments for non-trivial logic (especially debouncing)
- Keep implementation simple but correct
- Avoid unnecessary abstractions

**Goal:** Deliver a robust processing system that consumes signals reliably at scale, correctly debounces and aggregates events, persists data across multiple systems safely, and maintains visibility into system health and throughput.

---

### Execution Overview

Stages 3 and 4 focus on implementing the **processing layer** of the Incident Management System.

At this stage, signals are already being ingested into a **Redis Stream**. The system is responsible for **asynchronously consuming these signals**, applying **debouncing and aggregation logic**, and persisting data across multiple storage systems.

A **consumer group-based architecture** is used to enable **horizontal scalability and fault tolerance**, ensuring that each message is processed by exactly one worker. A configurable **worker pool (goroutines)** allows concurrent processing while maintaining controlled resource usage.

To prevent redundant work, the system applies a **10-second sliding window per `component_id`**. Multiple signals within this window are grouped into a single **Work Item**, ensuring efficient aggregation while preserving all raw data.

Processed data is persisted across three systems:

- **MongoDB** for raw signal storage (append-only audit log)
- **PostgreSQL** for aggregated Work Items (transactional state)
- **Redis** for maintaining hot state (active debounce windows and counters)

Basic observability is included through **throughput logging** and an enhanced **health endpoint**, providing visibility into system performance, dependencies, and consumer lag.

The system is designed to be **concurrent, fault-tolerant, and consistent under load**, while strictly adhering to the defined scope.

---

### Implementation Plan

#### 1. Configuration Setup

- Load all configurations from environment variables:
  - Redis (host, port, stream, consumer group)
  - MongoDB URI
  - PostgreSQL URI
  - Worker count
  - Debounce window (default: 10 seconds)
- Ensure no hardcoded values are used

#### 2. Redis Consumer Group Setup

- Create and use a Redis Stream consumer group
- Ensure:
  - Each message is delivered to only one worker
  - Pending messages can be reprocessed in case of failures
- Continuously read from the stream using blocking reads

#### 3. Worker Pool Implementation

- Implement a configurable goroutine-based worker pool
- Workers:
  - Continuously consume messages from Redis
  - Process signals concurrently
  - Operate independently and safely under high load
- Worker count is controlled via environment variables

#### 4. Debouncing & Aggregation

- Apply a **per `component_id` sliding window (10 seconds)**
- Logic:
  - First signal → create a new Work Item
  - Subsequent signals within the window → aggregate into the same Work Item
- Maintain debounce state in Redis for fast access and consistency
- Ensure correctness under concurrent updates

#### 5. Multi-Sink Persistence

**MongoDB**

- Store every raw signal payload in `signal_audits`
- Append-only writes

**PostgreSQL**

- Store aggregated Work Items with full schema
- Upsert on `component_id` within active debounce window

**Redis**

- Maintain active debounce windows
- Store counters and aggregation metadata

#### 6. Observability

- Every 5 seconds: log `Processed_Signals/sec` and `Work_Items_Created/sec`
- Use thread-safe atomic counters; reset after each interval

#### 7. Health Endpoint

- Extend `/health` to validate:
  - Service status
  - Redis connectivity
  - MongoDB connectivity
  - PostgreSQL connectivity
  - Consumer lag (pending messages in stream)

#### 8. Concurrency & Fault Tolerance

- Use idiomatic Go concurrency (goroutines, channels, sync primitives)
- Handle: duplicate message delivery, worker crashes, partial persistence failures
- Ensure thread safety across all shared resources

#### 9. Infrastructure Setup

- Use **Docker Compose** to run: Redis, MongoDB, PostgreSQL
- Ensure services are properly networked and accessible to all workers

---

## Stages 5 & 6 — Workflow Engine, Analytics & UI

### Prompt

> **Role:** You are a Senior Full-Stack & Systems Engineer responsible for building the final interaction and analytics layers of a high-throughput Incident Management System.

**System Context:**
The ingestion and processing layers are already complete:

- Signals are ingested via API and stored in Redis Streams
- Workers process, debounce, and aggregate signals
- Data is persisted in MongoDB (raw signals), PostgreSQL (aggregated work items), and Redis (hot state)

Your task is to build the **user-facing workflow layer**, **real-time caching layer**, and **analytics system**.

**Scope (Strict) — Implement:**

- Redis-based Hot Path (live dashboard cache)
- Aggregation / analytics engine (MTTR, trends)
- Workflow API (incident lifecycle + RCA)
- React dashboard (live feed + investigation + closure)
- Docker Compose setup for the full system

**Do NOT implement:** ingestion logic, worker/debouncing core logic, or CI/CD pipelines.

---

**Core Requirements:**

**1. Redis Hot Path (Live Feed)**

- Use Redis as a low-latency cache for active incidents
- Implement:
  - **Sorted Set (ZSET)** → store `incident_id` sorted by `last_seen_at`
  - **Hash (HSET)** → store incident metadata: `component_id`, `status`, `signal_count`
- Ensure:
  - Fast reads for UI (<10ms target)
  - Atomic updates when incidents change
  - Removal from cache when incident is closed

**2. Aggregation & Analytics**

- Implement a background aggregation system to compute:
  - **MTTR (Mean Time to Resolution)**
  - Incident counts (hourly / daily)
  - Error rates per component
- Aggregation must:
  - Run asynchronously
  - Be decoupled from real-time processing
  - Not impact system throughput

**3. Workflow API**

- Implement APIs for:
  - Live incident feed (from Redis)
  - Incident details and history
  - Incident status updates
  - RCA submission
- Enforce rule: **an incident cannot be marked as "Closed" without an RCA**
- Ensure consistency between PostgreSQL (source of truth) and Redis (cache layer)

**4. Investigation Support**

- Fetch raw signal data from MongoDB
- Allow full visibility into signal payloads per incident
- Ensure efficient querying and response times

**5. React Dashboard**

- Build UI using **React + Tailwind CSS**
- Include:
  - Live incident feed (real-time view)
  - Incident detail view (signals + metadata)
  - RCA submission form
- Requirements:
  - Fast rendering using cached data
  - Clean, high-density SRE-style layout

**6. Observability**

- Display system metrics: signals/sec, processing lag
- Track: cache efficiency, average MTTR
- Extend `/health` to verify: Redis, MongoDB, PostgreSQL, Aggregation service

**7. Infrastructure (Docker Compose)**

- Provide a full Docker Compose setup for: backend services, Redis, MongoDB, PostgreSQL, frontend
- Ensure: services are properly networked, configuration is environment-driven, easy local setup

**Architecture Constraints:**

- Ensure thread-safe updates to Redis
- Keep caching and DB logic clearly separated
- Avoid blocking operations in critical paths
- Maintain clean, modular architecture

**Goal:** Build a production-ready incident command center that provides real-time visibility using Redis Hot Path, enables deep investigation via MongoDB audit logs, supports structured incident resolution with RCA enforcement, and delivers actionable insights through analytics (MTTR, trends).

---

### Execution Overview

Stages 5 and 6 introduce the **human interaction layer** and **analytics capabilities** of the Incident Management System.

At this stage, processed data from the backend is transformed into a **real-time command center** for monitoring, investigation, and incident resolution.

A **Redis-based Hot Path** is implemented to serve a low-latency live feed of active incidents. This avoids repeated heavy queries to PostgreSQL and ensures near real-time UI responsiveness.

An **aggregation layer** is introduced to compute system-level metrics such as **MTTR (Mean Time to Resolution)** and incident trends. These computations are performed asynchronously to avoid impacting core processing performance.

A **Workflow API** enables interaction with the system, including fetching live incident data, viewing historical signal data, and submitting Root Cause Analysis (RCA).

On top of this, a **React-based dashboard** provides a real-time incident feed, deep inspection of raw signals, and incident lifecycle management with mandatory RCA enforcement. The system maintains consistency between cache and storage layers, ensuring accurate and up-to-date system state.

---

### Implementation Plan

#### 1. Redis Hot Path

- Implement ZSET for incident ordering by `last_seen_at`
- Implement HSET for fast incident metadata lookups
- Ensure atomic updates on incident state changes
- Purge cache entries on incident closure

#### 2. Aggregation & Analytics Engine

- Implement a background goroutine for async metric computation
- Compute MTTR from PostgreSQL `first_seen_at` / `last_seen_at` deltas on closed incidents
- Aggregate incident counts per hour / day using time-bucketed queries
- Compute per-component error rates from `signal_audits` in MongoDB
- Store computed aggregates in Redis for fast dashboard access

#### 3. Workflow API

- `GET /incidents` — Fetch live feed from Redis ZSET
- `GET /incidents/:id` — Fetch full incident details from PostgreSQL
- `GET /incidents/:id/signals` — Fetch raw signals from MongoDB
- `PATCH /incidents/:id/status` — Update status; enforce RCA requirement before allowing `closed`
- `POST /incidents/:id/rca` — Submit RCA; update PostgreSQL and invalidate Redis cache

#### 4. React Dashboard

- **Live Feed View** — polling or SSE from `/incidents`, render high-density incident list with severity indicators
- **Detail View** — signal timeline, metadata, status history
- **RCA Form** — gated form that enforces RCA text before allowing closure

#### 5. Observability Extensions

- Display ingestion throughput (signals/sec) sourced from the ingestion-layer metrics
- Display processing lag from Redis pending entry count
- Track and display cache hit/miss ratio
- Show average MTTR across resolved incidents

#### 6. Docker Compose (Full System)

- Define services: ingestion API, processing workers, workflow API, React frontend, Redis, MongoDB, PostgreSQL
- Use a shared internal network for service-to-service communication
- Drive all configuration through `.env` / environment variables
- Expose only necessary ports to the host

---

## Stage 7 — High-Concurrency Tuning & Network Optimizations

### Prompt

> "When I'm running the docker compose file it's not exceeding the 5k cap max, and showing error: `read: connection reset by peer`. And when I run the API service and frontend service locally, not in container, it is accepting everything and giving no error. Can you refactor the current setup to ensure it is suitable for high-throughput scenarios (10k+ RPS)?"

---

### Execution Overview

Stage 7 addresses a critical bottleneck discovered during load testing: **Docker Bridge NAT limitations were causing connection resets under heavy synthetic load**, capping throughput at approximately 5,000 req/sec when running fully containerized — while the same services running locally had no such ceiling.

The root cause was the combination of Docker's bridge network NAT overhead and a per-HTTP-connection rate limiter that did not account for batched payloads. The following changes resolved the issue and pushed sustained ingestion beyond **275,000 signals/sec** in benchmarks.

---

### Implementation Plan

#### 1. JSON Array Batching

- Restructure the payload format to accept an **array of signals** per HTTP request (e.g., 500 signals per request)
- This dramatically reduces the number of TCP connections required at high signal volumes
- Reduces per-connection overhead and NAT pressure on the Docker bridge

#### 2. Nginx Configuration Hardening

- Increase `worker_connections` to **10,000** per Nginx worker
- Switch event model to `epoll` for efficient I/O multiplexing under high connection counts
- Tune keepalive settings to reduce connection churn

#### 3. Rate Limiter Refactor

- Migrate from a **per-HTTP-connection** rate limit model to a **token-consumption model** based on `len(signals_array)`
- This ensures the rate limit accurately reflects signal throughput rather than request count
- Prevents batch requests from bypassing effective limits while correctly accounting for burst size

#### 4. Result

- Resolved `connection reset by peer` errors under Docker Compose
- Achieved sustained ingestion of **275,000+ signals/sec** in load benchmarks
- System correctly enforces throughput limits at the signal level regardless of batch size

---

_End of Document_
