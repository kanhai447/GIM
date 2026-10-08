CREATE TABLE friends (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    friend_id BIGINT UNSIGNED NOT NULL,
    remark VARCHAR(128) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY ux_friends_user_friend (user_id, friend_id),
    KEY ix_friends_friend_user (friend_id, user_id),
    CONSTRAINT ck_friends_distinct_users CHECK (user_id <> friend_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE friend_verifies (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    requester_id BIGINT UNSIGNED NOT NULL,
    receiver_id BIGINT UNSIGNED NOT NULL,
    status TINYINT UNSIGNED NOT NULL DEFAULT 0,
    message VARCHAR(128) NOT NULL DEFAULT '',
    pending_low_id BIGINT UNSIGNED GENERATED ALWAYS AS (CASE WHEN status = 0 THEN LEAST(requester_id, receiver_id) ELSE NULL END) STORED,
    pending_high_id BIGINT UNSIGNED GENERATED ALWAYS AS (CASE WHEN status = 0 THEN GREATEST(requester_id, receiver_id) ELSE NULL END) STORED,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY ux_friend_verifies_pending_pair (pending_low_id, pending_high_id),
    KEY ix_friend_verifies_receiver_status_created (receiver_id, status, created_at, id),
    KEY ix_friend_verifies_requester_created (requester_id, created_at, id),
    CONSTRAINT ck_friend_verifies_distinct_users CHECK (requester_id <> receiver_id),
    CONSTRAINT ck_friend_verifies_status CHECK (status BETWEEN 0 AND 2)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
