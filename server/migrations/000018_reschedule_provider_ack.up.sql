-- 住户改期后服务方的知悉标记。
-- 住户改期立即生效，服务方可能正按旧时间安排行程：不告诉他就会白跑一趟。
-- 订阅消息只发一次且可能被忽略，所以站内要一直提醒到他确认为止。
-- provider_ack_at 为 NULL 表示「这次改期他还没看到/没确认」。

ALTER TABLE order_reschedules
    ADD COLUMN provider_ack_at DATETIME NULL COMMENT '服务方确认知悉的时间，NULL 表示未确认' AFTER handled_at;

-- 存量记录一律视为已读。
-- 这些改期发生在还没有提醒功能的时候，补回提醒只会让服务方升级当天
-- 一打开任务列表就看到一堆早已过去的时间变更，反而淹没真正的新单。
UPDATE order_reschedules
SET provider_ack_at = COALESCE(handled_at, updated_at)
WHERE initiated_by = 'user'
  AND status = 'applied'
  AND provider_ack_at IS NULL;
