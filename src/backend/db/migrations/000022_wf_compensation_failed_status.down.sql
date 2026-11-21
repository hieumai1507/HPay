DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM workflow_instances WHERE status = 'compensation_failed') THEN
    RAISE EXCEPTION 'cannot drop enum value compensation_failed from wf_status: % row(s) still use it - resolve them first',
      (SELECT count(*) FROM workflow_instances WHERE status = 'compensation_failed');
  END IF;
END $$;

ALTER TYPE wf_status RENAME TO wf_status_old;
CREATE TYPE wf_status AS ENUM ('running', 'waiting_approval', 'compensating', 'completed', 'failed', 'compensated', 'recovering');
ALTER TABLE workflow_instances
  ALTER COLUMN status DROP DEFAULT,
  ALTER COLUMN status TYPE wf_status USING status::text::wf_status,
  ALTER COLUMN status SET DEFAULT 'running';
DROP TYPE wf_status_old;