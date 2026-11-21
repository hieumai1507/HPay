-- ADR-0014: a money request that needs authentication is held in awaiting_auth
-- until the server verifies the credential; it never touches the ledger before that.
-- This file contains ONLY this statement (ALTER TYPE ... ADD VALUE cannot share a
-- transaction with statements that use the new value).
ALTER TYPE txn_status ADD VALUE 'awaiting_auth';