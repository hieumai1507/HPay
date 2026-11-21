-- Wire the existing ActionPipeline handler (docs 06 Task 2 / mockup 07's
-- Action Priority column) into the workflow definitions that actually run
-- transactions. The handler and services.action_priority/actions have existed
-- since migration 000003 but no workflow step ever referenced handler
-- "action_pipeline" — this closes that gap.
UPDATE workflow_definitions SET is_active = false WHERE name = 'p2p_transfer' AND version = 2;

INSERT INTO workflow_definitions (name, version, steps, is_active)
VALUES (
  'p2p_transfer', 3, '[
    {"name":"VALIDATE","type":"auto","handler":"validator","retry":3,"timeout_ms":5000,"on_failure":"fail"},
    {"name":"FRAUD_CHECK","type":"auto","handler":"fraud","retry":2,"timeout_ms":10000,"on_failure":"fail"},
    {"name":"ACTIONS_BEFORE_GL","type":"auto","handler":"action_pipeline","retry":2,"timeout_ms":10000,"on_failure":"fail"},
    {"name":"MOVE_FUNDS","type":"auto","handler":"wallet","retry":3,"timeout_ms":5000,"on_failure":"compensate","config":{"compensatable":true}},
    {"name":"ACTIONS_AFTER_GL","type":"auto","handler":"action_pipeline","retry":2,"timeout_ms":10000,"on_failure":"fail"},
    {"name":"CONFIRM","type":"auto","handler":"confirmer","retry":3,"timeout_ms":5000,"on_failure":"fail"},
    {"name":"ROUND_UP","type":"auto","handler":"round_up","retry":3,"timeout_ms":5000,"on_failure":"fail"}
  ]'::jsonb, true
)
ON CONFLICT ON CONSTRAINT wf_def_unique DO NOTHING;

UPDATE workflow_definitions SET is_active = false WHERE name = 'org_disbursement' AND version = 1;

INSERT INTO workflow_definitions (name, version, steps, is_active)
VALUES (
  'org_disbursement', 2, '[
    {"name":"VALIDATE","type":"auto","handler":"validator","retry":3,"timeout_ms":5000,"on_failure":"fail"},
    {"name":"FRAUD_CHECK","type":"auto","handler":"fraud","retry":2,"timeout_ms":10000,"on_failure":"fail"},
    {"name":"ACTIONS_BEFORE_GL","type":"auto","handler":"action_pipeline","retry":2,"timeout_ms":10000,"on_failure":"fail"},
    {"name":"WAIT_APPROVAL","type":"human_approval","retry":0,"timeout_ms":86400000,"on_failure":"fail"},
    {"name":"MOVE_FUNDS","type":"auto","handler":"wallet","retry":3,"timeout_ms":5000,"on_failure":"compensate","config":{"compensatable":true}},
    {"name":"CONFIRM","type":"auto","handler":"confirmer","retry":3,"timeout_ms":5000,"on_failure":"fail"}
  ]'::jsonb, true
)
ON CONFLICT ON CONSTRAINT wf_def_unique DO NOTHING;
