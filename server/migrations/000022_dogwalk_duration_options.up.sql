-- 遛宠时长计价改为「档位表」：后台逐档配置，单选一档即订单金额。
-- 默认档位与旧公式等价（基础价 ¥40 含 20 分钟，每超 10 分钟加 ¥10）：
-- 20分钟 ¥40 / 30分钟 ¥50 / 40分钟 ¥60 / 60分钟 ¥80。
-- 附加项（多宠/距离/熟悉费/节日加价）配置不变。
UPDATE service_skus
SET rules = JSON_SET(
        COALESCE(rules, JSON_OBJECT()),
        '$.duration_options',
        JSON_ARRAY(
            JSON_OBJECT('code', 'dur_20', 'name', '20分钟', 'price', 4000, 'minutes', 20),
            JSON_OBJECT('code', 'dur_30', 'name', '30分钟', 'price', 5000, 'minutes', 30),
            JSON_OBJECT('code', 'dur_40', 'name', '40分钟', 'price', 6000, 'minutes', 40),
            JSON_OBJECT('code', 'dur_60', 'name', '60分钟', 'price', 8000, 'minutes', 60)
        )
    ),
    updated_at = NOW(3)
WHERE service_type = 'dogwalk'
  AND plan = 'single';
