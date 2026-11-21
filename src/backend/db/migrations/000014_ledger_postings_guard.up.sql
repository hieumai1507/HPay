-- ADR-0010: a ledger posting can happen at most once per (transaction, kind).
--
-- ledger.Post used to be applicable twice to the same transaction (workflow
-- step redelivery, BE-C1). ledger_entries cannot carry a plain unique key on
-- transaction_id because compensation and round-up legitimately post again on
-- a transaction that was already posted, so each posting gets its own guard row.
--
-- kind: forward | compensate | round_up.
-- total_amount: sum of the debit legs, stored so a replay with a DIFFERENT
-- payload for the same (transaction, kind) can be told apart from a true replay.

CREATE TYPE posting_kind AS ENUM ('forward', 'compensate', 'round_up');

CREATE TABLE IF NOT EXISTS ledger_postings (
    transaction_id UUID         NOT NULL REFERENCES transactions(id),
    kind           posting_kind NOT NULL,
    total_amount   BIGINT       NOT NULL CHECK (total_amount > 0),
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (transaction_id, kind)
);

-- Back-fill: every transaction that already has ledger entries was posted once
-- as 'forward'. Earlier compensation or round-up entries on the same
-- transaction cannot be told apart from the forward ones and are folded into
-- the forward row; a replay of those old steps is not possible, so this is
-- harmless. total_amount is the sum of the debit legs of all existing entries.
INSERT INTO ledger_postings (transaction_id, kind, total_amount)
SELECT transaction_id, 'forward', SUM(amount)
FROM ledger_entries
WHERE direction = 'debit'
GROUP BY transaction_id
ON CONFLICT DO NOTHING;