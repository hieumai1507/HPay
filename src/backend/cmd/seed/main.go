// Command seed populates the dev database with realistic demo state
// (.claude/skills/seed-demo-data). Idempotent: every transaction carries a
// fixed idempotency key, users upsert by phone — safe to re-run. All money
// moves through ledger.Post; the seeder asserts balance == SUM(ledger) for
// every touched wallet at the end and fails loudly on drift.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/auth"
	"github.com/hieumai1507/hpay-backend/internal/ledger"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
)

const demoPassword = "Demo@123"

// e2eAdminPassword is a FIXED, KNOWN plaintext credential for the two
// Playwright E2E fixture admins (src/admin/e2e). Never used for real/demo
// accounts — only created when --e2e-admins is passed.
const e2eAdminPassword = "E2ETest@123"

type seeder struct {
	pool    *pgxpool.Pool
	ctx     context.Context
	touched map[uuid.UUID]string // wallet id -> label, for the final assert
}

func main() {
	e2eAdmins := flag.Bool("e2e-admins", false, "additionally upsert 2 fixed-credential admin_users for the Playwright E2E suite (src/admin/e2e)")
	allowRemote := flag.Bool("allow-remote-db", false, "allow seeding a database whose host is not local (the seeder creates accounts with known passwords)")
	flag.Parse()

	url := os.Getenv("DATABASE_URL")
	host, err := checkSeedTarget(os.Getenv("APP_ENV"), url, *allowRemote)
	if err != nil {
		fatal("%v", err)
	}
	fmt.Printf("seeding database host %q (APP_ENV=%s)\n", host, os.Getenv("APP_ENV"))
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		fatal("connect: %v", err)
	}
	defer pool.Close()

	s := &seeder{pool: pool, ctx: ctx, touched: map[uuid.UUID]string{}}
	s.run()

	if *e2eAdmins {
		s.seedE2EAdmins()
	}
}

func fatal(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "seed: "+f+"\n", a...)
	os.Exit(1)
}

func (s *seeder) run() {
	hash, err := auth.HashPassword(demoPassword)
	if err != nil {
		fatal("hash: %v", err)
	}

	// --- system wallets (created by migration; verify present) ---
	cashIn := s.systemWallet("SYSTEM_CASH_IN")
	s.systemWallet("SYSTEM_FEE")

	// --- users ---
	u1 := s.user("0900000001", "Hieu Mai", "verified", "active", hash)
	u2 := s.user("0900000002", "Minh Anh", "verified", "active", hash)
	u3 := s.user("0900000003", "Tran Van Ba", "manual_review", "active", hash)
	u4 := s.user("0900000004", "Le Thi Tu", "none", "locked", hash)
	owner := s.user("0900000011", "Acme Owner", "verified", "active", hash)
	maker := s.user("0900000012", "Acme Maker", "verified", "active", hash)
	l1 := s.user("0900000013", "Acme Checker L1", "verified", "active", hash)
	l2 := s.user("0900000014", "Acme Checker L2", "verified", "active", hash)

	// Offer packages by KYC tier (docs/specs/w3-offer-limits): u1 is fully verified (L2), u2 basic
	// verified (L1), the manual-review and locked users stay on L0; the Acme Corp actors use the
	// business package.
	s.assignOffer(u1, "OFFER_CUST_L2")
	s.assignOffer(u2, "OFFER_CUST_L1")
	s.assignOffer(u3, "OFFER_CUST_L0")
	s.assignOffer(u4, "OFFER_CUST_L0")
	for _, biz := range []uuid.UUID{owner, maker, l1, l2} {
		s.assignOffer(biz, "OFFER_BIZ_STARTER")
	}

	w1 := s.mainWallet(u1, "Hieu Mai main")
	w2 := s.mainWallet(u2, "Minh Anh main")
	s.mainWallet(u3, "user3 main")
	w4 := s.mainWallet(u4, "user4 main")
	wOwner := s.mainWallet(owner, "owner main")
	wMaker := s.mainWallet(maker, "maker main")
	wL1 := s.mainWallet(l1, "checker L1 main")
	wL2 := s.mainWallet(l2, "checker L2 main")
	vault := s.vaultWallet(u1, "Holiday fund")

	// --- u1 history: >=10 mixed txns across several days; final $498.00 (49,800 cents) ---
	s.txn("seed-u1-topup-1", "topup", cashIn, w1, 60_000, u1, -6)
	s.txn("seed-u1-topup-2", "topup", cashIn, w1, 24_000, u1, -4)
	s.txn("seed-u1-p2p-1", "p2p", w1, w2, 2000, u1, -5)
	s.txn("seed-u1-p2p-2", "p2p", w1, w2, 2800, u1, -3)
	s.txn("seed-u1-p2p-3", "p2p", w1, w2, 3200, u1, -2)
	flagged := s.txn("seed-u1-p2p-4", "p2p", w1, w2, 7000, u1, -1)
	velocity := s.txn("seed-u1-p2p-5", "p2p", w1, w2, 4000, u1, 0)
	s.txn("seed-u2-p2p-back", "p2p", w2, w1, 400, u2, -1)
	s.txn("seed-u1-vault-1", "vault_move", w1, vault, 16_000, u1, -3)
	s.txn("seed-u1-vault-2", "vault_move", w1, vault, 800, u1, -1)

	// --- u2 / org-side funding ---
	s.txn("seed-u2-topup-1", "topup", cashIn, w2, 2000, u2, -4)
	s.txn("seed-u11-topup-1", "topup", cashIn, wOwner, 2000, owner, -2)

	// --- organization Acme Corp + policy "Large spend" (large spend) v3 ---
	orgID := s.org("Acme Corp", owner)
	orgWallet := s.orgWallet(orgID)
	s.member(orgID, owner, "admin", 0)
	s.member(orgID, maker, "maker", 0)
	s.member(orgID, l1, "checker", 1)
	s.member(orgID, l2, "checker", 2)
	policyID := s.policy(orgID, "Large spend", 3, owner)

	s.txn("seed-org-topup", "topup", cashIn, orgWallet, 3_418_000, owner, -7)
	// completed disbursement with a full step trail
	disb := s.txn("seed-org-disb-done", "org_disbursement", orgWallet, w2, 48_000, maker, -2)
	s.instance("seed-inst-disb-done", "org_disbursement", disb, "completed", "", -2,
		[]step{{"VALIDATE", "completed"}, {"FRAUD_CHECK", "completed"}, {"WAIT_APPROVAL", "completed"},
			{"MOVE_FUNDS", "completed"}, {"CONFIRM", "completed"}})

	// pending disbursement at 1/2 signatures, parked at WAIT_APPROVAL
	pend := s.pendingTxn("seed-org-disb-pending", "org_disbursement", orgWallet, wMaker, 180_000, maker, "pending_approval")
	pendInst := s.instance("seed-inst-disb-pending", "org_disbursement", pend, "waiting_approval", "WAIT_APPROVAL", 0,
		[]step{{"VALIDATE", "completed"}, {"FRAUD_CHECK", "completed"}, {"WAIT_APPROVAL", "started"}})
	s.approvalOrgTxn(pend, orgID, pendInst, policyID, maker, l1)

	// --- workflow variety for the admin monitor ---
	failT := s.compensatedTxn("seed-wf-failed", w2, w4, 1200, u2, "failed", -1)
	s.instance("seed-inst-failed", "p2p_transfer", failT, "failed", "MOVE_FUNDS", -1,
		[]step{{"VALIDATE", "completed"}, {"FRAUD_CHECK", "completed"}, {"MOVE_FUNDS", "failed"}})
	compT := s.compensatedTxn("seed-wf-comp", w2, w4, 1200, u2, "failed", -2)
	s.instance("seed-inst-comp", "p2p_transfer", compT, "compensated", "CONFIRM", -2,
		[]step{{"VALIDATE", "completed"}, {"FRAUD_CHECK", "completed"}, {"MOVE_FUNDS", "completed"},
			{"CONFIRM", "failed"}, {"MOVE_FUNDS", "compensated"}})
	asyncT := s.pendingTxn("seed-wf-async", "p2p", wMaker, wL1, 800, maker, "processing_async")
	s.instance("seed-inst-async", "p2p_transfer", asyncT, "running", "MOVE_FUNDS", 0,
		[]step{{"VALIDATE", "completed"}, {"FRAUD_CHECK", "completed"}, {"MOVE_FUNDS", "started"}})

	// --- u4: locked with a confirmed_fraud flag + reversal pending in My Tasks ---
	s.txn("seed-u4-topup", "topup", cashIn, w4, 4000, u4, -3)
	badTxn := s.txn("seed-u4-p2p", "p2p", w4, w2, 2000, u4, -1)
	s.fraudFlag(badTxn, "VELOCITY_5MIN", 92,
		`{"signals":[{"code":"VELOCITY_5MIN","points":60,"evidence":"3 transactions in 4 minutes"}],"advisory":"AI: behaviour resembles an account takeover — recommend locking the account and reversing the transactions"}`,
		"confirmed_fraud")
	s.reversalApproval(badTxn)

	// --- fraud flags on u1 traffic ---
	// reasons matches the real shape internal/fraud/handler.go writes
	// ({signals:[{code,points,evidence}], advisory}) — not a plain string
	// array, which the admin console's Fraud Monitor can't render (F6 bug).
	s.fraudFlag(velocity, "VELOCITY_5MIN", 87,
		`{"signals":[{"code":"VELOCITY_5MIN","points":55,"evidence":"5 transactions in 5 minutes"}],"advisory":"AI: unusual frequency compared with the last 30 days — review manually"}`,
		"open")
	s.fraudFlag(flagged, "AMOUNT_SPIKE", 45,
		`{"signals":[{"code":"AMOUNT_SPIKE","points":45,"evidence":"4.2 times the 30-day average"}],"advisory":"AI: familiar recipient, low risk"}`,
		"cleared")

	// --- kyc queue: u3 sitting at manual_review, ai_score 64 ---
	s.kycRequest(u3)
	writeKYCDemoImages(s.ctx)

	// --- recon: yesterday, 3 matched + 3 missing_external at the cutoff window ---
	y := time.Now().AddDate(0, 0, -1)
	r1 := s.reconTopup("seed-recon-t1", cashIn, wMaker, 4000, maker, y.Add(-10*time.Hour))
	r2 := s.reconTopup("seed-recon-t2", cashIn, wL1, 4000, l1, y.Add(-8*time.Hour))
	r3 := s.reconTopup("seed-recon-t3", cashIn, wL2, 4000, l2, y.Add(-6*time.Hour))
	s.statement(r1, 4000, y, "match")
	s.statement(r2, 4000, y, "match")
	s.statement(r3, 4000, y, "match")
	night := time.Date(y.Year(), y.Month(), y.Day(), 23, 58, 0, 0, time.Local)
	s.statement("SEEDX1"+shortID(), 10_000, night, "missing")
	s.statement("SEEDX2"+shortID(), 4800, night.Add(2*time.Minute), "missing")
	s.statement("SEEDX3"+shortID(), 3200, night.Add(3*time.Minute), "missing")
	s.reconRun(y)

	// --- money requests: 1 pending incoming for u1 + a split group 2/3 paid ---
	s.moneyRequest("seed-mreq-1", u2, u1, 800, "Coffee money this week", nil, "pending", uuid.Nil)
	split := fixedUUID("seed-split-group")
	p1 := s.txn("seed-split-pay-u2", "p2p", w2, w1, 600, u2, 0)
	p2 := s.txn("seed-split-pay-u11", "p2p", wOwner, w1, 600, owner, 0)
	s.moneyRequest("seed-mreq-s1", u1, u2, 600, "Split: group dinner", &split, "paid", p1)
	s.moneyRequest("seed-mreq-s2", u1, owner, 600, "Split: group dinner", &split, "paid", p2)
	s.moneyRequest("seed-mreq-s3", u1, maker, 600, "Split: group dinner", &split, "pending", uuid.Nil)

	s.assertLedger()
	s.summary()
}

// seedE2EAdmins upserts exactly 2 admin_users rows with fixed, known plaintext
// credentials for the Playwright E2E suite (src/admin/e2e/fixtures/auth.ts):
// a maker (role ops) and a checker (role superadmin), so maker-checker specs
// (My Tasks) have two distinct, real, logged-in admin identities to race.
// Additive only — never touches the default seed data/behavior above.
func (s *seeder) seedE2EAdmins() {
	hash, err := auth.HashPassword(e2eAdminPassword)
	if err != nil {
		fatal("e2e admin hash: %v", err)
	}
	s.adminUser("e2e-maker@HPay.dev", "E2E Maker", "ops", hash)
	s.adminUser("e2e-checker@HPay.dev", "E2E Checker", "superadmin", hash)
	s.seedE2EReversible()
	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Println(" E2E admin fixtures OK — password for both: " + e2eAdminPassword)
	fmt.Println("   e2e-maker@HPay.dev    role=ops")
	fmt.Println("   e2e-checker@HPay.dev  role=superadmin")
	fmt.Println("─────────────────────────────────────────────────────────")
}

// e2eReversiblePool is how many completed transactions the admin E2E suite gets to reverse.
// A completed reversal is final (one reversal per transaction, ADR-0013), so every test that
// approves a reversal consumes one of these for good; the suite uses a fresh one each time.
const e2eReversiblePool = 40

// e2eReversibleAmount is the amount of each reversible transaction, in cents ($1.00).
const e2eReversibleAmount = 100

// seedE2EReversible creates completed p2p transactions dedicated to the E2E suite. They are
// flagged only by their idempotency key prefix, belong to throwaway users, and are only created
// with --e2e-admins, so the default demo data is unchanged.
func (s *seeder) seedE2EReversible() {
	cashIn := s.systemWallet("SYSTEM_CASH_IN")
	hash, err := auth.HashPassword(demoPassword)
	if err != nil {
		fatal("e2e reversible hash: %v", err)
	}
	from := s.user("0900009001", "E2E Sender", "verified", "active", hash)
	to := s.user("0900009002", "E2E Receiver", "verified", "active", hash)
	wFrom := s.mainWallet(from, "e2e sender main")
	wTo := s.mainWallet(to, "e2e receiver main")
	s.txn("e2e-pool-topup", "topup", cashIn, wFrom, int64(e2eReversiblePool)*e2eReversibleAmount, from, -1)
	for i := 1; i <= e2eReversiblePool; i++ {
		key := fmt.Sprintf("e2e-reversible-%03d", i)
		id := s.txn(key, "p2p", wFrom, wTo, e2eReversibleAmount, from, -1)
		// A completed workflow instance makes the transaction discoverable through
		// GET /v1/admin/workflows?definition=p2p_transfer, which is how the E2E fixture finds it.
		s.instance("inst-"+key, "p2p_transfer", id, "completed", "", -1,
			[]step{{"VALIDATE", "completed"}, {"FRAUD_CHECK", "completed"}, {"MOVE_FUNDS", "completed"}, {"CONFIRM", "completed"}})
	}
	fmt.Printf("   E2E reversible pool: %d completed p2p transactions (key prefix e2e-reversible-)\n", e2eReversiblePool)
}

// adminUser upserts a single admin_users row by email (idempotent, safe to
// re-run — matches the demo seed's upsert-by-natural-key pattern above).
func (s *seeder) adminUser(email, fullName, role, passwordHash string) uuid.UUID {
	var id uuid.UUID
	err := s.pool.QueryRow(s.ctx, `
		INSERT INTO admin_users (id, email, password_hash, full_name, role, status)
		VALUES ($1,$2,$3,$4,$5,'active')
		ON CONFLICT (email) DO UPDATE
		  SET password_hash=$3, full_name=$4, role=$5, status='active'
		RETURNING id`,
		fixedUUID("admin:"+email), email, passwordHash, fullName, role).Scan(&id)
	if err != nil {
		fatal("admin user %s: %v", email, err)
	}
	return id
}

// ---------- helpers ----------

func fixedUUID(key string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("hpay-seed:"+key))
}
func shortID() string { return uuid.NewString()[:6] }

func (s *seeder) systemWallet(code string) uuid.UUID {
	var id uuid.UUID
	if err := s.pool.QueryRow(s.ctx, `SELECT id FROM wallets WHERE system_code=$1`, code).Scan(&id); err != nil {
		fatal("system wallet %s missing — run migrations first: %v", code, err)
	}
	return id
}

func (s *seeder) user(phone, name, kyc, status, hash string) uuid.UUID {
	var id uuid.UUID
	err := s.pool.QueryRow(s.ctx, `
		INSERT INTO users (id, phone, password_hash, full_name, kyc_status, status)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (phone) DO UPDATE
		  SET full_name=$4, kyc_status=$5, status=$6, password_hash=$3
		RETURNING id`,
		fixedUUID("user:"+phone), phone, hash, name, kyc, status).Scan(&id)
	if err != nil {
		fatal("user %s: %v", phone, err)
	}
	return id
}

// assignOffer puts a user on an Offer Factory package. Without this OfferGuard
// falls back to the platform default, which still meters the transaction but
// hides the per-package limits the demo is meant to show.
func (s *seeder) assignOffer(userID uuid.UUID, offerCode string) {
	if _, err := s.pool.Exec(s.ctx, `
		INSERT INTO user_offers (user_id, offer_id, assigned_by)
		SELECT $1, o.id, 'seed' FROM offers o WHERE o.code = $2
		ON CONFLICT (user_id) DO UPDATE
		  SET offer_id = EXCLUDED.offer_id, assigned_by = 'seed', assigned_at = now()`,
		userID, offerCode); err != nil {
		fatal("assign offer %s: %v", offerCode, err)
	}
}

func (s *seeder) mainWallet(userID uuid.UUID, label string) uuid.UUID {
	if _, err := s.pool.Exec(s.ctx, `
		INSERT INTO wallets (owner_type, owner_id, currency, purpose)
		VALUES ('user',$1,$2,'main') ON CONFLICT DO NOTHING`, userID, wallet.DefaultCurrency); err != nil {
		fatal("wallet: %v", err)
	}
	var id uuid.UUID
	if err := s.pool.QueryRow(s.ctx, `
		SELECT id FROM wallets WHERE owner_type='user' AND owner_id=$1 AND purpose='main'`,
		userID).Scan(&id); err != nil {
		fatal("wallet select: %v", err)
	}
	s.touched[id] = label
	return id
}

func (s *seeder) vaultWallet(userID uuid.UUID, label string) uuid.UUID {
	var id uuid.UUID
	err := s.pool.QueryRow(s.ctx, `
		SELECT id FROM wallets WHERE owner_type='user' AND owner_id=$1 AND purpose='vault' AND label=$2`,
		userID, label).Scan(&id)
	if err == nil {
		s.touched[id] = "vault " + label
		return id
	}
	if err := s.pool.QueryRow(s.ctx, `
		INSERT INTO wallets (owner_type, owner_id, currency, purpose, label)
		VALUES ('user',$1,$3,'vault',$2) RETURNING id`, userID, label, wallet.DefaultCurrency).Scan(&id); err != nil {
		fatal("vault: %v", err)
	}
	s.touched[id] = "vault " + label
	return id
}

// txn inserts a completed transaction with real ledger legs, backdated by
// dayOffset days. Fixed idempotency key => re-runs are no-ops.
func (s *seeder) txn(key, typ string, from, to uuid.UUID, amount int64, by uuid.UUID, dayOffset int) uuid.UUID {
	if id, ok := s.existingTxn(key); ok {
		return id
	}
	tx, err := s.pool.Begin(s.ctx)
	if err != nil {
		fatal("begin: %v", err)
	}
	defer tx.Rollback(s.ctx)
	at := time.Now().AddDate(0, 0, dayOffset)
	var id uuid.UUID
	if err := tx.QueryRow(s.ctx, `
		INSERT INTO transactions (ref_code, idempotency_key, type, status,
		  sender_wallet_id, receiver_wallet_id, amount, fee, initiated_by, created_at, completed_at)
		VALUES ($1,$2,$3,'completed',$4,$5,$6,0,$7,$8,$8) RETURNING id`,
		wallet.NewRefCode(), key, typ, from, to, amount, by, at).Scan(&id); err != nil {
		fatal("txn %s: %v", key, err)
	}
	if err := ledger.Post(s.ctx, tx, id, ledger.KindForward, []ledger.Leg{
		{WalletID: from, Direction: store.EntryDirectionDebit, Amount: amount},
		{WalletID: to, Direction: store.EntryDirectionCredit, Amount: amount},
	}); err != nil {
		fatal("ledger %s: %v", key, err)
	}
	if err := tx.Commit(s.ctx); err != nil {
		fatal("commit %s: %v", key, err)
	}
	return id
}

// pendingTxn inserts a transaction in a non-final status with NO ledger legs.
func (s *seeder) pendingTxn(key, typ string, from, to uuid.UUID, amount int64, by uuid.UUID, status string) uuid.UUID {
	if id, ok := s.existingTxn(key); ok {
		return id
	}
	var id uuid.UUID
	if err := s.pool.QueryRow(s.ctx, `
		INSERT INTO transactions (ref_code, idempotency_key, type, status,
		  sender_wallet_id, receiver_wallet_id, amount, fee, initiated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,0,$8) RETURNING id`,
		wallet.NewRefCode(), key, typ, status, from, to, amount, by).Scan(&id); err != nil {
		fatal("pending txn %s: %v", key, err)
	}
	return id
}

// compensatedTxn posts legs and their reversal — net zero, final status failed
// (E07-shaped material for the admin monitor).
func (s *seeder) compensatedTxn(key string, from, to uuid.UUID, amount int64, by uuid.UUID, status string, dayOffset int) uuid.UUID {
	if id, ok := s.existingTxn(key); ok {
		return id
	}
	tx, err := s.pool.Begin(s.ctx)
	if err != nil {
		fatal("begin: %v", err)
	}
	defer tx.Rollback(s.ctx)
	at := time.Now().AddDate(0, 0, dayOffset)
	var id uuid.UUID
	if err := tx.QueryRow(s.ctx, `
		INSERT INTO transactions (ref_code, idempotency_key, type, status,
		  sender_wallet_id, receiver_wallet_id, amount, fee, initiated_by, created_at, failure_reason)
		VALUES ($1,$2,'p2p',$3,$4,$5,$6,0,$7,$8,'receiver wallet locked mid-flight (seed)') RETURNING id`,
		wallet.NewRefCode(), key, status, from, to, amount, by, at).Scan(&id); err != nil {
		fatal("comp txn %s: %v", key, err)
	}
	legs := []ledger.Leg{
		{WalletID: from, Direction: store.EntryDirectionDebit, Amount: amount},
		{WalletID: to, Direction: store.EntryDirectionCredit, Amount: amount},
	}
	if err := ledger.Post(s.ctx, tx, id, ledger.KindForward, legs); err != nil {
		fatal("ledger %s: %v", key, err)
	}
	rev := []ledger.Leg{
		{WalletID: to, Direction: store.EntryDirectionDebit, Amount: amount},
		{WalletID: from, Direction: store.EntryDirectionCredit, Amount: amount},
	}
	if err := ledger.Post(s.ctx, tx, id, ledger.KindCompensate, rev); err != nil {
		fatal("ledger rev %s: %v", key, err)
	}
	if err := tx.Commit(s.ctx); err != nil {
		fatal("commit %s: %v", key, err)
	}
	return id
}

func (s *seeder) existingTxn(key string) (uuid.UUID, bool) {
	var id uuid.UUID
	err := s.pool.QueryRow(s.ctx, `SELECT id FROM transactions WHERE idempotency_key=$1`, key).Scan(&id)
	if err == nil {
		return id, true
	}
	if err != pgx.ErrNoRows {
		fatal("lookup %s: %v", key, err)
	}
	return uuid.Nil, false
}

func (s *seeder) org(name string, owner uuid.UUID) uuid.UUID {
	id := fixedUUID("org:" + name)
	if _, err := s.pool.Exec(s.ctx, `
		INSERT INTO organizations (id, name, created_by) VALUES ($1,$2,$3)
		ON CONFLICT (id) DO NOTHING`, id, name, owner); err != nil {
		fatal("org: %v", err)
	}
	return id
}

func (s *seeder) orgWallet(orgID uuid.UUID) uuid.UUID {
	if _, err := s.pool.Exec(s.ctx, `
		INSERT INTO wallets (owner_type, owner_id, currency, purpose)
		VALUES ('organization',$1,$2,'main') ON CONFLICT DO NOTHING`, orgID, wallet.DefaultCurrency); err != nil {
		fatal("org wallet: %v", err)
	}
	var id uuid.UUID
	if err := s.pool.QueryRow(s.ctx, `
		SELECT id FROM wallets WHERE owner_type='organization' AND owner_id=$1 AND purpose='main'`,
		orgID).Scan(&id); err != nil {
		fatal("org wallet select: %v", err)
	}
	s.touched[id] = "Acme Corp org"
	return id
}

func (s *seeder) member(orgID, userID uuid.UUID, role string, level int) {
	var lv any
	if level > 0 {
		lv = level
	}
	if _, err := s.pool.Exec(s.ctx, `
		INSERT INTO org_members (org_id, user_id, role, checker_level)
		VALUES ($1,$2,$3,$4) ON CONFLICT (org_id, user_id) DO NOTHING`,
		orgID, userID, role, lv); err != nil {
		fatal("member: %v", err)
	}
}

// policy "Large spend" v3: >= $400.00 needs 1 L1 signature; the second tier adds 1 L2 signature.
func (s *seeder) policy(orgID uuid.UUID, name string, version int, by uuid.UUID) uuid.UUID {
	id := fixedUUID(fmt.Sprintf("policy:%s:%d", name, version))
	if _, err := s.pool.Exec(s.ctx, `
		INSERT INTO approval_policies (id, org_id, name, min_amount, max_amount, tiers, version, is_active, created_by)
		VALUES ($1,$2,$3,40000,NULL,
		  '[{"level":1,"required_approvals":1},{"level":2,"required_approvals":1}]'::jsonb,
		  $4,true,$5)
		ON CONFLICT (id) DO NOTHING`, id, orgID, name, version, by); err != nil {
		fatal("policy: %v", err)
	}
	return id
}

type step struct{ name, status string }

func (s *seeder) instance(key, defName string, txnID uuid.UUID, status, current string, dayOffset int, steps []step) uuid.UUID {
	id := fixedUUID("inst:" + key)
	var defID uuid.UUID
	if err := s.pool.QueryRow(s.ctx,
		`SELECT id FROM workflow_definitions WHERE name=$1 AND is_active`, defName).Scan(&defID); err != nil {
		fatal("definition %s: %v", defName, err)
	}
	var cur any
	if current != "" {
		cur = current
	}
	at := time.Now().AddDate(0, 0, dayOffset)
	if _, err := s.pool.Exec(s.ctx, `
		INSERT INTO workflow_instances (id, definition_id, transaction_id, status, current_step, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$6) ON CONFLICT (id) DO NOTHING`,
		id, defID, txnID, status, cur, at); err != nil {
		fatal("instance %s: %v", key, err)
	}
	for i, st := range steps {
		// The engine only writes a step_execution once the step settles
		// (step_status is completed|failed). A step still in flight has no row
		// at all — the instance's current_step is what marks where it parked.
		if st.status == "started" {
			continue
		}
		// A compensating run is a second execution of the same step; the engine
		// numbers those from 101 so they never collide with forward attempts.
		attempt, status := 1, st.status
		if st.status == "compensated" {
			attempt, status = 101, "completed"
		}
		fin := at.Add(time.Duration(i+1) * time.Second)
		if _, err := s.pool.Exec(s.ctx, `
			INSERT INTO step_executions (instance_id, step_name, attempt, status, input, output, started_at, finished_at)
			VALUES ($1,$2,$3,$4,'{}','{}',$5,$6)
			ON CONFLICT (instance_id, step_name, attempt) DO NOTHING`,
			id, st.name, attempt, status, at.Add(time.Duration(i)*time.Second), fin); err != nil {
			fatal("step %s/%s: %v", key, st.name, err)
		}
	}
	return id
}

// approvalOrgTxn: pending request at 1/2 signatures — L1 already approved.
func (s *seeder) approvalOrgTxn(txnID, orgID, instID, policyID, maker, l1 uuid.UUID) {
	snap, _ := json.Marshal(map[string]any{
		"policy_id": policyID, "name": "Large spend", "version": 3,
		"tiers": []map[string]int{{"level": 1, "required_approvals": 1}, {"level": 2, "required_approvals": 1}},
	})
	id := fixedUUID("approval:org:" + txnID.String())
	if _, err := s.pool.Exec(s.ctx, `
		INSERT INTO approval_requests (id, subject_type, subject_id, org_id, instance_id, policy_id,
		  policy_snapshot, status, current_level, created_by, expires_at)
		VALUES ($1,'org_transaction',$2,$3,$4,$5,$6,'pending',2,$7, now() + interval '24 hours')
		ON CONFLICT (id) DO NOTHING`,
		id, txnID, orgID, instID, policyID, snap, maker); err != nil {
		fatal("approval: %v", err)
	}
	if _, err := s.pool.Exec(s.ctx, `
		INSERT INTO approval_actions (request_id, checker_id, level, action)
		VALUES ($1,$2,1,'approve') ON CONFLICT (request_id, checker_id) DO NOTHING`,
		id, l1); err != nil {
		fatal("approval action: %v", err)
	}
}

func (s *seeder) reversalApproval(txnID uuid.UUID) {
	var adminID uuid.UUID
	if err := s.pool.QueryRow(s.ctx,
		`SELECT id FROM admin_users ORDER BY created_at LIMIT 1`).Scan(&adminID); err != nil {
		fatal("admin user missing — run migrations: %v", err)
	}
	snap := `{"policy_id":"00000000-0000-0000-0000-000000000000","name":"reversal","version":1,"tiers":[{"level":1,"required_approvals":1}]}`
	if _, err := s.pool.Exec(s.ctx, `
		INSERT INTO approval_requests (id, subject_type, subject_id, policy_snapshot, status,
		  current_level, reason, created_by, expires_at)
		VALUES ($1,'reversal',$2,$3,'pending',1,'confirmed fraud — reverse and refund (seed)',$4, now() + interval '24 hours')
		ON CONFLICT (id) DO NOTHING`,
		fixedUUID("approval:reversal:"+txnID.String()), txnID, snap, adminID); err != nil {
		fatal("reversal approval: %v", err)
	}
}

func (s *seeder) fraudFlag(txnID uuid.UUID, rule string, score int, reasons, status string) {
	if _, err := s.pool.Exec(s.ctx, `
		INSERT INTO fraud_flags (id, transaction_id, rule_code, score, reasons, status, reviewed_at)
		VALUES ($1,$2,$3,$4,$5::jsonb,$6::flag_status, CASE WHEN $6::flag_status='open' THEN NULL ELSE now() END)
		ON CONFLICT (id) DO NOTHING`,
		fixedUUID("flag:"+rule+":"+txnID.String()), txnID, rule, score, reasons, status); err != nil {
		fatal("flag: %v", err)
	}
}

func (s *seeder) kycRequest(userID uuid.UUID) {
	if _, err := s.pool.Exec(s.ctx, `
		INSERT INTO kyc_requests (id, user_id, id_card_url, selfie_url, ai_score, ai_verdict, ai_reasons, status)
		VALUES ($1,$2,'seed/kyc/id-card.png','seed/kyc/selfie.png',64,'manual_review',
		  '["ID photo is slightly blurry at the right corner","Name matches but face confidence is 71%","A reviewer needs to confirm"]'::jsonb,
		  'manual_review')
		ON CONFLICT (id) DO UPDATE SET id_card_url = EXCLUDED.id_card_url, selfie_url = EXCLUDED.selfie_url`, fixedUUID("kyc:"+userID.String()), userID); err != nil {
		fatal("kyc: %v", err)
	}
}

func (s *seeder) reconTopup(key string, from, to uuid.UUID, amount int64, by uuid.UUID, at time.Time) string {
	if id, ok := s.existingTxn(key); ok {
		var ref string
		_ = s.pool.QueryRow(s.ctx, `SELECT ref_code FROM transactions WHERE id=$1`, id).Scan(&ref)
		return ref
	}
	tx, err := s.pool.Begin(s.ctx)
	if err != nil {
		fatal("begin: %v", err)
	}
	defer tx.Rollback(s.ctx)
	ref := wallet.NewRefCode()
	var id uuid.UUID
	if err := tx.QueryRow(s.ctx, `
		INSERT INTO transactions (ref_code, idempotency_key, type, status,
		  sender_wallet_id, receiver_wallet_id, amount, fee, initiated_by, created_at, completed_at)
		VALUES ($1,$2,'topup','completed',$3,$4,$5,0,$6,$7,$7) RETURNING id`,
		ref, key, from, to, amount, by, at).Scan(&id); err != nil {
		fatal("recon topup %s: %v", key, err)
	}
	if err := ledger.Post(s.ctx, tx, id, ledger.KindForward, []ledger.Leg{
		{WalletID: from, Direction: store.EntryDirectionDebit, Amount: amount},
		{WalletID: to, Direction: store.EntryDirectionCredit, Amount: amount},
	}); err != nil {
		fatal("recon ledger %s: %v", key, err)
	}
	if err := tx.Commit(s.ctx); err != nil {
		fatal("commit %s: %v", key, err)
	}
	return ref
}

func (s *seeder) statement(ref string, amount int64, at time.Time, kind string) {
	var n int
	if err := s.pool.QueryRow(s.ctx,
		`SELECT count(*) FROM bank_statements WHERE ref_code=$1`, ref).Scan(&n); err != nil {
		fatal("stmt check: %v", err)
	}
	if n > 0 {
		return
	}
	if _, err := s.pool.Exec(s.ctx, `
		INSERT INTO bank_statements (ref_code, amount, stmt_date, raw)
		VALUES ($1,$2,$3,jsonb_build_object('seed',true,'kind',$4::text))`,
		ref, amount, at, kind); err != nil {
		fatal("stmt: %v", err)
	}
}

func (s *seeder) reconRun(day time.Time) {
	if _, err := s.pool.Exec(s.ctx, `
		INSERT INTO reconciliation_runs (run_date, total, matched, mismatched, ai_summary)
		VALUES ($1::date, 6, 3, 3,
		 'Reconciliation for yesterday: 3 of 6 matched exactly. 3 bank statement lines have no matching internal transaction and all fall in the 23:58–00:02 window, most likely an end-of-day cutoff mismatch; compare with the next morning batch before treating them as losses.')
		ON CONFLICT (run_date) DO NOTHING`, day); err != nil {
		fatal("recon run: %v", err)
	}
}

func (s *seeder) moneyRequest(key string, requester, payer uuid.UUID, amount int64, note string, split *uuid.UUID, status string, paidTxn uuid.UUID) {
	var paid any
	if paidTxn != uuid.Nil {
		paid = paidTxn
	}
	var sg any
	if split != nil {
		sg = *split
	}
	if _, err := s.pool.Exec(s.ctx, `
		INSERT INTO money_requests (id, requester_id, payer_id, amount, note, split_group_id, status, paid_txn_id, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8, now() + interval '48 hours')
		ON CONFLICT (id) DO NOTHING`,
		fixedUUID("mreq:"+key), requester, payer, amount, note, sg, status, paid); err != nil {
		fatal("money request %s: %v", key, err)
	}
}

func (s *seeder) assertLedger() {
	bad := 0
	for id, label := range s.touched {
		var balance, sum int64
		if err := s.pool.QueryRow(s.ctx, `SELECT balance FROM wallets WHERE id=$1`, id).Scan(&balance); err != nil {
			fatal("assert %s: %v", label, err)
		}
		if err := s.pool.QueryRow(s.ctx, `
			SELECT COALESCE(SUM(CASE direction WHEN 'credit' THEN amount ELSE -amount END),0)
			FROM ledger_entries WHERE wallet_id=$1`, id).Scan(&sum); err != nil {
			fatal("assert %s: %v", label, err)
		}
		if balance != sum {
			fmt.Fprintf(os.Stderr, "LEDGER DRIFT %-22s balance=%d sum=%d\n", label, balance, sum)
			bad++
		}
	}
	if bad > 0 {
		fatal("%d wallet(s) drifted from ledger — seed aborted, inspect above", bad)
	}
}

func (s *seeder) summary() {
	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Println(" HPay demo seed OK — password for every user: Demo@123")
	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Println(" 0900000001  Hieu Mai      verified, $498.00, vault 'Holiday fund' $168.00 of $240.00")
	fmt.Println(" 0900000002  Minh Anh      verified, P2P counterpart, 1 money request pending")
	fmt.Println(" 0900000003  (queue)       KYC manual_review, ai_score 64 → admin KYC queue")
	fmt.Println(" 0900000004  (locked)      confirmed_fraud + reversal pending in admin My Tasks")
	fmt.Println(" 0900000011-14 Acme Corp    owner/maker/checker L1/L2 — policy 'Large spend' v3")
	fmt.Println("   org wallet $33,700.00 · 1 disbursement waiting 1/2 signatures ($1,800.00)")
	fmt.Println(" Admin monitor: instances failed@MOVE_FUNDS · compensated · processing_async")
	fmt.Println(" Recon yesterday: 3 matched + 3 missing_external in the 23:58–00:02 window")
	fmt.Println(" Demo flow: login 0900000001 → Home → History → Split tracking;")
	fmt.Println("            admin → KYC queue → Fraud flags → My Tasks → Recon report")
	fmt.Println("─────────────────────────────────────────────────────────")
	var bal int64
	_ = s.pool.QueryRow(s.ctx,
		`SELECT balance FROM wallets WHERE owner_type='user'
		   AND owner_id=(SELECT id FROM users WHERE phone='0900000001') AND purpose='main'`).Scan(&bal)
	fmt.Printf(" sanity: u1 main balance = %d (want 49800)\n", bal)
}
