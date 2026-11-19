DROP INDEX uk_users_phone ON users;

ALTER TABLE users DROP COLUMN phone_key;

ALTER TABLE users DROP COLUMN password_hash;
