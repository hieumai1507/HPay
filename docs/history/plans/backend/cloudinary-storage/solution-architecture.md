[← implementation-plan](implementation-plan.md) · cloudinary-storage

# Cloudinary Cloud Storage — Solution Architecture

## 1. Context — Position of the Cloudinary driver in the system

Cloudinary is integrated through the `storage.Store` interface, making the application fully independent of the physical storage infrastructure.

```mermaid
flowchart TD
  subgraph Client
    mob["📱 Mobile App\n(KYC submit)"]
    adm["🖥 Admin Panel\n(View KYC images)"]
  end

  subgraph API Gateway
    router["router.go"]
    kyc_h["kyc_handler.go"]
    admin_h["admin_handler.go"]
  end

  subgraph Abstraction Layer
    store_i["storage.Store (Interface)\nPut(key, bytes) · Get(key)"]
  end

  subgraph Drivers
    local["storage.Local\n(Stores on local disk)"]
    cl["storage.Cloudinary\n(Signed upload + HTTP Get)"]
  end

  subgraph External Cloud
    cld_api["☁️ Cloudinary Storage"]
  end

  mob -->|Upload| kyc_h
  adm -->|View image via HMAC URL| admin_h
  kyc_h --> store_i
  admin_h --> store_i
  
  router -->|Initialized based on env| store_i
  store_i -.->|Implement| local
  store_i -.->|Implement| cl

  local -->|Read/Write| disk[("Local Folder\n./data/kyc")]
  cl -->|REST API POST/GET| cld_api
```

## 2. Main flow — Image submission & viewing flow

### A. KYC submission flow (Put)

```mermaid
sequenceDiagram
  autonumber
  actor User as Mobile App
  participant API as internal/kyc
  participant ST as storage.Cloudinary
  participant CLD as Cloudinary REST API
  
  User->>API: Submit KYC profile (2 images)
  API->>ST: Put(ctx, "req-1/id_card.jpg", bytes)
  ST->>ST: Generate timestamp & SHA-1 signature using the API Secret
  ST->>CLD: Multipart POST (file + signature + api_key)
  CLD-->>ST: 200 OK (Upload succeeded)
  ST-->>API: Return nil error
  API-->>User: 202 Accepted (Transaction is being processed)
```

### B. Admin image viewing flow (Get)

```mermaid
sequenceDiagram
  autonumber
  actor Admin as Admin SPA
  participant API as internal/api (ServeFile)
  participant ST as storage.Cloudinary
  participant CLD as Cloudinary CDN
  
  Admin->>API: GET /v1/files/kyc/req-1/id_card.jpg?exp=...&sig=... (HMAC URL)
  API->>API: Verify HMAC signature & expiry time (5 minutes)
  API->>ST: Get(ctx, "req-1/id_card.jpg")
  ST->>CLD: HTTP GET https://res.cloudinary.com/cloud/image/upload/req-1/id_card.jpg
  CLD-->>ST: Return image binary data
  ST-->>API: Return []byte
  API-->>Admin: Return image binary data with the corresponding Content-Type
```

## 3. Decisions & trade-offs

| Decision | Alternative | Reason & Benefits |
| --- | --- | --- |
| **Proxy images through the backend** in the `Get()` function | Return the direct Cloudinary URL to the client for display. | - **Security and privacy:** KYC images contain sensitive user information (PII). If direct Cloudinary links were used, outsiders could guess the URL or share the link permanently. Proxying through the backend keeps the 5-minute HMAC Signed URL protection in place.<br> - **Independence:** The client does not need to know whether the image lives on Cloudinary or on a local disk, preserving the system's encapsulation. |
| Use the **plain Go HTTP client** instead of installing the official Cloudinary SDK | Run `go get github.com/cloudinary/cloudinary-go/v2` | - **Minimal dependencies:** Reduces the application build size and avoids security risks from transitive sub-packages.<br> - **High reliability:** The Cloudinary APIs are very stable; calling the REST API through a multipart form takes very few lines of code and is easy to debug/mock for testing. |

## 4. Invariants touched (Security & fintech standards)

- **PII protection (sensitive personal information):** Never expose the direct Cloudinary URL externally. All image viewing must pass a secure HMAC signature check within 5 minutes.

- **Infrastructure independence:** The storage key in the DB is always a relative key (for example: `req-123/selfie.jpg`) rather than an absolute URL, making future data migration between cloud providers extremely simple without running an SQL migration.
