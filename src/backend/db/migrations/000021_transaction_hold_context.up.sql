-- ADR-0014: at confirm time the engine must start exactly the workflow the REQUEST
-- chose, so a held request records what it resolved instead of re-deriving it later:
--   workflow_name  the workflow definition resolved from the service row;
--   service_code   the service, for the action pipeline step.
-- NULL for transactions created before this migration and for requests that needed
-- no confirmation.
ALTER TABLE transactions
  ADD COLUMN IF NOT EXISTS workflow_name VARCHAR(60),
  ADD COLUMN IF NOT EXISTS service_code  VARCHAR(40);