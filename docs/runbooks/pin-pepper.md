---
title: PIN pepper runbook (generate, store, rotate, lose, count)
type: runbook
status: verified-against-code
last_verified: 2026-11-25
code: [src/backend/internal/auth/pin_pepper.go, src/backend/internal/auth/pin.go, src/backend/internal/platform/config/config.go]
adr: ../adr/0018-pin-pepper-and-hash-cost.md
---

# PIN pepper runbook

The transaction PIN hash is `argon2id(HMAC-SHA256(pepper, pin), salt, m=19456 KiB, t=2, p=1)` (ADR-0018, BR-AUTH-46 to 52). The pepper is a secret that lives only in the server configuration. This page is for the operator who deploys, rotates or recovers it.

| Setting | Meaning |
|---|---|
| `PIN_PEPPER` | Base64 of 32 random bytes. Required unless `APP_ENV` is `dev` or `ci`. |
| `PIN_PEPPER_ID` | Name of that key, stored inside every new hash (`k=<id>`). Default `p1`. Pattern `[a-z0-9-]{1,16}`. |
| `PIN_PEPPER_OLD` | Retired keys that may still verify: `id:base64,id2:base64`. Ids are unique and differ from `PIN_PEPPER_ID`. |

The server refuses to start in production or demo when `PIN_PEPPER` is missing, malformed, not 32 bytes, or equal to the public dev test value, and when an id is empty, repeated or badly shaped. Startup logs only the key id and the number of old keys, never a key.

## 1. Generate a key
```
openssl rand -base64 32
```
Use a new random key per environment. Never reuse the TOTP key or the JWT secret. Never paste the key into a ticket, chat or log.

## 2. Where the key must NOT be stored
- Not in the database (any table, any setting row).
- Not in database backups, dumps or replicas, and not in the same backup set as them.
- Not in the same secret file or secret object as `DATABASE_URL` where that can be avoided: someone who can read one should not automatically get both.
- Not in the repository, in `.env.example`, in images, in CI logs or in error trackers.

Why: a copy of `users.pin_hash` alone reveals nothing that can be tested offline, because every hash is of `HMAC(pepper, pin)`. A copy of the table plus the pepper lets an attacker try all 1,000,000 PINs at the lighter cost, which is worse than before the pepper existed. Keep the two in different trust zones.

Store it in the secret manager with versioned backups held by two people.

## 3. First deployment (migration by use)
1. Generate and store the key. Set `PIN_PEPPER` (and optionally `PIN_PEPPER_ID`).
2. Deploy. Nothing changes for users: legacy hashes still verify, and each one is rewritten to the new format by the user's next successful PIN check (payment confirm, change, reset).
3. Run the count query (section 6). The `legacy` row shrinks as users pay. Users who never pay keep their legacy hash, which is as safe as before.
4. **Rollback warning:** a build from before this change cannot read a hash with `k=`. After the first rewrite, rolling back the application means every rewritten user needs a PIN reset. Do not roll back past this release without that plan.

## 4. Planned rotation
1. Generate a new key. Pick a new id, for example `p2`.
2. Set `PIN_PEPPER` to the new key, `PIN_PEPPER_ID=p2`, and `PIN_PEPPER_OLD=p1:<the old base64>`. Deploy. Both keys verify; the first successful check of each user rewrites the hash under `p2`.
3. Run the count query until the row for `p1` is zero, or until the cutoff date you announced.
4. Remove `p1` from `PIN_PEPPER_OLD` and deploy.
5. Users still on `p1` at that point fail verification exactly like a wrong PIN (the client is not told why; the server log names the key id) and use the PIN reset (ADR-0016). Each such failure counts toward the 5-attempt lock, and the reset clears it.
6. Never change `PIN_PEPPER` without moving the old value to `PIN_PEPPER_OLD`: that is the same as losing the key.

## 5. Loss (no copy of the current key)
Every hash under that key id is unverifiable. There is no way back. Recovery is the PIN reset of ADR-0016 for every affected user.
1. Count the affected users (section 6, rows for the lost key id).
2. Generate a new key with a new id, deploy it as `PIN_PEPPER` (the lost one is simply gone, do not list it in `PIN_PEPPER_OLD`).
3. Expect a wave of "PIN incorrect" and lockouts. Tell users first. Status message template:

   > Subject: Action needed: reset your HPay transaction PIN
   > We have updated how transaction PINs are protected. For your safety, please reset your PIN once: open the app, go to Settings, Transaction PIN, Forgot PIN, enter your password and the code we send you, then choose a new PIN. Payments that need a PIN are paused until you do. Your balance and history are not affected. If you cannot receive the code, contact support.

4. Support can use the operator fallback (permission `pin.reset`, superadmin only, reason required) for users who cannot receive the code.
5. After a reset a 24 hour cooling period caps PIN transactions at 2,500,000 VND (BR-AUTH-39).
6. Track progress with the count query until the lost id reaches zero.

## 6. Count hashes per key id
Read-only. Run it with a read-only database role. It selects no hash and no secret, only the key id parsed out of the hash.
```sql
SELECT
  CASE
    WHEN pin_hash IS NULL OR pin_hash = '' THEN 'no pin'
    WHEN pin_hash ~ '^\$argon2id\$v=19\$m=[0-9]+,t=[0-9]+,p=[0-9]+,k=[a-z0-9-]{1,16}\$'
      THEN 'key ' || substring(pin_hash from ',k=([a-z0-9-]{1,16})\$')
    WHEN pin_hash LIKE '$argon2id$%' THEN 'legacy'
    ELSE 'unreadable'
  END AS key_id,
  count(*) AS users
FROM users
GROUP BY 1
ORDER BY 1;
```
Reading it: `key p2` is the current key after a rotation; `key p1` must reach zero before p1 is removed; `legacy` is the raw-PIN format from before ADR-0018; `unreadable` should be zero. The sum of the rows is the number of users.

To see only hashes that are on a key other than the current one (replace `p2`):
```sql
SELECT count(*) FROM users
WHERE pin_hash IS NOT NULL AND pin_hash <> ''
  AND pin_hash NOT LIKE '$argon2id$v=19$m=19456,t=2,p=1,k=p2$%';
```

## 7. Checks after a deploy
- The startup log has `PIN pepper loaded key_id=<id> old_keys=<n>` and no key material.
- A test payment with a known user succeeds, and the count query shows that user under the current id.
- No jump in `pin.failed` audit rows right after the deploy. A jump means the wrong pepper is configured in one environment: fix the configuration, do not reset PINs.

## Related
ADR-0018, ADR-0016 (PIN reset), ADR-0017 (TOTP key, same loading rules), `domain/identity-and-auth.md`, `reference/configuration.md`.
