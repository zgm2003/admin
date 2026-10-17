-- Recharge packages are identified by their immutable database ID.
ALTER TABLE IF EXISTS payment_recharge_package
  DROP COLUMN IF EXISTS code RESTRICT;
