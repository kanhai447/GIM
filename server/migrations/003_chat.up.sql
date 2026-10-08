CREATE TABLE chat_messages (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    send_user_id BIGINT UNSIGNED NOT NULL,
    rev_user_id BIGINT UNSIGNED NOT NULL,
    client_msg_id VARCHAR(64) NOT NULL,
    msg_type TINYINT UNSIGNED NOT NULL,
    msg_preview VARCHAR(255) NOT NULL DEFAULT '',
    msg JSON NOT NULL,
    system_msg JSON NULL,
    conversation_low_id BIGINT UNSIGNED GENERATED ALWAYS AS (LEAST(send_user_id, rev_user_id)) STORED,
    conversation_high_id BIGINT UNSIGNED GENERATED ALWAYS AS (GREATEST(send_user_id, rev_user_id)) STORED,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY ux_chat_messages_sender_client (send_user_id, client_msg_id),
    KEY ix_chat_messages_conversation_cursor (conversation_low_id, conversation_high_id, id),
    KEY ix_chat_messages_sender_receiver_cursor (send_user_id, rev_user_id, id),
    KEY ix_chat_messages_receiver_sender_cursor (rev_user_id, send_user_id, id),
    KEY ix_chat_messages_created (created_at, id),
    CONSTRAINT ck_chat_messages_distinct_users CHECK (send_user_id <> rev_user_id),
    CONSTRAINT ck_chat_messages_client_msg_id CHECK (CHAR_LENGTH(client_msg_id) > 0),
    CONSTRAINT ck_chat_messages_type CHECK (msg_type > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE chat_sessions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    peer_user_id BIGINT UNSIGNED NOT NULL,
    last_message_id BIGINT UNSIGNED NULL,
    last_message_at DATETIME(3) NULL,
    last_read_message_id BIGINT UNSIGNED NULL,
    unread_count INT UNSIGNED NOT NULL DEFAULT 0,
    is_top TINYINT(1) NOT NULL DEFAULT 0,
    hidden_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY ux_chat_sessions_user_peer (user_id, peer_user_id),
    KEY ix_chat_sessions_user_sort (user_id, is_top DESC, last_message_at DESC, id DESC),
    KEY ix_chat_sessions_peer_user (peer_user_id, user_id),
    CONSTRAINT ck_chat_sessions_distinct_users CHECK (user_id <> peer_user_id),
    CONSTRAINT ck_chat_sessions_is_top CHECK (is_top IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE chat_message_deletions (
    user_id BIGINT UNSIGNED NOT NULL,
    message_id BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (user_id, message_id),
    KEY ix_chat_message_deletions_message (message_id, user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
