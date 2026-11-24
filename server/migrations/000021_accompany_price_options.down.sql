-- 回滚：清掉档位配置（旧的多选服务项配置无法还原，回滚后陪诊需在后台重新配置）
UPDATE service_skus
SET rules = JSON_REMOVE(COALESCE(rules, JSON_OBJECT()), '$.service_items'),
    updated_at = NOW(3)
WHERE service_type = 'accompany';
