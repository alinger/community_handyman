-- 包月每天拆成两段：打卡（人到了）与本趟完成（袋子提走了）。
-- 原来只有打卡，服务方点一下「出发上门」当天就算服务完成、单子直接进已完成，
-- 中间没有「这趟到底做完没有」的确认，住户看到的也是「今日已服务」。
-- finished_at 为 NULL 表示「已上门、本趟还没收尾」，用来把这张单留在「服务中」。

ALTER TABLE monthly_redemptions
    ADD COLUMN finished_at DATETIME NULL COMMENT '本趟服务完成时间，NULL 表示已上门未完成' AFTER remark;

-- 存量核销记录都是旧流程「打卡即完成」，补成已完成的时刻，
-- 否则升级当天所有历史日期都会显示成「已上门未完成」。
UPDATE monthly_redemptions
SET finished_at = created_at
WHERE finished_at IS NULL;
