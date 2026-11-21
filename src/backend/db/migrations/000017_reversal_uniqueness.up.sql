-- ADR-0013 / audit BE-H3: one reversal per transaction, enforced by the database.
-- The comments in reversal.go and transactions.sql referred to these constraints,
-- but no migration ever created them.
--
-- Guard: if duplicate reversals already exist the index cannot be built; fail
-- loudly with the offending ids instead of picking a winner.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM transactions WHERE reversal_of IS NOT NULL GROUP BY reversal_of HAVING count(*) > 1) THEN
    RAISE EXCEPTION 'duplicate reversals exist for: %',
      (SELECT string_agg(reversal_of::text, ', ') FROM (
         SELECT reversal_of FROM transactions WHERE reversal_of IS NOT NULL GROUP BY reversal_of HAVING count(*) > 1) d);
  END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS one_reversal_per_txn
  ON transactions (reversal_of) WHERE reversal_of IS NOT NULL;

-- At most one PENDING approval per subject (a finished request does not block a
-- later one, e.g. after a rejection or expiry).
CREATE UNIQUE INDEX IF NOT EXISTS one_pending_approval_per_subject
  ON approval_requests (subject_type, subject_id) WHERE status = 'pending';