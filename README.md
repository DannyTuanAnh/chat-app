# Real-time Chat Backend System

> A learning project for exploring how to design and implement a real-time chat backend system using **Go** and **microservices architecture**.

---

## Overview

The project focuses on several core backend engineering problems:

- Real-time communication with WebSocket
- Multi-device connection management
- Service-to-service communication via gRPC
- Authentication and authorization
- Secure internal communication using mTLS
- RBAC for microservice communication
- Redis Pub/Sub for scalable WebSocket servers
- Push notifications for offline devices using Firebase Cloud Messaging
- Non-blocking WebSocket writes for slow connections

> [!NOTE]
> This is primarily a **learning project**. The goal is to understand and apply core backend and distributed-system concepts rather than build a fully production-ready system. Advanced concerns such as large-scale distributed coordination, sophisticated message delivery guarantees, observability, and fault-tolerant infrastructure are intentionally outside the scope of this project.

---

## Architecture

The application is designed around a **microservice architecture**, where different business domains are separated into independent services.

```
                         ┌─────────────────┐
                         │   Android App   │
                         └────────┬────────┘
                                  │
                         REST / WebSocket
                                  │
                                  ▼
                         ┌─────────────────┐
                         │   API Gateway   │
                         └───────┬─────────┘
                                 │
                  ┌──────────────┼──────────────┐
                  │              │              │
                gRPC           gRPC           gRPC
                  │              │              │
                  ▼              ▼              ▼
             ┌─────────┐   ┌─────────┐   ┌─────────┐
             │  Auth   │   │  User   │   │  Friend │
             │ Service │   │ Service │   │ Service │
             └─────────┘   └─────────┘   └─────────┘
                                  │
                                  ▼
                            ┌───────────┐
                            │   Chat    │
                            │  Service  │
                            └─────┬─────┘
                                  │
                            Redis Pub/Sub
                                  │
                    ┌─────────────┴─────────────┐
                    ▼                           ▼
             ┌─────────────┐             ┌─────────────┐
             │ WebSocket 1 │             │ WebSocket 2 │
             └─────────────┘             └─────────────┘
                    │                           │
                    ▼                           ▼
                Connected                   Connected
                 Devices                     Devices

                                  │ Offline device
                                  ▼
                         ┌─────────────────┐
                         │ Notify Service  │
                         │      (FCM)      │
                         └─────────────────┘
```

The architecture **separates business services from the real-time connection layer**.

The WebSocket layer is designed so that multiple server instances can run simultaneously. **Redis Pub/Sub** is used to propagate real-time events between instances instead of relying on direct server-to-server communication.

---

## Technologies

### Backend

| Technology      | Role                                      |
| --------------- | ----------------------------------------- |
| **Go**          | Primary language                          |
| **Gin**         | HTTP / REST API framework                 |
| **gRPC**        | Internal service-to-service communication |
| **RESTful API** | Client-facing HTTP API                    |
| **WebSocket**   | Real-time bidirectional communication     |

### Data & Messaging

| Technology        | Role                                          |
| ----------------- | --------------------------------------------- |
| **PostgreSQL**    | Persistent relational data                    |
| **Redis**         | Caching and connection-related data           |
| **Redis Pub/Sub** | Real-time event propagation between instances |

### Cloud & Infrastructure

| Technology                   | Role                                   |
| ---------------------------- | -------------------------------------- |
| **Google Cloud Platform**    | Cloud provider                         |
| **Google Cloud Storage**     | Object storage for user profile images |
| **Cloud Functions**          | Event-driven image processing          |
| **Google Cloud Vision API**  | Automated image content analysis       |
| **Firebase Cloud Messaging** | Push notifications for Android devices |

### Security

| Technology | Role                                    |
| ---------- | --------------------------------------- |
| **mTLS**   | Secure service-to-service communication |
| **RBAC**   | Authorization between internal services |

---

## Key Engineering Problems

### 1. Non-blocking WebSocket Write Architecture

#### The Problem

A straightforward implementation sends messages directly inside a loop:

```go
for _, client := range recipients {
    client.Conn.Write(ctx, message)
}
```

Although simple, the writes are **sequential**. If one client has a slow network connection, its `Write()` operation blocks all following clients:

```
Client 1 ── 0ms
Client 2 ── 100ms  ← slow connection blocks everything
Client 3 ── waiting...
Client 4 ── waiting...
```

**Total time: ~100ms** — even though Clients 3 and 4 are on fast connections.

#### The Solution — Per-client Write Pumps

Instead of writing directly inside the broadcast loop, the manager only dispatches messages to each client's **buffered channel**:

```go
for _, client := range recipients {
    select {
    case client.SendChan <- message:
        // message dispatched
    case <-client.Ctx.Done():
        // client disconnected
    }
}
```

Each client has its own **WritePump** goroutine that handles socket I/O independently:

```
                    WebSocket Manager
                           │
             ┌─────────────┼─────────────┐
             │             │             │
             ▼             ▼             ▼
          Client A      Client B      Client C
          SendChan      SendChan      SendChan
             │             │             │
             ▼             ▼             ▼
        WritePump A   WritePump B   WritePump C
             │             │             │
             ▼             ▼             ▼
          Socket A      Socket B      Socket C
```

This separates **message dispatch** from **socket I/O** — a slow socket is isolated to its own WritePump.

#### Benchmark Results

**Test A — Single Slow Client** (`Client 49 → fast`, `Client 50 → slow 100ms`, `Client 51 → fast`)

| Metric    | Old Architecture     | New Architecture |
| --------- | -------------------- | ---------------- |
| Client 49 | 0ms                  | 0ms              |
| Client 50 | ~100ms               | ~100ms           |
| Client 51 | **~100ms** (blocked) | **~0ms**         |

**Test B — 10 fast clients + 10 slow clients (100ms each)**

| Metric                    | Old Architecture | New Architecture |
| ------------------------- | ---------------- | ---------------- |
| Dispatch time             | ~1.005s          | ~0ms             |
| Total test time           | ~1.005s          | ~100ms           |
| fast[0]                   | 0ms              | 0ms              |
| fast[5]                   | ~503ms           | 0ms              |
| fast[9]                   | ~905ms           | 0ms              |
| Slow client blocks others | ✅ Yes           | ❌ No            |

> [!IMPORTANT]
> The key improvement is not just lower benchmark time — it is that **the latency of one WebSocket connection no longer propagates to unrelated connections**.

---

### 2. Multi-device Connection Management

A user can connect from **multiple devices simultaneously**. The WebSocket connection manager maintains connections using a nested structure:

```
map[UserID]map[DeviceID]*Client
```

Conceptually:

```
User 1001
├── phone   → Client
├── laptop  → Client
└── tablet  → Client
```

This allows the server to:

- Send a message to **all devices** of a user
- Send to **specific devices** while excluding the originating device
- Determine if a user is truly **offline** (only when the last connected device disconnects)

```
phone    connected
laptop   disconnected
tablet   connected
─────────────────────
→ User is still online
```

---

### 3. Scalable WebSocket Servers with Redis Pub/Sub

Running a single WebSocket server creates a scaling bottleneck. When scaled horizontally, users on different server instances cannot directly communicate:

```
              ┌── WebSocket Server 1  (User A)
              │
Client ───────┼── WebSocket Server 2  (User B)
              │
              └── WebSocket Server 3  (User C)
```

**Redis Pub/Sub** acts as an event distribution layer so any service can publish an event and all WebSocket instances receive it:

```
                 Redis Pub/Sub
                 /     |     \
                ▼       ▼       ▼
             WS-1    WS-2    WS-3
              │       │       │
              ▼       ▼       ▼
           Clients Clients Clients
```

WebSocket servers can be scaled **horizontally** without requiring direct server-to-server calls.

> [!NOTE]
> This design addresses the core scaling problem of multiple WebSocket instances. It does not attempt to solve every distributed-system concern such as guaranteed event delivery, durable event streaming, or exactly-once processing.

---

### 4. mTLS for Service-to-Service Security

Internal service communication uses **Mutual TLS (mTLS)**. Unlike regular TLS, both sides authenticate each other using certificates:

```
┌───────────────┐                    ┌───────────────┐
│  API Gateway  │ ◄── TLS + certs ──► │ Auth Service  │
└───────────────┘                    └───────────────┘
```

Each connection provides:

- **Encryption** — data in transit is secure
- **Server authentication** — client verifies server identity
- **Client authentication** — server verifies client identity
- **Service identity** — services cannot impersonate each other

This prevents services from blindly trusting any client that can reach the internal network endpoint.

---

### 5. RBAC for Internal Service Authorization

mTLS answers **"Who are you?"** — but not **"What are you allowed to do?"**

The project also applies **Role-Based Access Control (RBAC)** to internal service communication:

```
Service Identity
      │
      ▼
 Authentication  ──  mTLS
      │
      ▼
 Authorization   ──  RBAC
      │
      ▼
  Allowed RPC Method
```

A service may be authenticated successfully through mTLS but still be **denied access** to a specific RPC method if its role does not have the required permission.

| Layer    | Question                        |
| -------- | ------------------------------- |
| **mTLS** | _"Who are you?"_                |
| **RBAC** | _"What are you allowed to do?"_ |

This separation is important because authenticating a service does not mean it should have unrestricted access to every other service.

---

### 6. Automated Profile Image Moderation

User profile images are uploaded to **Google Cloud Storage**. Instead of relying entirely on manual moderation, the pipeline uses an event-driven function to automatically analyze new images:

```
Backend System
      │
      │  Upload profile image
      ▼
Google Cloud Storage
      │
      │  Object creation event
      ▼
Cloud Function
      │
      ▼
Google Cloud Vision API
      │
      ▼
Image safety / content analysis
      │
      ├── ✅ Accept
      │
      └── ❌ Reject / flag
```

The important architectural idea is **decoupling image processing from the main request flow**:

```
User upload  →  Storage  →  Async Processing  →  Content Analysis
```

Rather than making the API server perform every step synchronously.

> [!NOTE]
> This is an automated moderation mechanism based on an external vision/content-analysis service. It is not a guarantee that every image violating a community standard will be detected correctly.

---

### 7. Firebase Cloud Messaging for Offline Devices

WebSocket is suitable for real-time communication while the app is active. However, a mobile device may:

- Close the application
- Lose its WebSocket connection
- Lose network connectivity
- Be suspended by the operating system

When a device is no longer connected through WebSocket, the **Notify Service** falls back to **Firebase Cloud Messaging (FCM)**:

```
             New Message
                 │
                 ▼
          WebSocket Layer
                 │
     ┌───────────┴───────────┐
     │                       │
Device connected         Device offline
     │                       │
     ▼                       ▼
 WebSocket              Notify Service
                              │
                              ▼
                             FCM
                              │
                              ▼
                       Android Device
```

This provides a **separate delivery path** for devices not currently connected to the WebSocket server.

---

## Project Scope & Learning Objectives

This project focuses on **learning and implementing** core backend engineering concepts in practice:

**Service Design**

- Designing services around business domains
- RESTful API design
- gRPC service-to-service communication

**Real-time & Connection Management**

- WebSocket connection management
- Multi-device session management
- Avoiding head-of-line blocking in real-time message delivery
- Designing a WebSocket layer that can scale horizontally

**Data & Messaging**

- Redis caching and Pub/Sub
- PostgreSQL database design

**Security & Authorization**

- Service-to-service authentication with mTLS
- Service authorization with RBAC
- Authentication and session management

**Cloud & Async Processing**

- Event-driven processing with Cloud Functions
- Automated image content moderation
- Push notifications with Firebase Cloud Messaging

**Go & Concurrency**

- Concurrency and goroutine-based processing in Go

> [!IMPORTANT]
> This project does **not** attempt to provide a complete production-grade implementation of every distributed-system problem.
>
> The primary goal is to understand **why** these architectural decisions are necessary, **what problem** each one solves, and **what trade-offs** they introduce. The project intentionally focuses on solving a **limited set of core backend problems** rather than attempting to implement every production-level concern found in large-scale messaging platforms.
