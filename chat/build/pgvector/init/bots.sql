CREATE TABLE bots
(
    id         bigserial PRIMARY KEY,
    name       varchar(64)  NOT NULL,
    avatar     varchar(255) NOT NULL,
    "desc"     varchar(255) NOT NULL,
    user_id    bigint       NOT NULL DEFAULT 0,
    created_at timestamp             DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp             DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE bots IS '机器人基础设置表';
COMMENT ON COLUMN bots.id IS '机器人ID';
COMMENT ON COLUMN bots.name IS '机器人名称';
COMMENT ON COLUMN bots.avatar IS '机器人头像';
COMMENT ON COLUMN bots.desc IS '机器人描述';
COMMENT ON COLUMN bots.user_id IS '创建人用户ID 关联 user.id';
COMMENT ON COLUMN bots.created_at IS '创建时间';
COMMENT ON COLUMN bots.updated_at IS '更新时间';

-- 加入索引 idx_user_id
CREATE INDEX bots_idx_user_id ON bots (user_id);
