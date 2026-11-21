# 🏗 Solution Architecture — M2 Auth & KYC

## 1. Context — where M2 sits and navigation in the app

```mermaid
flowchart TD
  subgraph APP["hpay-mobile (M2)"]
    subgraph ROUTER["expo-router navigation stack"]
      ob["(auth)/onboarding\nApp introduction"] --> lg["(auth)/login\nLogin form"]
      ob --> rg["(auth)/register\nRegistration form"]
      rg --> otp["(auth)/otp\n6-digit code verification"]

      intro["(kyc)/intro\nKYC instructions"] --> cap_id["(kyc)/capture-id\nCapture CCCD (rectangular frame)"]
      cap_id --> cap_selfie["(kyc)/capture-selfie\nCapture portrait (oval frame)"]
      cap_selfie --> upload["Upload kyc/submit"]
      upload --> st["(kyc)/status\nApplication status"]
    end

    subgraph STATE["State & Store Management"]
      ss["stores/session\naccessToken, user profile"]
      sec["SecureStore\nrefresh_token, kyc_drafts"]
      dev["stores/device\ndeviceId"]
    end

    subgraph SEC_FUNC["Client Security Services"]
      idle["IdleMonitor\nAppState check & lock timer"] --> bio["LocalBiometrics\nexpo-local-authentication"]
      privacy["PrivacyOverlay\nApp Switcher mask view"]
      root["RootCheck\nexpo-device heuristics"]
    end
  end

  lg & otp -->|"POST /v1/auth/*"| be_auth["Backend F2 Auth"]
  upload -->|"POST /v1/kyc/submit"| be_kyc["Backend F8 KYC"]
  st -->|"GET /v1/kyc/status"| be_kyc
```

## 2. Main flow — Detailed business flows

### A. Registration & OTP verification (Simulated OTP Flow)

Because the backend creates the user and issues the JWT directly at the `/v1/auth/register` endpoint, the client app is designed with the flow: enter information → simulated OTP verification on the client side → call the register API. This design both gives the end user an OTP experience and matches the API contract with the backend.

```mermaid
sequenceDiagram
  participant U as User
  participant RG as register.tsx
  participant OTP as otp.tsx
  participant API as api/client
  participant SS as session store
  participant BE as Backend (/register)

  U->>RG: Enter Phone number, Full name, Password
  RG->>RG: Validate form (Zod)
  RG->>OTP: Navigate with the registration info payload
  Note over OTP: Start the 120s countdown
  U->>OTP: Enter the 6-digit OTP code (e.g. 123456)
  OTP->>API: Call apiRequest("/v1/auth/register", payload)
  API->>BE: HTTP POST /v1/auth/register
  BE-->>API: 201 Created (user profile & token pair)
  API-->>OTP: Return success
  OTP->>SS: setSession(accessToken, user) & store refresh_token in SecureStore
  OTP->>SS: Navigate based on user.kyc_status (if none -> kyc/intro, otherwise -> home)
```

### B. KYC flow, Client-Side Heuristic and Resume Upload (M10/M11)

The process of capturing the CCCD document and portrait photo, performing a rough image quality check before upload, and the recovery/resume-upload mechanism when the network connection is interrupted.

```mermaid
sequenceDiagram
  participant U as User
  participant ID as capture-id.tsx
  participant SF as capture-selfie.tsx
  participant DR as kyc-draft store
  participant API as api/client
  participant BE as Backend (/kyc/submit)

  U->>ID: Capture the front of the CCCD
  Note over ID: M10: Run the Heuristic check (brightness & blur)<br/>Based on file size and metadata
  alt Image fails the standard (too dark / too blurry)
    ID-->>U: Show a warning toast and force a retake
  else Image passes the standard
    ID->>DR: Store the CCCD image URI locally in the Draft State
    ID->>SF: Navigate to Selfie capture
  end

  U->>SF: Capture the portrait inside the oval frame
  Note over SF: M10: Run the Selfie Heuristic check
  SF->>DR: Store the Selfie image URI locally in the Draft State

  SF->>API: Execute the multipart/form-data upload (CCCD + Selfie)
  alt M11: Network connection lost during upload
    API-->>SF: Return NetworkError
    SF->>DR: Keep the image URIs in SecureStore/Draft
    SF-->>U: Show a connection error with a "Retry" button
    U->>SF: Press "Retry"
    SF->>API: Read the URIs from the Draft and upload again
  end

  API->>BE: POST /v1/kyc/submit
  BE-->>API: 202 Accepted (Verifying workflow started)
  API-->>SF: Success
  SF->>DR: Clear the drafts (Clear Drafts)
  SF->>SF: Navigate to (kyc)/status (3s poll mode)
```

## 3. State — User state lifecycle

```mermaid
stateDiagram-v2
  [*] --> Unauthenticated : Not logged in
  Unauthenticated --> Registering : Fill in the registration form
  Registering --> OTPVerification : Verify the OTP code
  OTPVerification --> Authenticated : Registration succeeded / Logged in directly

  state Authenticated {
    [*] --> KycCheck : Check kyc_status
    KycCheck --> KycIntro : kyc_status === none & not skipped in this session
    KycCheck --> MainHome : kyc_status !== none or "Later" was pressed

    state KycFlow {
      KycIntro --> CapturingID : Start verification
      CapturingID --> CapturingSelfie : CCCD passes the standard
      CapturingSelfie --> Uploading : Selfie passes the standard
      Uploading --> KycStatusScreen : Upload succeeded
    }

    state KycStatusScreen {
      [*] --> PendingPoll : kyc_status === pending
      PendingPoll --> Verified : AI approval succeeded
      PendingPoll --> ManualReview : Moved to manual review
      PendingPoll --> Rejected : Rejected

      Rejected --> KycIntro : Press "Re-verify"
    }
  }

  Authenticated --> IdleLocked : No interaction > 5 minutes / background > 5 minutes
  IdleLocked --> Authenticated : Fingerprint/Face or Passcode succeeded
```

## 4. Data model delta — Client-side storage state

```mermaid
erDiagram
  ZUSTAND_SESSION_STORE {
    string accessToken "In-memory token"
    object userProfile "User { id, phone, kyc_status, full_name }"
    string status "authenticated | unauthenticated | bootstrapping"
    string logoutReason "user | refresh_expired | refresh_revoked"
    boolean kycSkippedInSession "Whether the user pressed 'Later'"
  }

  SECURE_STORE {
    string refresh_token "Key: hpay_refresh_token"
    string device_id "Key: hpay_device_id"
    string kyc_draft_id_uri "Key: kyc_draft_id_uri (Path of the CCCD front image)"
    string kyc_draft_selfie_uri "Key: kyc_draft_selfie_uri (Path of the portrait image)"
    string last_active_time "Key: last_active_time (For Idle Lock)"
  }
```

## 5. Decisions & trade-offs — Technical decisions

| Decision | Alternatives | Rationale & trade-off assessment |
|---|---|---|
| Simulate the OTP flow on the client side during registration | Force the backend to change the API | The backend already finished F2. Simulating ensures the FE has a high-quality OTPInput UI matching the design spec without disturbing the frozen contract-first API contract. |
| Rough image-quality heuristic via file size and image resolution | Send the image to the server/AI for direct analysis | Saves bandwidth and detects poor-quality images immediately (e.g. a file that is too small, < 50KB, is usually blurry or a camera render error), optimizing the instant-retake experience. |
| AppState overlay that covers the screen with a thin full-screen View instead of blocking screenshots at the native level | Integrate a Native library that blocks screenshots | Maximum compatibility with Expo Go and Android/iOS devices without deep Gradle/Xcode configuration changes of the Expo bare workflow. On real devices at release, a security flag can be added later. |

## 6. Invariants touched (index.html §8)

| Invariant | How M2 complies |
|---|---|
| Money transfers always need PIN/OTP; Biometrics only unlocks the app | The biometric unlock is used only in `useLocalUnlock` to dismiss the 5-minute Idle Lock screen. It never stores transfer credentials or auto-signs transactions via biometrics. |
| Never log tokens/OTP to the console | Completely disable printing the entered OTP and tokens in `console.log` via Babel's strip-logs configuration in production build mode. |

hpay-mobile · docs/plans/m2-auth-kyc · 18/07/2026
