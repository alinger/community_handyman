DROP INDEX idx_addresses_user_default ON addresses;

ALTER TABLE addresses
    DROP COLUMN is_default;
