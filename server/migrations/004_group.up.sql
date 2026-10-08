CREATE TABLE `groups` (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(64) NOT NULL,
    owner_id BIGINT UNSIGNED NOT NULL,
    avatar VARCHAR(256) NOT NULL DEFAULT '',
    description VARCHAR(255) NOT NULL DEFAULT '',
    notice VARCHAR(512) NOT NULL DEFAULT '',
    status TINYINT UNSIGNED NOT NULL DEFAULT 1,
    is_search TINYINT(1) NOT NULL DEFAULT 1,
    join_policy TINYINT UNSIGNED NOT NULL DEFAULT 1,
    verification_question JSON NULL,
    allow_member_invite TINYINT(1) NOT NULL DEFAULT 1,
    allow_temporary_session TINYINT(1) NOT NULL DEFAULT 1,
    is_all_muted TINYINT(1) NOT NULL DEFAULT 0,
    member_limit INT UNSIGNED NOT NULL DEFAULT 100,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY ix_groups_owner_status (owner_id, status, id),
    CONSTRAINT ck_groups_status CHECK (status IN (1, 2)),
    CONSTRAINT ck_groups_is_search CHECK (is_search IN (0, 1)),
    CONSTRAINT ck_groups_join_policy CHECK (join_policy BETWEEN 0 AND 4),
    CONSTRAINT ck_groups_allow_member_invite CHECK (allow_member_invite IN (0, 1)),
    CONSTRAINT ck_groups_allow_temporary_session CHECK (allow_temporary_session IN (0, 1)),
    CONSTRAINT ck_groups_is_all_muted CHECK (is_all_muted IN (0, 1)),
    CONSTRAINT ck_groups_member_limit CHECK (member_limit > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE group_members (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    group_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    member_nickname VARCHAR(32) NOT NULL DEFAULT '',
    role TINYINT UNSIGNED NOT NULL DEFAULT 3,
    joined_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY ux_group_members_group_user (group_id, user_id),
    KEY ix_group_members_group_role_user (group_id, role, user_id),
    KEY ix_group_members_user_group (user_id, group_id),
    CONSTRAINT ck_group_members_role CHECK (role BETWEEN 1 AND 3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE group_verifies (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    group_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    reviewer_id BIGINT UNSIGNED NULL,
    status TINYINT UNSIGNED NOT NULL DEFAULT 0,
    request_type TINYINT UNSIGNED NOT NULL DEFAULT 1,
    message VARCHAR(128) NOT NULL DEFAULT '',
    verification_answer JSON NULL,
    pending_group_id BIGINT UNSIGNED GENERATED ALWAYS AS (CASE WHEN status = 0 THEN group_id ELSE NULL END) STORED,
    pending_user_id BIGINT UNSIGNED GENERATED ALWAYS AS (CASE WHEN status = 0 THEN user_id ELSE NULL END) STORED,
    pending_request_type TINYINT UNSIGNED GENERATED ALWAYS AS (CASE WHEN status = 0 THEN request_type ELSE NULL END) STORED,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY ux_group_verifies_pending (pending_group_id, pending_user_id, pending_request_type),
    KEY ix_group_verifies_group_status_created (group_id, status, created_at, id),
    KEY ix_group_verifies_user_created (user_id, created_at, id),
    CONSTRAINT ck_group_verifies_status CHECK (status BETWEEN 0 AND 2),
    CONSTRAINT ck_group_verifies_request_type CHECK (request_type BETWEEN 1 AND 2)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE group_messages (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    group_id BIGINT UNSIGNED NOT NULL,
    send_user_id BIGINT UNSIGNED NOT NULL,
    group_member_id BIGINT UNSIGNED NOT NULL,
    client_msg_id VARCHAR(64) NOT NULL,
    msg_type TINYINT UNSIGNED NOT NULL,
    msg_preview VARCHAR(255) NOT NULL DEFAULT '',
    msg JSON NOT NULL,
    system_msg JSON NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY ux_group_messages_sender_client (send_user_id, client_msg_id),
    KEY ix_group_messages_group_cursor (group_id, id),
    KEY ix_group_messages_sender_cursor (send_user_id, id),
    KEY ix_group_messages_member_cursor (group_member_id, id),
    CONSTRAINT ck_group_messages_client_msg_id CHECK (CHAR_LENGTH(client_msg_id) > 0),
    CONSTRAINT ck_group_messages_type CHECK (msg_type > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE group_sessions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    group_id BIGINT UNSIGNED NOT NULL,
    last_message_id BIGINT UNSIGNED NULL,
    last_message_at DATETIME(3) NULL,
    last_read_message_id BIGINT UNSIGNED NULL,
    unread_count INT UNSIGNED NOT NULL DEFAULT 0,
    is_top TINYINT(1) NOT NULL DEFAULT 0,
    hidden_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY ux_group_sessions_user_group (user_id, group_id),
    KEY ix_group_sessions_user_sort (user_id, is_top DESC, last_message_at DESC, id DESC),
    KEY ix_group_sessions_group_user (group_id, user_id),
    CONSTRAINT ck_group_sessions_is_top CHECK (is_top IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE group_message_deletions (
    user_id BIGINT UNSIGNED NOT NULL,
    message_id BIGINT UNSIGNED NOT NULL,
    group_id BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (user_id, message_id),
    KEY ix_group_message_deletions_user_group (user_id, group_id, message_id),
    KEY ix_group_message_deletions_message (message_id, user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
