CREATE TABLE chat_config
(
    id         bigserial PRIMARY KEY,
    "user"     varchar(191) NOT NULL,
    agent_id   bigint       NOT NULL DEFAULT 0,
    model      varchar(191) NOT NULL,
    prompt     varchar(500) NOT NULL,
    created_at timestamp             DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp             DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE chat_config IS '聊天配置表';
COMMENT ON COLUMN chat_config.id IS '聊天配置ID';
COMMENT ON COLUMN chat_config."user" IS '用户标识';
COMMENT ON COLUMN chat_config.agent_id IS '应用ID';
COMMENT ON COLUMN chat_config.model IS '模型';
COMMENT ON COLUMN chat_config.prompt IS '系统初始设置';
COMMENT ON COLUMN chat_config.created_at IS '创建时间';
COMMENT ON COLUMN chat_config.updated_at IS '更新时间';

-- 加入索引 user_idx
CREATE INDEX chat_config_user_idx ON chat_config ("user", agent_id);
