CREATE TABLE bots_prompt
(
    id bigserial PRIMARY KEY,
    bot_id bigint NOT NULL DEFAULT 0,
    prompt text NOT NULL,
    created_at timestamp DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE bots_prompt IS '机器人prompt设置表';

COMMENT ON COLUMN bots_prompt.id IS '机器人初始设置ID';
COMMENT ON COLUMN bots_prompt.bot_id IS '机器人ID 关联 bots.id';
COMMENT ON COLUMN bots_prompt.prompt IS '机器人初始设置';
COMMENT ON COLUMN bots_prompt.created_at IS '创建时间';
COMMENT ON COLUMN bots_prompt.updated_at IS '更新时间';

-- 加入索引 idx_bot_id
CREATE INDEX bots_prompt_idx_bot_id ON bots_prompt(bot_id);
