-- 000003 回滚：只删除本次种子，避免误清业务数据
DELETE
FROM slot_inventories
WHERE service_date BETWEEN CURDATE() AND DATE_ADD(CURDATE(), INTERVAL 6 DAY);

DELETE
FROM service_skus
WHERE service_type IN ('garbage', 'dogwalk', 'accompany')
  AND plan IN ('single', 'monthly');

DELETE
FROM service_areas
WHERE community IN ('东方冠郡一期', '东方冠郡二期', '万科清林径', '兴欣花苑', '同润欣汇名邸',
                    '同润新云都会', '招商象屿星耀翠湾', '中粮首创禧瑞祥云', '丝庐雅苑一期',
                    '丝庐雅苑二期', '中洲华庭', '吉宝汇锦城一期');
