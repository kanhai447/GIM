CREATE TABLE settings (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    setting_key VARCHAR(128) NOT NULL,
    setting_value JSON NOT NULL,
    is_public TINYINT(1) NOT NULL DEFAULT 0,
    description VARCHAR(255) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY ux_settings_key (setting_key),
    KEY ix_settings_public_key (is_public, setting_key),
    CONSTRAINT ck_settings_key CHECK (CHAR_LENGTH(setting_key) > 0),
    CONSTRAINT ck_settings_is_public CHECK (is_public IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
