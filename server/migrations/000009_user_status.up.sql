-- 用户封禁状态：1 正常，0 封禁。封禁后小程序端登录被拒绝。
ALTER TABLE users
    ADD COLUMN status TINYINT NOT NULL DEFAULT 1 COMMENT '1 正常 0 封禁' AFTER role;
