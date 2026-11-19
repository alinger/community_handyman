-- 000004 补全遛宠计价规则：距离阈值显式化、加价日期可配置
-- Q-05 决议：加价日期由后台配置，改节日不发版。当前配置春节两天。

UPDATE service_skus
SET rules = JSON_SET(COALESCE(rules, JSON_OBJECT()),
                     '$.distance_threshold_km', 3,
                     '$.holiday_dates', JSON_ARRAY('2027-02-06', '2027-02-07'))
WHERE service_type = 'dogwalk'
  AND plan = 'single';
