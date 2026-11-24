-- BUG-0017 follow-up: the daily/monthly quota and the interval throttle measure a transaction
-- from the moment it STARTS, not from when it was requested. A request can wait up to 10 minutes
-- for its credential (ADR-0014) and held requests are excluded from the quota, so measuring by
-- created_at let a request made before midnight dodge both days, and holding several requests
-- then confirming them together bypassed the throttle.
-- NULL means "not started yet" (awaiting_auth) or a row from before this column; readers use
-- COALESCE(started_at, created_at), so existing rows keep their old meaning.
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ;