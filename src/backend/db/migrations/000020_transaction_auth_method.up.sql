-- ADR-0014: remember which authentication method a transaction required at request
-- time, so confirm checks that exact method even if the offer changes in between.
-- NULL for transactions created before this migration and for requests that needed
-- no confirmation. Reuses the enum from migration 000004.
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS auth_method auth_method_enum;