-- 000006 陪诊服务选项化 + 地址联系人
-- 1) 地址增加「联系人」（联系方式 contact_phone 建表时已有）
ALTER TABLE addresses
    ADD COLUMN contact_name VARCHAR(32) NOT NULL DEFAULT '' COMMENT '联系人' AFTER contact_phone;

-- 2) 订单结构化保存陪诊选择，不再只把信息塞进 remark
ALTER TABLE orders
    ADD COLUMN hospital      VARCHAR(64) NOT NULL DEFAULT '' COMMENT '就诊医院 code' AFTER garbage_type,
    ADD COLUMN patient_name  VARCHAR(32) NOT NULL DEFAULT '' COMMENT '就诊人' AFTER hospital,
    ADD COLUMN service_items JSON        NULL COMMENT '已选陪诊服务项 code 数组' AFTER patient_name;

-- 3) 陪诊 SKU 规则：医院改为后台可配置选项、服务项带可配置单价（单位：分）
--    金额 = 所选服务项 price 之和，base_price 不再参与陪诊计价。
--    价格只存这里，前端只回传 code，杜绝前端篡改金额。
UPDATE service_skus
SET rules = JSON_SET(COALESCE(rules, JSON_OBJECT()),
                     '$.billing', 'itemized',
                     '$.hospitals', JSON_ARRAY(
                         JSON_OBJECT('code', 'renji_east', 'name', '仁济医院东院'),
                         JSON_OBJECT('code', 'renji_west', 'name', '仁济医院西院'),
                         JSON_OBJECT('code', 'zhoupu', 'name', '周浦医院'),
                         JSON_OBJECT('code', 'pudong_people', 'name', '浦东新区人民医院'),
                         JSON_OBJECT('code', 'dongfang_south', 'name', '东方医院南院')
                     ),
                     '$.service_items', JSON_ARRAY(
                         JSON_OBJECT('code', 'registration', 'name', '取号', 'price', 500),
                         JSON_OBJECT('code', 'pickup_supplies', 'name', '代取医疗用品', 'price', 1000),
                         JSON_OBJECT('code', 'guide', 'name', '引导就诊', 'price', 2000),
                         JSON_OBJECT('code', 'exam_booking', 'name', '预约检查', 'price', 1000),
                         JSON_OBJECT('code', 'sample', 'name', '样本送检', 'price', 1500),
                         JSON_OBJECT('code', 'medicine', 'name', '取药取报告', 'price', 1000),
                         JSON_OBJECT('code', 'record', 'name', '记录医嘱', 'price', 1500),
                         JSON_OBJECT('code', 'docs', 'name', '代取文书', 'price', 1000),
                         JSON_OBJECT('code', 'admission', 'name', '出入院手续', 'price', 2000),
                         JSON_OBJECT('code', 'payment', 'name', '缴费', 'price', 500),
                         JSON_OBJECT('code', 'daily_care', 'name', '生活辅助', 'price', 2000),
                         JSON_OBJECT('code', 'emotional', 'name', '情感支持', 'price', 1000)
                     ))
WHERE service_type = 'accompany'
  AND plan = 'single';
