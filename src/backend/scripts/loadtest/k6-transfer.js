// STALE: do not use. This script calls routes that no longer exist (/v1/transactions/transfer,
// /v1/wallets/topup). The flow is now /v1/transactions/request + /v1/transactions/confirm with a PIN,
// and the login, register and confirm rate limits apply. Use `make loadtest` (the Go load test) instead;
// see scripts/loadtest/README.md. Kept as a starting point for an HTTP-level rewrite.

// F10 load test — 100 TPS P2P transfer for 60s (NFR doc 01 §5).
// Open model (constant-arrival-rate) so we measure real TPS, not a closed
// loop that self-throttles when the system slows down.
//
// Run:  make dev   (server on :8080, docker compose up + migrate-up first)
//       make loadtest
// Note: register/login are rate-limited 5/min/IP — this script creates exactly
// 4 users in setup(); wait a minute between repeated runs.
//
// Scenarios:
//   transfers  — 100 req/s POST /v1/transactions/transfer (202-accepted path)
//   e2e_sample — 5 req/s transfer + poll detail until completed (<2s NFR)
import http from "k6/http";
import { check, fail, sleep } from "k6";
import { Trend, Rate } from "k6/metrics";

const BASE = __ENV.BASE_URL || "http://localhost:8080";
const e2eTime = new Trend("transfer_e2e_ms", true);
const e2eOk = new Rate("transfer_e2e_completed");

export const options = {
  scenarios: {
    transfers: {
      executor: "constant-arrival-rate",
      rate: 100,
      timeUnit: "1s",
      duration: "60s",
      preAllocatedVUs: 150,
      maxVUs: 300,
      exec: "transfer",
    },
    e2e_sample: {
      executor: "constant-arrival-rate",
      rate: 5,
      timeUnit: "1s",
      duration: "60s",
      preAllocatedVUs: 20,
      maxVUs: 40,
      exec: "transferE2E",
    },
  },
  thresholds: {
    // NFR: p95 read < 100ms; transfer accept fast; e2e < 2s; errors < 1%
    "http_req_duration{name:transfer}": ["p(95)<500"],
    "http_req_duration{name:detail}": ["p(95)<100"],
    transfer_e2e_ms: ["p(95)<2000"],
    transfer_e2e_completed: ["rate>0.99"],
    http_req_failed: ["rate<0.01"],
  },
};

const PASS = "LoadTest@123";

function auth(token) {
  return { headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" } };
}

export function setup() {
  const stamp = String(Date.now()).slice(-7);
  const users = [];
  for (let i = 0; i < 4; i++) {
    const phone = `097${stamp}${i}`;
    const res = http.post(
      `${BASE}/v1/auth/register`,
      JSON.stringify({ phone, password: PASS, full_name: `Load User ${i}` }),
      { headers: { "Content-Type": "application/json" } }
    );
    if (res.status !== 201) fail(`register ${phone}: ${res.status} ${res.body}`);
    users.push({ phone, token: res.json("tokens.access_token") });
  }
  // Fund the two senders: KYC 'none' caps topups at 2M each — do 2 per sender.
  for (const u of [users[0], users[1]]) {
    for (let t = 0; t < 2; t++) {
      const res = http.post(`${BASE}/v1/wallets/topup`, JSON.stringify({ amount: 8_000 }), {
        headers: { ...auth(u.token).headers, "Idempotency-Key": `${u.phone}-fund-${t}` },
      });
      if (res.status !== 201) fail(`topup ${u.phone}: ${res.status} ${res.body}`);
    }
  }
  return { users };
}

function doTransfer(data, tagName) {
  const sender = data.users[__VU % 2]; // users[0]/users[1] send
  const receiver = data.users[2 + (__VU % 2)]; // users[2]/users[3] receive
  return http.post(
    `${BASE}/v1/transactions/transfer`,
    JSON.stringify({ receiver_phone: receiver.phone, amount: 400, note: "k6" }),
    {
      headers: { ...auth(sender.token).headers, "Idempotency-Key": `k6-${__VU}-${__ITER}-${Date.now()}` },
      tags: { name: tagName },
    }
  );
}

export function transfer(data) {
  const res = doTransfer(data, "transfer");
  check(res, { "transfer accepted": (r) => r.status === 202 || r.status === 201 });
}

export function transferE2E(data) {
  const start = Date.now();
  const res = doTransfer(data, "transfer");
  if (!check(res, { "e2e accepted": (r) => r.status === 202 || r.status === 201 })) {
    e2eOk.add(false);
    return;
  }
  const ref = res.json("ref_code");
  const sender = data.users[__VU % 2];
  for (let i = 0; i < 40; i++) {
    const d = http.get(`${BASE}/v1/transactions/${ref}`, {
      ...auth(sender.token),
      tags: { name: "detail" },
    });
    if (d.status === 200 && d.json("status") === "completed") {
      e2eTime.add(Date.now() - start);
      e2eOk.add(true);
      return;
    }
    if (Date.now() - start > 2000) break;
    sleep(0.05);
  }
  e2eOk.add(false);
}
