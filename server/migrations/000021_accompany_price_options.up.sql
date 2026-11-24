-- 陪诊计价改为「档位单选」：服务项列表换成后台可配的时长档位（默认 4 档），
-- 单选一档即订单金额；旧的多选求和与基础服务费（base_item）、手动报价开关一并废弃。
-- 医院列表保持不变。
UPDATE service_skus
SET rules = JSON_SET(
        JSON_REMOVE(COALESCE(rules, JSON_OBJECT()), '$.base_item', '$.manual_quote', '$.billing'),
        '$.service_items',
        JSON_ARRAY(
            JSON_OBJECT('code', 'dur_20', 'name', '20分钟', 'price', 2500, 'minutes', 20),
            JSON_OBJECT('code', 'dur_30', 'name', '30分钟', 'price', 3500, 'minutes', 30),
            JSON_OBJECT('code', 'dur_40', 'name', '40分钟', 'price', 4500, 'minutes', 40),
            JSON_OBJECT('code', 'dur_60', 'name', '60分钟', 'price', 6000, 'minutes', 60)
        )
    ),
    updated_at = NOW(3)
WHERE service_type = 'accompany';
