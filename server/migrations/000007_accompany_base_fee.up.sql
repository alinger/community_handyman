-- 000007 陪诊默认必选的基础服务费
-- 基础费写进陪诊 SKU 的 rules.base_item，后台可用 PUT /api/admin/skus/:id 改价，不发给前端决定。
-- 计价时强制计入，前端无法取消；未配置时保持旧的「只按所选项目求和」行为。
UPDATE service_skus
SET rules = JSON_SET(COALESCE(rules, JSON_OBJECT()),
                     '$.base_item', JSON_OBJECT('code', 'base_fee', 'name', '基础服务费', 'price', 1000))
WHERE service_type = 'accompany'
  AND plan = 'single';
