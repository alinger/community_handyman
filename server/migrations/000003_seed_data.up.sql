-- 000003 种子数据
-- 三部分：服务范围（13 条）、服务产品（4 档）、未来 7 天时段库存（含工作日遛宠只开晚间的规则）
-- 管理员账号不在此处插入，避免把密码哈希写进版本库，由 `go run ./cmd/migrate -seed-admin` 生成

-- 1. 服务范围：对齐需求文档表 1
INSERT INTO service_areas (community, lane, house_from, house_to, house_parity, status, created_at, updated_at)
VALUES ('东方冠郡一期', '汇腾路、新环东路、汇豪路、汇友路', 0, 0, 'all', 1, NOW(3), NOW(3)),
       ('东方冠郡二期', '众安路779弄', 1, 89, 'all', 1, NOW(3), NOW(3)),
       ('东方冠郡二期', '众安路879弄', 1, 39, 'all', 1, NOW(3), NOW(3)),
       ('万科清林径', '牌楼东路528弄', 0, 0, 'all', 1, NOW(3), NOW(3)),
       ('兴欣花苑', '牌楼东路102弄', 1, 0, 'all', 1, NOW(3), NOW(3)),
       ('同润欣汇名邸', '众安路880弄', 1, 28, 'all', 1, NOW(3), NOW(3)),
       ('同润新云都会', '杨辉路855弄', 1, 8, 'all', 1, NOW(3), NOW(3)),
       ('招商象屿星耀翠湾', '众安路1108弄', 0, 0, 'all', 1, NOW(3), NOW(3)),
       ('中粮首创禧瑞祥云', '杨辉路966弄', 0, 0, 'all', 1, NOW(3), NOW(3)),
       ('丝庐雅苑一期', '申江南路7777弄', 1, 9, 'all', 1, NOW(3), NOW(3)),
       ('丝庐雅苑二期', '申江南路7677弄', 0, 0, 'all', 1, NOW(3), NOW(3)),
       ('中洲华庭', '新环北路1333弄', 0, 0, 'all', 1, NOW(3), NOW(3)),
       ('吉宝汇锦城一期', '新环北路1180弄', 0, 0, 'all', 1, NOW(3), NOW(3));

-- 2. 服务产品：代倒垃圾单次与包月、遛宠单次、陪诊（价格面议）
INSERT INTO service_skus (service_type, plan, base_price, rules, slots, status, created_at, updated_at)
VALUES ('garbage', 'single', 500,
        '{"unit":"bag","bags_included":1,"extra_bag":0,"monthly_price":20000,"monthly_daily_limit":1}',
        '["08-10","12-14","18-20"]', 1, NOW(3), NOW(3)),
       ('garbage', 'monthly', 20000,
        '{"daily_limit":1,"carry_over":false}',
        '["08-10","12-14","18-20"]', 1, NOW(3), NOW(3)),
       ('dogwalk', 'single', 4000,
        '{"feed_only":3500,"base":4000,"base_minutes":20,"step_minutes":10,"step_price":1000,"extra_pet":500,"over_3km":1000,"familiar":2000,"familiar_free_orders":5,"holiday_extra":1500,"max_minutes":120}',
        '["18-20"]', 1, NOW(3), NOW(3)),
       ('accompany', 'single', 0,
        '{"manual_quote":true,"billing":"hourly","items":["取号","代取医疗用品","引导就诊","预约检查","样本送检","取药取报告","记录医嘱","代取文书","出入院手续","缴费","生活辅助","情感支持"]}',
        '["08-10","12-14"]', 1, NOW(3), NOW(3));

-- 3. 时段库存：从今天起 7 天，三个时段三种服务
-- 遛宠在工作日（WEEKDAY < 5）只开晚间，用 total = 0 表达，让规则落在数据而不是代码分支
INSERT INTO slot_inventories (service_date, slot_code, service_type, total, used, created_at, updated_at)
SELECT DATE_ADD(CURDATE(), INTERVAL d.n DAY),
       s.code,
       t.st,
       CASE
           WHEN t.st = 'dogwalk' AND WEEKDAY(DATE_ADD(CURDATE(), INTERVAL d.n DAY)) < 5
               AND s.code IN ('08-10', '12-14') THEN 0
           WHEN t.st = 'dogwalk' THEN 5
           WHEN t.st = 'garbage' THEN 20
           ELSE 3
           END,
       0,
       NOW(3),
       NOW(3)
FROM (SELECT 0 AS n
      UNION ALL SELECT 1
      UNION ALL SELECT 2
      UNION ALL SELECT 3
      UNION ALL SELECT 4
      UNION ALL SELECT 5
      UNION ALL SELECT 6) d
         CROSS JOIN (SELECT '08-10' AS code, 1 AS seq
                     UNION ALL SELECT '12-14', 2
                     UNION ALL SELECT '18-20', 3) s
         CROSS JOIN (SELECT 'garbage' AS st
                     UNION ALL SELECT 'dogwalk'
                     UNION ALL SELECT 'accompany') t;
