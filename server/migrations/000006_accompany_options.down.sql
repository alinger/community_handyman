-- 000006 回滚
UPDATE service_skus
SET rules = JSON_REMOVE(rules, '$.hospitals', '$.service_items', '$.billing')
WHERE service_type = 'accompany'
  AND plan = 'single';

ALTER TABLE orders
    DROP COLUMN service_items,
    DROP COLUMN patient_name,
    DROP COLUMN hospital;

ALTER TABLE addresses
    DROP COLUMN contact_name;
