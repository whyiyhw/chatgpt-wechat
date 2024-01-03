CREATE TABLE chat
(
    id          bigserial PRIMARY KEY,
    "user"      varchar(191) NOT NULL,
    message_id  varchar(191) NOT NULL,
    open_kf_id  varchar(191) NOT NULL,
    agent_id    bigint       NOT NULL DEFAULT 0,
    req_content varchar(500) NOT NULL,
    res_content text         NOT NULL,
    created_at  timestamp             DEFAULT CURRENT_TIMESTAMP,
    updated_at  timestamp             DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE chat IS '聊天记录表';
COMMENT ON COLUMN chat.id IS '聊天记录ID';
COMMENT ON COLUMN chat."user" IS 'weCom用户标识/customer用户标识';
COMMENT ON COLUMN chat.message_id IS 'message_id customer消息唯一ID';
COMMENT ON COLUMN chat.open_kf_id IS '客服标识';
COMMENT ON COLUMN chat.agent_id IS '应用ID';
COMMENT ON COLUMN chat.req_content IS '用户发送内容';
COMMENT ON COLUMN chat.res_content IS 'openai响应内容';
COMMENT ON COLUMN chat.created_at IS '创建时间';
COMMENT ON COLUMN chat.updated_at IS '更新时间';

-- 加入索引 user_idx
CREATE INDEX chat_user_idx ON chat ("user", agent_id);

-- 加入索引 user_message_idx
CREATE INDEX chat_user_message_idx ON chat ("user", message_id);
