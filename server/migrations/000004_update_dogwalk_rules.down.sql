-- 000004 回滚：移除本次新增的两个键，保留原有规则
UPDATE service_skus
SET rules = JSON_REMOVE(rules, '$.distance_threshold_km', '$.holiday_dates')
WHERE service_type = 'dogwalk'
  AND plan = 'single';
