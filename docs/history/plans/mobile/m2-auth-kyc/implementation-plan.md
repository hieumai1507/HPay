# 📋 Implementation Plan — M2 Auth & KYC mobile

Phase that builds the Login, Registration, OTP verification and user identity KYC flows of `hpay-mobile`. Backend dependency: **F2 (Auth) + F8 (KYC)** — already completed on the backend.

## 1. Scope & non-goals

### Ships

- **Login & Registration flow:**

  - `Onboarding`: 3 intro slides presenting the core value, with Create Account and Login buttons.
  - `Register`: Form with Phone number (+84), Full name, Password with a strength meter (4 levels), and a terms-agreement checkbox. Validated with react-hook-form + zod.
  - `OTP screen`: `OTPInput` component with 6 digit cells that auto-advance/auto-backspace and support pasting an OTP string. A 120s countdown blocks the resend button until the cooldown ends.
  - `Login`: Phone number + Password form. Supports showing detailed error messages from the backend (account locked `ERR_ACCOUNT_LOCKED`, wrong credentials `ERR_INVALID_CREDENTIALS`) and a notice that the login session was revoked (M05).

- **Identity KYC flow:**

  - `KYC Intro`: A screen with detailed instructions for the verification steps, warning about lighting quality and capture angle. Provides the "Start verification" and "Later" buttons (skip KYC for the current session to view the Home page, but with limited transactions).
  - `KYC Capture ID & Selfie`: Integrates `expo-camera`. Shows the standard overlay frame (an ID card/CCCD rectangular border for the rear camera and an oval for the front camera).
  - **Client-side image check heuristic (M10):** A preliminary brightness and blur heuristic before upload. If the image fails the requirements (e.g. blurry/dark based on size/file weight or sensor parameters), prompt the user to retake it right away.
  - **KYC Draft & Network drop recovery (M11):** Store the captured ID card and Selfie image URIs locally. If the network drops midway through the upload, keep the draft state and show a "Retry" button to continue the upload without forcing the user to capture again from scratch.
  - `KYC Status`: Shows states based on the `/v1/kyc/status` API:

    - `pending`: A waiting screen while the AI check runs (auto-polls every 3s).
    - `verified` / `auto_approved`: A success screen + limit raised to 100M VND/day.
    - `manual_review`: A notice that the file needs manual review by a specialist (wait 1-2 hours).
    - `rejected`: A notice of rejection with the reason from the system and a re-verify button.

- **Client Security:**

  - `Screen privacy (M07)`: Cover the app's sensitive content with a mask view (privacy mask screen) when the app moves to the background/inactive state on iOS/Android.
  - `Local Biometric Lock`: Integrates `expo-local-authentication`. If the app stays in the background for more than 5 minutes, then on return (active) unlocking by fingerprint/face or passcode is mandatory.
  - `Jailbreak/Root detection`: Check whether the device's system has been tampered with using `expo-device` heuristics, showing a small warning in Account/Home but without hard-blocking the user.

### Non-goals

- Home, Top-up, Transfer screens (M3).
- Request Money / receive-money QR flow (M4).
- Business Wallet flow (maker/checker) (M5).

## 2. Task breakdown

| # | Task | Files | Size | Depends |
|---|---|---|---|---|
| T1 | Build the Onboarding + Login + Register UI (form validation, strength meter) | `app/(auth)/onboarding.tsx`, `app/(auth)/login.tsx`, `app/(auth)/register.tsx` | M | M1 skeleton |
| T2 | Build OTPInput + the OTP verification screen (simulate the registration flow before calling the API) | `app/(auth)/otp.tsx`, `src/features/auth/components/OTPInput.tsx` | M | T1 |
| T3 | Build the KYC Intro screen and the camera frame for capturing the CCCD/Selfie with expo-camera | `app/(kyc)/intro.tsx`, `app/(kyc)/capture-id.tsx`, `app/(kyc)/capture-selfie.tsx`, `src/features/kyc/components/KYCPhotoFrame.tsx` | L | T1, T2 |
| T4 | Implement the client-side heuristic check (M10) and local storage of the KYC Draft (M11) | `src/features/kyc/helpers/photo-validator.ts`, `src/features/kyc/stores/kyc-draft.ts` | M | T3 |
| T5 | Design the KYC Status screen with the `/v1/kyc/status` API polling logic | `app/(kyc)/status.tsx`, `src/features/kyc/hooks/useKycStatus.ts` | M | T3, T4 |
| T6 | Implement the AppState privacy mask (M07) and the 5-minute Biometric Idle Lock | `app/_layout.tsx`, `src/stores/security.ts` | M | T1 |
| T7 | Write tests for the edge cases M07, M08, M10, M11 and form/OTP unit tests | `src/features/auth/__tests__/*`, `src/features/kyc/__tests__/*` | L | T1–T6 |

## 3. Schema changes

**None** — client-only phase.

## 4. API changes

Consumes the existing Backend endpoints:

- `POST /v1/auth/register`: Register a member
- `POST /v1/auth/login`: Log in
- `POST /v1/kyc/submit`: Submit the CCCD + Selfie images as multipart/form-data
- `GET /v1/kyc/status`: Get the current KYC status and AI Score/Reasons

## 5. Test plan

| Edge | Test (jest) | Content |
|---|---|---|
| M07 | `TestEdge_M07_AppBackground_PrivacyOverlay` | Verify that when AppState moves to background/inactive the security cover view is shown; it hides again on becoming active in under 5 minutes |
| M08 | `TestEdge_M08_DeepLink_PendingAuth` | If the app receives a deep link to approve a disbursement order while not logged in, store the target path in the store and navigate correctly after a successful login |
| M10 | `TestEdge_M10_KYCImageHeuristic_Validation` | Verify the heuristic that detects blurry/dark images (based on mock size/quality) rejects non-conforming images and shows the correct warning |
| M11 | `TestEdge_M11_KYCUpload_NetworkDropResume` | Simulate a network drop when calling KYC submit -> save the draft locally; when reconnected, pressing Retry submits again using the old URIs |

## 6. Rollout & risk

- **Camera permission:** If the user denies the camera permission, the KYC capture flow is blocked. Design the UI to show a clear message with a button that opens the device Settings to grant the permission again.
- **Devices without Biometrics:** If the phone does not support fingerprint/face, the system automatically falls back to the passcode/login password to unlock the 5-minute Idle Lock.
- **Offline draft:** Draft image URIs kept in SecureStore/AsyncStorage may be deleted if the OS cleans up temporary storage. A timestamp must be stored alongside so that drafts older than 24 hours are deleted.

## 7. Execution checklist

1. Initialize gen-feature-plan (this file + solution-architecture.md) -> **wait for user approval**
2. Create the Auth screens and logic (Onboarding, Login, Register, OTP)
3. Create the KYC screens and logic (Intro, Camera Capture ID/Selfie, Heuristics, Status)
4. Integrate Client Security (Privacy screen, Local Biometric Lock, Root warning)
5. Write and run Jest tests for the Edge cases M07/M08/M10/M11
6. Check typecheck, linting and run the tests: `npm run typecheck && npm run lint && npm test`
7. Deliver the results report and commit the code to Git.

hpay-mobile · docs/plans/m2-auth-kyc · 18/07/2026
