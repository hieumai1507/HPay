-- Wave 2 / ADR-0009: a failed compensation (the reverse posting did not happen) is its own\n-- terminal state so operators can find it. Only this statement in this file (enum rule).
ALTER TYPE wf_status ADD VALUE 'compensation_failed';
