-- 回滚陪诊基础服务费配置
UPDATE service_skus
SET rules = JSON_REMOVE(COALESCE(rules, JSON_OBJECT()), '$.base_item')
WHERE service_type = 'accompany'
  AND plan = 'single';
