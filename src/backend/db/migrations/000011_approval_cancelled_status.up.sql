-- Adds a terminal status distinct from 'rejected' for when the MAKER cancels
-- their own pending request, as opposed to a checker rejecting it (docs plan
-- approval-cancel). This file contains ONLY this statement: ALTER TYPE ...
-- ADD VALUE cannot run in the same transaction as any statement that uses the
-- new value (gen-migration skill rule #3).
ALTER TYPE approval_status ADD VALUE 'cancelled';
