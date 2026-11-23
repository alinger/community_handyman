-- 服务记录凭证按节点区分：stage 标记这条记录属于哪个动作。
-- start=开始服务凭证（遛宠接到宠物），finish=完成服务凭证（垃圾提袋走/宠物送回），
-- 空串=服务过程中的随手记录。历史记录统一视为过程记录。
ALTER TABLE service_records
    ADD COLUMN stage VARCHAR(16) NOT NULL DEFAULT '' COMMENT '凭证阶段：start 开始服务 / finish 完成服务 / 空=过程记录';
