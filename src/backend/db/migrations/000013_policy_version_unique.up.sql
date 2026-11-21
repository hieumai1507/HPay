-- org.Service.createPolicy computes the next version with a plain
-- SELECT MAX(version)+1 (no row lock), so two concurrent saves for the same
-- (org_id, name) can both read the same max and insert the same "next"
-- version — silent duplicate-version corruption instead of the clean
-- conflict A05/E09 expect. This constraint turns that race into a real,
-- catchable unique-violation instead of letting it corrupt data; the
-- service layer maps the violation to ERR_POLICY_VERSION_CONFLICT (409).
ALTER TABLE approval_policies
  ADD CONSTRAINT approval_policies_org_name_version_key UNIQUE (org_id, name, version);
