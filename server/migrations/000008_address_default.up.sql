-- 地址默认标记：每个用户至多一条默认地址
ALTER TABLE addresses
    ADD COLUMN is_default TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否默认地址' AFTER contact_phone;

-- 存量数据兼容：每个用户把最早的一条地址设为默认，避免升级后没有默认地址
UPDATE addresses a
JOIN (
    SELECT MIN(id) AS id, user_id
    FROM addresses
    GROUP BY user_id
) first_addr ON first_addr.id = a.id AND first_addr.user_id = a.user_id
SET a.is_default = 1;

-- 加速按用户查默认地址
ALTER TABLE addresses
    ADD INDEX idx_addresses_user_default (user_id, is_default);
