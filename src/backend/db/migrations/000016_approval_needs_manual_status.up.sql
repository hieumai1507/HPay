-- ADR-0013: a reversal that cannot complete (the receiver spent the money) must
-- not roll back the approver's decision. The request is parked as 'needs_manual'
-- with its action and an audit row preserved; nothing is posted.
-- This file contains ONLY this statement: ALTER TYPE ... ADD VALUE cannot run in
-- the same transaction as any statement that uses the new value (recipe rule 3).
ALTER TYPE approval_status ADD VALUE 'needs_manual';