-- 用户端手机号 + 密码登录
-- password_hash 用 bcrypt（长度 60），留 128 便于将来换算法。
ALTER TABLE users
    ADD COLUMN password_hash VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'bcrypt 密码哈希，空表示未设置密码（老账号/微信登录）' AFTER phone;

-- phone 从纯资料字段升级为登录标识，需要唯一约束。
-- 但未绑定手机号的老账号 phone 是空串，直接加唯一索引会互相冲突；
-- 用虚拟生成列把空串转成 NULL（MySQL 唯一索引允许多个 NULL），既保证非空手机号唯一，
-- 又不影响没有手机号的账号，且不需要改动 phone 的 NOT NULL 约束。
ALTER TABLE users
    ADD COLUMN phone_key VARCHAR(20) GENERATED ALWAYS AS (NULLIF(phone, '')) VIRTUAL COMMENT '手机号唯一索引载体，空串转 NULL';

ALTER TABLE users ADD UNIQUE KEY uk_users_phone (phone_key);
