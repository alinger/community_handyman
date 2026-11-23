-- 门禁与钥匙信息：需求要求「单独加密存储、仅服务开始前 2 小时至服务结束后可见」。
-- access_info 存的是 AES-256-GCM 密文，不是明文，直接用 SQL 查出来也读不懂。
ALTER TABLE orders
    ADD COLUMN entry_method VARCHAR(32) NOT NULL DEFAULT '' COMMENT '进入方式：knock 敲门 / password 门禁密码 / card 门禁卡 / property 物业代开 / other 其他',
    ADD COLUMN access_info  VARCHAR(512) NOT NULL DEFAULT '' COMMENT '门禁密码或钥匙说明，密文存储';
