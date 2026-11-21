DELETE FROM workflow_definitions WHERE name = 'org_disbursement' AND version = 2;
UPDATE workflow_definitions SET is_active = true WHERE name = 'org_disbursement' AND version = 1;

DELETE FROM workflow_definitions WHERE name = 'p2p_transfer' AND version = 3;
UPDATE workflow_definitions SET is_active = true WHERE name = 'p2p_transfer' AND version = 2;
