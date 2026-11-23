-- 回滚 000015
DROP TABLE IF EXISTS provider_income;
ALTER TABLE service_records DROP COLUMN owner_consented;
