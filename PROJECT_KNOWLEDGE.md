# RideSharing Microservices Project — Complete Reference

> Read this file to understand the full project architecture, services, data flow, and conventions.

---

## 1. Overview

A **ride-sharing platform** (Uber/Lyft clone) built with:
- **Go** backend microservices (single module `ride-sharing`)
- **Next.js 15** (React 19) web frontend
- **Kubernetes** orchestration via **Tilt** for local dev
- **RabbitMQ** for async event-driven communication (AMQP)
- **gRPC + Protobuf** for inter-service RPC
- **MongoDB** for persistence (driver v2)
- **Stripe** for payments
- **OpenTelemetry + Jaeger** for distributed tracing
- **OSRM** (public API) for route/distance calculation

---

## 2. Project Structure

```
microservices-go/
├── go.mod                          # Single Go module: "ride-sharing" (Go 1.25)
├── Makefile                        # Proto generation only
├── Tiltfile                        # Tilt config for local K8s dev (hot reload)
├── build/                          # Compiled Go binaries (gitignored)
├── docs/architecture/              # Architecture decision docs
├── infra/
│   ├── development/
│   │   ├── docker/                 # Dockerfiles for each service
│   │   └── k8s/                    # K8s manifests (Deployments, ConfigMap, Secrets)
│   └── production/
│       ├── docker/                 # Multi-stage Dockerfiles (api-gateway, trip-service only)
│       └── k8s/                    # Prod K8s manifests (partial)
├── services/
│   ├── api-gateway/                # HTTP/WebSocket entry point
│   ├── trip-service/               # Trip domain (Clean Architecture)
│   ├── driver-service/             # Driver management & location tracking
│   └── payment-service/            # Stripe payment processing (Clean Architecture)
├── shared/                         # Shared Go packages
│   ├── contracts/                  # AMQP routing keys, HTTP/WS response types
│   ├── db/                         # MongoDB connection helper
│   ├── env/                        # Environment variable helpers
│   ├── messaging/                  # RabbitMQ: connection, publishing, consuming
│   ├── proto/                      # Generated protobuf Go code
│   │   ├── driver/
│   │   └── trip/
│   ├── retry/                      # Exponential backoff retry
│   ├── tracing/                    # OpenTelemetry setup (HTTP + RabbitMQ)
│   ├── types/                      # Shared domain types (Coordinate, Route, OsrmApiResponse)
│   └── util/                       # Misc utilities (avatar URLs)
├── tools/
│   └── create_service.go           # CLI scaffold tool
└── web/                            # Next.js 15 frontend
    └── src/
        ├── app/                    # App Router pages
        ├── components/             # React components (maps, trip, driver, Stripe)
        ├── hooks/                  # WebSocket hooks (rider + driver)
        ├── contracts.ts            # Frontend API contract types & event enums
        ├── types.ts                # Frontend domain types
        └── constants.ts            # API_URL, WEBSOCKET_URL
```

---

## 3. Services

### 3.1 API Gateway (`services/api-gateway/`)

**Role:** Single entry point for all frontend HTTP/WebSocket requests. Routes to backend services via gRPC and WebSocket proxying.

| Detail | Value |
|---|---|
| Port | `:8081` (configurable via `GATEWAY_HTTP_ADDR`) |
| K8s Service type | **LoadBalancer** |
| Framework | `net/http` (stdlib) |

**Files:**
- `main.go` — Server startup, route registration, gRPC client connections
- `http.go` — HTTP handlers (trip preview, trip start, driver location update)
- `ws.go` — WebSocket handlers for rider and driver streams
- `middleware.go` — CORS middleware
- `types.go` — Request/response types
- `json.go` — JSON response helper
- `grpc_clients/trip_client.go` — gRPC client for trip-service
- `grpc_clients/driver_client.go` — gRPC client for driver-service

**Endpoints:**
| Endpoint | Method | Description |
|---|---|---|
| `/trip/preview` | POST | Preview trip route & fares (calls trip-service via gRPC) |
| `/trip/start` | POST | Start a trip (calls trip-service via gRPC) |
| `/ws/riders` | WebSocket | Rider event stream (trip updates, driver locations, payment) |
| `/ws/drivers` | WebSocket | Driver event stream (trip requests, location updates) |
| `/webhook/stripe` | POST | Stripe webhook receiver |

**gRPC Clients:**
- Connects to trip-service on `TRIP_SERVICE_GRPC_ADDR` (default `localhost:9093`)
- Connects to driver-service on `DRIVER_SERVICE_GRPC_ADDR` (default `localhost:9092`)

---

### 3.2 Trip Service (`services/trip-service/`)

**Role:** Core trip domain — route calculation, trip creation/management, fare calculation, event publishing.

| Detail | Value |
|---|---|
| gRPC Port | `:9093` (no HTTP server) |
| K8s Service type | **ClusterIP** |
| Architecture | Clean Architecture (Domain → Service → Infrastructure) |

**Structure:**
```
trip-service/
├── cmd/main.go                              # Entry point: wires all layers
├── internal/
│   ├── domain/
│   │   ├── trip.go                          # TripModel, TripRepository interface
│   │   └── ride_fare.go                     # RideFareModel, fare calculation
│   ├── service/service.go                   # Business logic (CreateTrip, PreviewTrip, GetRoute)
│   ├── grpc/grpc_handler.go                 # gRPC server handlers
│   └── infrastructure/
│       ├── http/http.go                     # OSRM API client
│       ├── repository/
│       │   ├── mongodb.go                   # MongoDB trip repository
│       │   └── inmem.go                     # In-memory trip repository (fallback)
│       └── events/
│           ├── trip_publisher.go             # Publishes trip events to RabbitMQ
│           ├── driver_consumer.go            # Consumes driver events (driver assigned/not found)
│           └── payment_consumer.go           # Consumes payment events (payment success/failure)
├── pkg/types/types.go                       # Shared request/response types
└── README.md
```

**Endpoints:**
| Endpoint | Method | Description |
|---|---|---|
| `/preview` | POST | Get route from OSRM for pickup→destination |
| gRPC `PreviewTrip` | RPC | Same as above, via gRPC |
| gRPC `CreateTrip` | RPC | Create a new trip |

**Domain Models:**
```go
type TripModel struct {
    ID          bson.ObjectID  `bson:"_id,omitempty"`
    UserID      string
    Pickup      Coordinate
    Destination Coordinate
    Status      string          // "pending", "driver_assigned", "in_progress", "completed", "cancelled"
    RideFare    *RideFareModel
}

type RideFareModel struct {
    Distance float64
    Duration float64
    Fares    []FareOption      // sedan, suv, van, luxury pricing
}
```

**Key Flows:**
1. `PreviewTrip` → calls OSRM API → calculates fares → returns route + pricing
2. `CreateTrip` → stores in MongoDB → publishes `trip.event.created` → consumes driver assignment events
3. `driver_consumer.go` — listens for `driver.cmd.trip_request`, matches nearest drivers via geohash
4. `payment_consumer.go` — listens for `payment.event.session_created`, updates trip status

---

### 3.3 Driver Service (`services/driver-service/`)

**Role:** Driver registration, location tracking, trip request handling via gRPC and RabbitMQ.

| Detail | Value |
|---|---|
| gRPC Port | `:9092` |
| K8s Service type | **ClusterIP** |
| Architecture | Flat (no Clean Architecture) |

**Files:**
- `main.go` — Server startup, gRPC + RabbitMQ setup
- `grpc_handler.go` — gRPC handlers (RegisterDriver, UnregisterDriver)
- `sevice.go` — Driver business logic
- `trip_consumer.go` — Consumes trip events, finds nearby drivers, sends trip requests
- `utils.go` — Geohash-based driver location indexing

**Key Logic:**
- Drivers register with their location (latitude/longitude) and package type
- Driver locations stored in-memory with geohash indexing for proximity queries
- When a trip is created, `trip_consumer.go` finds drivers within a geohash radius
- Sends `driver.cmd.trip_request` to matching drivers via RabbitMQ
- Waits for `driver.cmd.trip_accept` or `driver.cmd.trip_decline`

**Driver Location Tracking:**
- Uses `mmcloughlin/geohash` for spatial indexing
- `utils.go` provides `FindNearbyDrivers(lat, lng, radius)` using geohash neighbor expansion
- Drivers send periodic location updates via gRPC `UpdateLocation`

---

### 3.4 Payment Service (`services/payment-service/`)

**Role:** Stripe payment session creation and webhook handling.

| Detail | Value |
|---|---|
| Architecture | Clean Architecture (Domain → Service → Infrastructure) |
| Transport | RabbitMQ consumer only (no HTTP/gRPC server) |

**Structure:**
```
payment-service/
├── cmd/main.go                              # Entry point
├── internal/
│   ├── domain/domain.go                     # Payment domain types
│   ├── service/service.go                   # Payment business logic
│   └── infrastructure/
│       ├── stripe/stripe.go                 # Stripe API client
│       └── events/trip_consumer.go          # Consumes trip events, creates payment sessions
├── pkg/types/types.go                       # Shared types
```

**Key Flows:**
1. Consumes `trip.event.driver_assigned` from RabbitMQ
2. Creates Stripe Checkout session for the trip fare
3. Publishes `payment.event.session_created` with the checkout URL
4. API Gateway receives the event and sends it to the rider via WebSocket
5. Rider completes payment on Stripe → Stripe webhook → `payment.event.success`

---

## 4. Shared Packages (`shared/`)

### `shared/messaging/` — RabbitMQ Abstraction
| File | Purpose |
|---|---|
| `connection_manager.go` | Singleton connection/channel management, reconnection logic |
| `rabbitmq.go` | `AMQPClient` struct: publish, consume, declare queues/exchanges |
| `events.go` | Event type definitions (TripEvent, DriverEvent, PaymentEvent) |
| `queue_consumer.go` | Generic queue consumer with automatic ack/nack |

**Pattern:** All services use `messaging.NewAMQPClient(url)` to get a shared connection. Each service declares its own queues and exchanges. Messages are JSON-serialized event structs.

### `shared/contracts/` — API Contracts
- `amqp.go` — All RabbitMQ routing keys (trip.*, driver.cmd.*, payment.*)
- `http.go` — `APIResponse{Data, Error}` wrapper
- `ws.go` — `WSMessage{Type, Data}` for WebSocket messages

### `shared/db/mongodb.go`
- `ConnectMongoDB(uri)` — returns `*mongo.Database`
- Uses MongoDB driver v2 (`go.mongodb.org/mongo-driver/v2`)

### `shared/env/env.go`
- `GetString(key, fallback)`, `GetInt(key, fallback)`, `GetBool(key, fallback)`

### `shared/retry/retry.go`
- `WithBackoff(ctx, Config, operation)` — exponential backoff (default: 3 retries, 1s→10s)

### `shared/tracing/`
- `tracing.go` — OpenTelemetry + Jaeger tracer provider setup
- `http.go` — HTTP client middleware for trace propagation
- `rabbitmq.go` — RabbitMQ publisher/consumer trace propagation

### `shared/proto/`
- Generated from `proto/trip.proto` and `proto/driver.proto`
- `trip/` — TripService gRPC (PreviewTrip, CreateTrip)
- `driver/` — DriverService gRPC (RegisterDriver, UnregisterDriver)

---

## 5. Frontend (`web/`)

**Stack:** Next.js 15, React 19, TypeScript, Tailwind CSS, Leaflet maps, Radix UI, Stripe.js

### Pages
- `/` — Home: choose "I Need a Ride" (rider) or "I Want to Drive" (driver)
- `/?payment=success` — Payment success confirmation

### Key Components
| Component | Role |
|---|---|
| `RiderMap.tsx` | Full rider experience: map, click-to-destination, route preview, fare selection, driver tracking |
| `DriverMap.tsx` | Full driver experience: map, location updates, trip request accept/decline |
| `DriverPackageSelector.tsx` | Driver selects car type (sedan/suv/van/luxury) before going online |
| `RiderTripOverview.tsx` | Rider sidebar: trip status states (looking, assigned, payment, completed, cancelled) |
| `DriverTripOverview.tsx` | Driver sidebar: trip request, accept/decline buttons |
| `DriverCard.tsx` | Driver info card (name, photo, car plate) |
| `DriversList.tsx` | Fare/package selection list for riders |
| `StripePaymentButton.tsx` | Stripe Checkout redirect button |
| `RoutingControl.tsx` | Leaflet polyline for route display |
| `MapClickHandler.ts` | Leaflet map click → set destination |
| `PackagesMeta.tsx` | Car package metadata (sedan/suv/van/luxury with icons/pricing) |
| `TripOverviewCard.tsx` | Reusable card wrapper for trip status displays |

### WebSocket Hooks
| Hook | Connects to | Purpose |
|---|---|---|
| `useRiderStreamConnection` | `ws://.../riders?userID=X` | Receives driver locations, trip status, payment sessions |
| `useDriverStreamConnection` | `ws://.../drivers?userID=X&packageSlug=Y` | Receives trip requests, sends accept/decline |

### Frontend Event Types (`contracts.ts`)
```
Server → Client:
  trip.event.created, trip.event.no_drivers_found, trip.event.driver_assigned
  driver.cmd.location, driver.cmd.trip_request, driver.cmd.register
  payment.event.session_created

Client → Server:
  driver.cmd.trip_accept, driver.cmd.trip_decline
```

---

## 6. Infrastructure

### Kubernetes (Development)
| Resource | Type | Port | Access |
|---|---|---|---|
| `api-gateway` | LoadBalancer | 8081 | External (Tilt port-forward) |
| `trip-service` | ClusterIP | 9093 | Internal only |
| `driver-service` | ClusterIP | 9092 | Internal only |
| `payment-service` | ClusterIP | — | Internal only (RabbitMQ consumer) |
| `web` | Deployment | 3000 | Tilt port-forward |
| `rabbitmq` | StatefulSet | 5672/15672 | Tilt port-forward |
| `jaeger` | Deployment | 16686/14268 | Tilt port-forward |
| `app-config` | ConfigMap | — | `ENVIRONMENT=development` |
| `secrets` | Secret | — | MongoDB URI, Stripe key, etc. |

### Kubernetes (Production)
- Images from `europe-west1-docker.pkg.dev/{{PROJECT_ID}}/ride-sharing/`
- Only api-gateway and trip-service have production K8s manifests and Dockerfiles
- `ENVIRONMENT=production`

### Tiltfile
- Compiles Go binaries → builds Docker images → deploys to K8s
- Uses `ext://restart_process` for hot reload
- Port forwards: api-gateway=8081, web=3000, rabbitmq=5672/15672, jaeger=16686/14268
- Windows `.bat` build scripts included

---

## 7. Data Flow

### Rider Flow
```
1. User clicks map → sets pickup + destination
2. POST /trip/preview (API Gateway → gRPC → Trip Service → OSRM API)
   → Returns route geometry + fare options (sedan/suv/van/luxury)
3. User selects fare → POST /trip/start (API Gateway → gRPC → Trip Service)
   → Trip created in MongoDB (status: "pending")
   → Trip Service publishes trip.event.created to RabbitMQ
4. WebSocket /riders ← receives:
   - trip.event.created (trip confirmation)
   - driver.cmd.trip_request (driver nearby found)
   - trip.event.driver_assigned (driver accepted)
   - driver.cmd.location (real-time driver position)
   - payment.event.session_created (Stripe checkout URL)
5. User completes payment on Stripe → redirected to /?payment=success
```

### Driver Flow
```
1. Driver selects car package → WebSocket /drivers connects
   → Driver registered via gRPC (location + package type)
2. Driver location updates sent periodically via gRPC UpdateLocation
3. When trip created, trip-service publishes trip.event.created
   → driver-service consumes, finds nearby drivers via geohash
   → Sends driver.cmd.trip_request to matching drivers
4. Driver sees trip request on map → accepts or declines
   → Sends driver.cmd.trip_accept or driver.cmd.trip_decline
5. If accepted: route displayed, driver navigates to pickup
```

### Payment Flow
```
1. Trip Service publishes trip.event.driver_assigned
   → payment-service consumes
2. payment-service creates Stripe Checkout session
3. Publishes payment.event.session_created with checkout URL
4. Trip Service consumes → publishes to rider via WebSocket
5. Rider clicks "Pay Now" → Stripe Checkout page
6. Stripe webhook → payment-service → payment.event.success
7. Trip Service consumes → updates trip status to "completed"
```

---

## 8. Tooling

### `tools/create_service.go`
```bash
go run tools/create_service.go -name payment
```
Creates `services/payment-service/` with Clean Architecture scaffolding.

### `Makefile`
```bash
make generate-proto   # Generates Go code from proto/*.proto
```

---

## 9. Dependencies (go.mod)

| Dependency | Purpose |
|---|---|
| `google.golang.org/grpc` | gRPC framework |
| `google.golang.org/protobuf` | Protocol Buffers |
| `go.mongodb.org/mongo-driver/v2` | MongoDB driver v2 |
| `go.opentelemetry.io/otel` + Jaeger | Distributed tracing |
| `github.com/stripe/stripe-go/v81` | Stripe payments |
| `github.com/rabbitmq/amqp091-go` | RabbitMQ AMQP client |
| `github.com/gorilla/websocket` | WebSocket support |
| `github.com/mmcloughlin/geohash` | Geohash for driver location indexing |
| `github.com/google/uuid` | UUID generation |

---

## 10. Conventions

- **Single Go module** (`ride-sharing`) for all services — not per-service modules
- **Clean Architecture** in trip-service and payment-service (Domain → Service → Infrastructure)
- **Flat structure** in api-gateway and driver-service
- **MongoDB driver v2**: use `bson.ObjectID` and `bson.NewObjectID()` from `go.mongodb.org/mongo-driver/v2/bson`
- **Go 1.22+ routing**: `mux.HandleFunc("POST /path", handler)` method-prefixed patterns
- **Event naming**: `entity.action` pattern (e.g., `trip.event.created`, `driver.cmd.location`)
- **Frontend types**: contracts in `web/src/contracts.ts`, domain types in `web/src/types.ts`
- **gRPC**: proto definitions in `proto/`, generated code in `shared/proto/`
- **Config**: environment variables via K8s ConfigMap + Secrets, read with `shared/env`
