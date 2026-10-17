-- Recharge packages are identified by their immutable database ID.
DROP TRIGGER IF EXISTS payment_recharge_package_identity ON payment_recharge_package;
DROP FUNCTION IF EXISTS payment_recharge_package_identity_guard();
DROP INDEX IF EXISTS ux_payment_recharge_package_code_active;
ALTER TABLE IF EXISTS payment_recharge_package
  DROP CONSTRAINT IF EXISTS ck_payment_recharge_package_code,
  DROP COLUMN IF EXISTS code RESTRICT;
