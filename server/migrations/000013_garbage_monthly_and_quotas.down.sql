-- 回滚 000013。
-- 注意：包月服务期与免费次数一旦产生业务数据，删列会丢记录，生产回滚前先备份。
DROP TABLE IF EXISTS user_quotas;

ALTER TABLE orders
    DROP COLUMN extra_slot_code,
    DROP COLUMN monthly_start,
    DROP COLUMN monthly_end,
    DROP COLUMN free_quota_used;

UPDATE service_skus
SET base_price = 20000,
    rules = JSON_SET(COALESCE(rules, JSON_OBJECT()), '$.monthly_price', 20000)
WHERE service_type = 'garbage' AND plan = 'monthly';
