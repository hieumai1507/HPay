ALTER TABLE transactions
  DROP COLUMN IF EXISTS service_code,
  DROP COLUMN IF EXISTS workflow_name;