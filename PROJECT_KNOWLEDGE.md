# RideSharing Microservices Project — Complete Reference

> Auto-generated knowledge base. Read this file to understand the full project architecture, services, data flow, and conventions.

---

## 1. Overview

A **ride-sharing platform** (like Uber/Lyft) built with:
- **Go** backend microservices
- **Next.js 15** (React 19) web frontend
- **Kubernetes** for orchestration (via **Tilt** for local dev)
- **RabbitMQ** for async event-driven communication (AMQP)
- **gRPC + Protobuf** for inter-service RPC
- **MongoDB** for persistence (driver v2)
- **Stripe** for payments
- **OpenTelemetry + Jaeger** for tracing
- **OSRM** (public API) for route/distance calculation

---

## 2. Project Structure

```
microservices-go/
├── go.mod                      # Single Go module: "ride-sharing" (Go 1.23)
├── go.sum
├── Makefile                    # Proto generation: `make generate-proto`
├── Tiltfile                    # Tilt config for local K8s dev
├── build/                      # Compiled Go binaries (api-gateway, trip-service)
├── docs/architecture/          # Architecture decision docs (RabbitMQ flow, trip creation flow)
├── infra/
│   ├── development/            # Local dev configs
│   │   ├── docker/             # Dockerfiles for each service
│   │   └── k8s/                # K8s manifests (Deployments, Services, ConfigMap)
│   └── production/             # Production configs (GCP Artifact Registry images)
│       ├── docker/
│       └── k8s/
├── services/
│   ├── api-gateway/            # HTTP API gateway (Go)
│   └── trip-service/           # Trip domain service (Go, Clean Architecture)
├── shared/                     # Shared Go packages across services
│   ├── contracts/              # API response types, AMQP routing keys, WebSocket messages
│   ├── env/                    # Environment variable helpers
│   ├── retry/                  # Exponential backoff retry utility
│   ├── types/                  # Shared domain types (Coordinate, Route, OsrmApiResponse)
│   └── util/                   # Misc utilities (avatar URLs)
├── tools/
│   └── create_service.go       # CLI scaffold tool: `go run tools/create_service.go -name payment`
└── web/                        # Next.js 15 frontend
    └── src/
        ├── app/                # Next.js App Router pages
        ├── components/         # React components (maps, trip UI, driver UI, Stripe)
        ├── hooks/              # WebSocket hooks (rider + driver connections)
        ├── contracts.ts        # Frontend API contract types & event enums
        ├── types.ts            # Frontend domain types
        └── constants.ts        # API_URL, WEBSOCKET_URL
```

---

## 3. Services

### 3.1 API Gateway (`services/api-gateway/`)

**Role:** Entry point for all frontend HTTP requests. Routes to backend services.

| Detail | Value |
|---|---|
| Port | `:8081` (configurable via `HTTP_ADDR` env) |
| K8s Service type | **LoadBalancer** (externally accessible) |
| Framework | `net/http` (stdlib) |

**Files:**
- `main.go` — Server setup, registers routes
- `http.go` — Handler implementations
- `types.go` — Request/response types
- `json.go` — JSON response helper

**Endpoints (planned/implemented):**
| Endpoint | Method | Status |
|---|---|---|
| `/trip/preview` | POST | Stubbed (returns "ok", has a logic bug) |
| `/trip/start` | POST | Defined in frontend contracts, not yet implemented |
| `/drivers` | WebSocket | Defined in frontend contracts, not yet implemented |
| `/riders` | WebSocket | Defined in frontend contracts, not yet implemented |

**Bug in `http.go`:**
```go
// Line 18: condition is inverted — should be == "" not != ""
if reqBody.UserID != "" {
    writeJSON(w, http.StatusBadRequest, "userID is required")
```

**Config:** Reads `GATEWAY_HTTP_ADDR` from K8s ConfigMap `app-config` (default `:8081`).

---

### 3.2 Trip Service (`services/trip-service/`)

**Role:** Core trip domain — route calculation, trip creation, fare management.

| Detail | Value |
|---|---|
| Port | `:8083` (hardcoded) |
| K8s Service type | **ClusterIP** (internal only, no port-forward in Tiltfile) |
| Architecture | Clean Architecture (Domain → Service → Infrastructure) |

**Structure:**
```
trip-service/
├── cmd/main.go                          # Entry point: wires repo → service → HTTP handler
├── internal/
│   ├── domain/trip.go                   # Models + interfaces
│   ├── service/service.go               # Business logic (CreateTrip, GetRoute)
│   └── infrastructure/
│       ├── http/http.go                 # HTTP handlers
│       ├── repository/inmem.go          # In-memory repository (map-based)
│       ├── events/                      # Empty (planned: RabbitMQ events)
│       └── grpc/                        # Empty (planned: gRPC handlers)
├── pkg/types/                           # Empty (planned: shared types)
└── README.md
```

**Endpoints:**
| Endpoint | Method | Description |
|---|---|---|
| `/preview` | POST | Get route from OSRM API for pickup→destination |

**Request body for `/preview`:**
```json
{
  "userID": "string",
  "pickup":      { "latitude": 40.7580, "longitude": -73.9855 },
  "destination": { "latitude": 40.7829, "longitude": -73.9654 }
}
```

**Domain models (`domain/trip.go`):**
```go
type TripModel struct {
    ID       bson.ObjectID
    UserID   string
    Status   string          // "pending", etc.
    RideFare *RideFareModel
}

type TripRepository interface {
    CreateTrip(ctx, trip) (*TripModel, error)
}

type TripService interface {
    CreateTrip(ctx, fare) (*TripModel, error)
    GetRoute(ctx, pickup, destination) (*OsrmApiResponse, error)
}
```

**Service logic (`service/service.go`):**
- `GetRoute()` — Calls public OSRM API (`router.project-osrm.org`) to calculate driving route
- `CreateTrip()` — Creates trip with `bson.NewObjectID()`, status "pending", stores in repo

**Repository:** In-memory map (`map[string]*TripModel`), no persistence yet.

---

## 4. Shared Packages (`shared/`)

### `shared/types/types.go`
```go
type Coordinate struct { Latitude, Longitude float64 }
type Route struct { Distance, Duration float64; Geometry []*Geometry }
type Geometry struct { Coordinates []*Coordinate }
type OsrmApiResponse struct { Routes []struct{ Distance, Duration float64; Geometry struct{ Coordinates [][]float64 } } }
```

### `shared/contracts/amqp.go` — RabbitMQ routing keys
```
Trip events:    trip.event.created, trip.event.driver_assigned, trip.event.no_drivers_found, trip.event.driver_not_rested
Driver cmds:    driver.cmd.trip_request, driver.cmd.trip_accept, driver.cmd.trip_decline, driver.cmd.location, driver.cmd.register
Payment events: payment.event.session_created, payment.event.success, payment.event.failed, payment.event.cancelled
Payment cmds:   payment.cmd.create_session
```

### `shared/contracts/http.go`
```go
type APIResponse struct { Data any; Error *APIError }
type APIError struct { Code, Message string }
```

### `shared/contracts/ws.go`
```go
type WSMessage struct { Type string; Data any }
type WSDriverMessage struct { Type string; Data json.RawMessage }
```

### `shared/env/env.go`
- `GetString(key, fallback)`, `GetInt(key, fallback)`, `GetBool(key, fallback)`

### `shared/retry/retry.go`
- `WithBackoff(ctx, Config, operation)` — Exponential backoff retry (default: 3 retries, 1s→10s)

### `shared/util/util.go`
- `GetRandomAvatar(index)` — Returns randomuser.me lego avatar URL

---

## 5. Frontend (`web/`)

**Stack:** Next.js 15, React 19, TypeScript, Tailwind CSS, Leaflet maps, Radix UI, Stripe.js

### Pages
- `/` — Home page: choose "I Need a Ride" (rider) or "I Want to Drive" (driver)
- `/?payment=success` — Payment success confirmation

### Key Components
| Component | Role |
|---|---|
| `RiderMap.tsx` | Full rider experience: map, click-to-destination, route preview, fare selection, driver tracking |
| `DriverMap.tsx` | Full driver experience: map, location updates, trip request accept/decline |
| `DriverPackageSelector.tsx` | Driver selects car type (sedan/suv/van/luxury) before going online |
| `RiderTripOverview.tsx` | Rider sidebar: shows trip status states (looking, assigned, payment, completed, cancelled) |
| `DriverTripOverview.tsx` | Driver sidebar: shows trip request, accept/decline buttons |
| `DriverCard.tsx` | Driver info card (name, photo, car plate) |
| `DriversList.tsx` | Fare/package selection list for riders |
| `StripePaymentButton.tsx` | Stripe Checkout redirect button |
| `RoutingControl.tsx` | Leaflet polyline for route display |
| `MapClickHandler.ts` | Leaflet map click event handler |
| `PackagesMeta.tsx` | Car package metadata (sedan/suv/van/luxury with icons) |
| `TripOverviewCard.tsx` | Reusable card wrapper for trip status displays |

### WebSocket Hooks
| Hook | Connects to | Purpose |
|---|---|---|
| `useRiderStreamConnection` | `ws://.../riders?userID=X` | Receives driver locations, trip status, payment sessions |
| `useDriverStreamConnection` | `ws://.../drivers?userID=X&packageSlug=Y` | Receives trip requests, sends accept/decline |

### Frontend Events (from `contracts.ts`)
```
Server → Client:
  trip.event.created, trip.event.no_drivers_found, trip.event.driver_assigned
  driver.cmd.location, driver.cmd.trip_request, driver.cmd.register
  payment.event.session_created

Client → Server:
  driver.cmd.trip_accept, driver.cmd.trip_decline
```

### API Integration
- **Rider:** `POST /trip/preview` → gets route + rideFares → `POST /trip/start` → WebSocket events
- **Driver:** WebSocket to `/drivers` → receives trip requests → sends accept/decline
- Default location: San Francisco (37.7749, -122.4194)

---

## 6. Infrastructure

### Kubernetes (Development)
| Resource | Type | Port | Access |
|---|---|---|---|
| `api-gateway` | LoadBalancer | 8081 | External (Tilt port-forward) |
| `trip-service` | ClusterIP | 8083 | Internal only |
| `web` | (Deployment only) | 3000 | Tilt port-forward |
| `app-config` | ConfigMap | — | `ENVIRONMENT=development`, `GATEWAY_HTTP_ADDR=:8081` |

### Kubernetes (Production)
- Same structure, images from `europe-west1-docker.pkg.dev/{{PROJECT_ID}}/ride-sharing/`
- `ENVIRONMENT=production`

### Dockerfiles (Development)
All use `alpine` base, copy `shared/` and `build/` dirs, run compiled binary.

### Tiltfile
- Uses `ext://restart_process` for hot reload
- Compiles Go binaries → builds Docker images → deploys to K8s
- **Issue:** `labels` should be lists (`["compiles"]`), not strings (`"compiles"`)
- **Issue:** Trip service has no `port_forwards` (needs `kubectl port-forward` to access)
- **Issue:** Web Dockerfile builds from root context (not `web/` only)

---

## 7. Tooling

### `tools/create_service.go`
CLI scaffold tool for new services:
```bash
go run tools/create_service.go -name payment
```
Creates: `services/payment-service/` with Clean Architecture directory structure.

### `Makefile`
```bash
make generate-proto   # Generates Go code from proto/*.proto files
```

---

## 8. Dependencies (from go.mod)

| Dependency | Purpose |
|---|---|
| `google.golang.org/grpc` | gRPC framework |
| `google.golang.org/protobuf` | Protocol Buffers |
| `go.mongodb.org/mongo-driver/v2` | MongoDB driver (v2 — uses `bson.ObjectID`, not `primitive.ObjectID`) |
| `go.opentelemetry.io/otel` + Jaeger | Distributed tracing |
| `github.com/stripe/stripe-go/v81` | Stripe payments |
| `github.com/rabbitmq/amqp091-go` | RabbitMQ AMQP client |
| `github.com/gorilla/websocket` | WebSocket support |
| `github.com/mmcloughlin/geohash` | Geohash encoding for driver location indexing |
| `github.com/google/uuid` | UUID generation |

---

## 9. Known Issues & TODOs

### Bugs
1. **API Gateway `http.go` line 18:** Validation inverted — `reqBody.UserID != ""` should be `== ""`
2. **Trip service HTTP handler:** Logs error from `GetRoute` but still returns 200 OK with nil data instead of 500
3. **API Gateway:** Stubbed — doesn't actually call trip-service, just returns "ok"

### Missing Implementation (expected — this is a skeleton project)
- **WebSocket handlers** (rider/driver WS endpoints) — empty `events/` and `grpc/` dirs
- **Trip persistence** — only in-memory repository, no MongoDB integration
- **Driver matching logic** — no geohash-based driver lookup
- **Payment flow** — Stripe integration in frontend but no backend payment service
- **Trip start endpoint** — `/trip/start` not implemented
- **gRPC server** — empty `grpc/` directory
- **Proto files** — `Makefile` references `proto/` dir but it doesn't exist yet

### Infrastructure
- Trip service ClusterIP has no `port_forwards` in Tiltfile (must `kubectl port-forward` manually)
- `labels` in Tiltfile are strings instead of lists
- Web Dockerfile context may not work correctly from root build context

---

## 10. Data Flow (Intended)

```
User (Browser)
  │
  ├─ Rider Flow:
  │   1. Click map → POST /trip/preview (via API Gateway → Trip Service → OSRM)
  │   2. Select fare → POST /trip/start (via API Gateway → Trip Service)
  │   3. WebSocket /riders ← receives: trip.event.created, driver.cmd.location, trip.event.driver_assigned
  │   4. Payment session → Stripe Checkout redirect
  │
  └─ Driver Flow:
      1. Select car package → WebSocket /drivers (registers driver with geohash)
      2. Receives: driver.cmd.trip_request
      3. Sends: driver.cmd.trip_accept or driver.cmd.trip_decline
      4. Route displayed on map
```

---

## 11. Conventions

- **Go module:** Single module `ride-sharing` for all services (not per-service modules)
- **Clean Architecture:** Domain interfaces → Service logic → Infrastructure implementations
- **MongoDB driver v2:** Use `bson.ObjectID` (not `primitive.ObjectID` from v1)
- **Go 1.22+ routing:** `mux.HandleFunc("POST /path", handler)` pattern syntax
- **Frontend types:** Contracts defined in `web/src/contracts.ts` and `web/src/types.ts`
- **Event naming:** `entity.action` pattern (e.g., `trip.event.created`, `driver.cmd.location`)
