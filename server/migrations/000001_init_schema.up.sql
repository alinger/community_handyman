-- 000001 初始化九张业务表
-- 对应需求文档第九章：用户、地址、服务范围、服务产品、时段库存、订单、订单节点、服务记录、报价单
-- 约定：金额 BIGINT 单位分；时间 DATETIME(3)；软删除用 deleted_at

CREATE TABLE users (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    openid       VARCHAR(64)  NOT NULL COMMENT '微信 openid',
    nickname     VARCHAR(64)  NOT NULL DEFAULT '',
    avatar       VARCHAR(512) NOT NULL DEFAULT '',
    phone        VARCHAR(20)  NOT NULL DEFAULT '',
    default_area VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '默认小区',
    role         VARCHAR(16)  NOT NULL DEFAULT 'user' COMMENT 'user/provider/admin',
    created_at   DATETIME(3)  NOT NULL,
    updated_at   DATETIME(3)  NOT NULL,
    deleted_at   DATETIME(3)  NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_users_openid (openid),
    KEY idx_users_deleted_at (deleted_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT ='用户';

CREATE TABLE addresses (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id         BIGINT UNSIGNED NOT NULL,
    community       VARCHAR(64)  NOT NULL COMMENT '小区',
    lane            VARCHAR(64)  NOT NULL COMMENT '路、弄',
    house_no        VARCHAR(32)  NOT NULL COMMENT '门牌号',
    extra           VARCHAR(255) NOT NULL DEFAULT '' COMMENT '补充说明',
    contact_phone   VARCHAR(20)  NOT NULL DEFAULT '',
    in_service_area TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '是否命中服务范围',
    area_id         BIGINT UNSIGNED NULL COMMENT '命中的范围规则',
    created_at      DATETIME(3)  NOT NULL,
    updated_at      DATETIME(3)  NOT NULL,
    deleted_at      DATETIME(3)  NULL,
    PRIMARY KEY (id),
    KEY idx_addresses_user_id (user_id),
    KEY idx_addresses_area_id (area_id),
    KEY idx_addresses_deleted_at (deleted_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT ='用户地址';

CREATE TABLE service_areas (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    community    VARCHAR(64)  NOT NULL COMMENT '小区名',
    lane         VARCHAR(128) NOT NULL COMMENT '路、弄，多段用顿号分隔',
    house_from   INT          NOT NULL DEFAULT 0 COMMENT '起号，0 表示不限',
    house_to     INT          NOT NULL DEFAULT 0 COMMENT '止号，0 表示不限',
    house_parity VARCHAR(8)   NOT NULL DEFAULT 'all' COMMENT 'all/odd/even',
    status       TINYINT      NOT NULL DEFAULT 1 COMMENT '1 启用 0 停用',
    effective_at DATETIME(3)  NULL,
    created_at   DATETIME(3)  NOT NULL,
    updated_at   DATETIME(3)  NOT NULL,
    deleted_at   DATETIME(3)  NULL,
    PRIMARY KEY (id),
    KEY idx_service_areas_community (community),
    KEY idx_service_areas_status (status),
    KEY idx_service_areas_deleted_at (deleted_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT ='服务范围';

CREATE TABLE service_skus (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    service_type VARCHAR(16) NOT NULL COMMENT 'garbage/dogwalk/accompany',
    plan         VARCHAR(16) NOT NULL COMMENT 'single/monthly',
    base_price   BIGINT      NOT NULL DEFAULT 0 COMMENT '基础价，单位分',
    rules        JSON        NULL COMMENT '计价规则',
    slots        JSON        NULL COMMENT '可约时段',
    status       TINYINT     NOT NULL DEFAULT 1,
    created_at   DATETIME(3) NOT NULL,
    updated_at   DATETIME(3) NOT NULL,
    deleted_at   DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_service_skus_type_plan (service_type, plan),
    KEY idx_service_skus_deleted_at (deleted_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT ='服务产品';

CREATE TABLE slot_inventories (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    service_date DATE        NOT NULL,
    slot_code    VARCHAR(16) NOT NULL COMMENT '08-10 / 12-14 / 18-20',
    service_type VARCHAR(16) NOT NULL,
    total        INT         NOT NULL DEFAULT 0 COMMENT '可接单量',
    used         INT         NOT NULL DEFAULT 0 COMMENT '已占用',
    created_at   DATETIME(3) NOT NULL,
    updated_at   DATETIME(3) NOT NULL,
    deleted_at   DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_slot (service_date, slot_code, service_type),
    KEY idx_slot_service_date (service_date),
    KEY idx_slot_deleted_at (deleted_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT ='时段库存';

CREATE TABLE orders (
    id               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    order_no         VARCHAR(32)  NOT NULL COMMENT '对外订单号',
    user_id          BIGINT UNSIGNED NOT NULL,
    service_type     VARCHAR(16)  NOT NULL,
    plan             VARCHAR(16)  NOT NULL,
    address_id       BIGINT UNSIGNED NOT NULL,
    address_snapshot JSON         NULL COMMENT '下单时的地址快照',
    service_date     DATE         NOT NULL,
    slot_code        VARCHAR(16)  NOT NULL,
    quantity         INT          NOT NULL DEFAULT 1 COMMENT '袋数',
    pet_count        INT          NOT NULL DEFAULT 0,
    duration_min     INT          NOT NULL DEFAULT 0,
    distance_km      DECIMAL(5, 2) NULL,
    garbage_type     VARCHAR(16)  NOT NULL DEFAULT '',
    remark           VARCHAR(200) NOT NULL DEFAULT '',
    pricing_snapshot JSON         NULL COMMENT '费用明细快照',
    total_amount     BIGINT       NOT NULL DEFAULT 0 COMMENT '应付，单位分',
    paid_amount      BIGINT       NOT NULL DEFAULT 0,
    refunded_amount  BIGINT       NOT NULL DEFAULT 0,
    status           VARCHAR(24)  NOT NULL,
    provider_id      BIGINT UNSIGNED NULL,
    paid_at          DATETIME(3)  NULL,
    finished_at      DATETIME(3)  NULL,
    cancel_reason    VARCHAR(200) NOT NULL DEFAULT '',
    created_at       DATETIME(3)  NOT NULL,
    updated_at       DATETIME(3)  NOT NULL,
    deleted_at       DATETIME(3)  NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_orders_order_no (order_no),
    KEY idx_orders_user_status (user_id, status),
    KEY idx_orders_status (status),
    KEY idx_orders_provider_id (provider_id),
    KEY idx_orders_service_date (service_date),
    KEY idx_orders_deleted_at (deleted_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT ='订单';

CREATE TABLE order_events (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    order_id    BIGINT UNSIGNED NOT NULL,
    node        VARCHAR(32) NOT NULL COMMENT '节点标识',
    operator    VARCHAR(32) NOT NULL COMMENT 'user/provider/admin/system',
    operator_id BIGINT UNSIGNED NULL,
    remark      VARCHAR(255) NOT NULL DEFAULT '',
    created_at  DATETIME(3) NOT NULL,
    updated_at  DATETIME(3) NOT NULL,
    deleted_at  DATETIME(3) NULL,
    PRIMARY KEY (id),
    KEY idx_order_events_order_id (order_id),
    KEY idx_order_events_deleted_at (deleted_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT ='订单节点，只追加不修改';

CREATE TABLE service_records (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    order_id      BIGINT UNSIGNED NOT NULL,
    media_type    VARCHAR(16)  NOT NULL COMMENT 'photo/video/advice/document',
    object_key    VARCHAR(255) NOT NULL,
    url           VARCHAR(512) NOT NULL DEFAULT '',
    size          BIGINT       NOT NULL DEFAULT 0,
    duration_sec  INT          NOT NULL DEFAULT 0,
    uploader_id   BIGINT UNSIGNED NOT NULL DEFAULT 0,
    uploader_role VARCHAR(16)  NOT NULL DEFAULT '',
    created_at    DATETIME(3)  NOT NULL,
    updated_at    DATETIME(3)  NOT NULL,
    deleted_at    DATETIME(3)  NULL,
    PRIMARY KEY (id),
    KEY idx_service_records_order_id (order_id),
    KEY idx_service_records_deleted_at (deleted_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT ='服务记录';

CREATE TABLE quotes (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    order_id        BIGINT UNSIGNED NOT NULL,
    amount          BIGINT       NOT NULL COMMENT '报价金额，单位分',
    duration_min    INT          NOT NULL DEFAULT 0,
    negotiation_log JSON         NULL,
    confirmed_at    DATETIME(3)  NULL,
    operator_id     BIGINT UNSIGNED NOT NULL DEFAULT 0,
    created_at      DATETIME(3)  NOT NULL,
    updated_at      DATETIME(3)  NOT NULL,
    deleted_at      DATETIME(3)  NULL,
    PRIMARY KEY (id),
    KEY idx_quotes_order_id (order_id),
    KEY idx_quotes_deleted_at (deleted_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT ='陪诊报价单';
