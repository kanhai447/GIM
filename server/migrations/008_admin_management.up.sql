ALTER TABLE chat_messages
    ADD COLUMN admin_deleted_at DATETIME(3) NULL AFTER system_msg;

ALTER TABLE group_messages
    ADD COLUMN admin_deleted_at DATETIME(3) NULL AFTER system_msg;
