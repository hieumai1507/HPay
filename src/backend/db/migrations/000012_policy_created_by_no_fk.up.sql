-- approval_policies.created_by is set by an org member (users.id) via the
-- self-service mobile path, but also needs to be settable by a platform
-- admin (admin_users.id) via the ops override path (hpay-admin "Tổ chức &
-- Policy"). Same precedent as approval_actions.checker_id and
-- approval_requests.created_by: drop the single-table FK so the column can
-- hold either kind of actor id — the app layer, not the DB, disambiguates
-- which table a given id belongs to.
ALTER TABLE approval_policies DROP CONSTRAINT IF EXISTS approval_policies_created_by_fkey;
