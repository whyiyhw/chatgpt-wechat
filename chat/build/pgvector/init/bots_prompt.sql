CREATE TABLE bots_prompt
(
    id         bigserial PRIMARY KEY,
    bot_id     bigint NOT NULL DEFAULT 0,
    prompt     text   NOT NULL,
    created_at timestamp       DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp       DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE bots_prompt IS '机器人prompt设置表';

COMMENT ON COLUMN bots_prompt.id IS '机器人初始设置ID';
COMMENT ON COLUMN bots_prompt.bot_id IS '机器人ID 关联 bots.id';
COMMENT ON COLUMN bots_prompt.prompt IS '机器人初始设置';
COMMENT ON COLUMN bots_prompt.created_at IS '创建时间';
COMMENT ON COLUMN bots_prompt.updated_at IS '更新时间';

-- 加入索引 idx_bot_id
CREATE INDEX bots_prompt_idx_bot_id ON bots_prompt (bot_id);

-- auto-generated definition
create table bots_with_custom
(
    id         bigserial
        primary key,
    bot_id     bigint    default 0 not null,
    open_kf_id varchar             not null,
    created_at timestamp default CURRENT_TIMESTAMP,
    updated_at timestamp default CURRENT_TIMESTAMP
);

comment on table bots_with_custom is '机器人关联客服设置表';
comment on column bots_with_custom.id is '机器人初始设置ID';
comment on column bots_with_custom.bot_id is '机器人ID 关联 bots.id';
comment on column bots_with_custom.open_kf_id is '客服id 关联企业微信 客服ID';
comment on column bots_with_custom.created_at is '创建时间';
comment on column bots_with_custom.updated_at is '更新时间';

create index bots_with_custom_idx_kf_id_bot_id on bots_with_custom (open_kf_id, bot_id);

-- auto-generated definition bots_with_model
create table bots_with_model
(
    id          bigserial
        primary key,
    bot_id      bigint         default 0    not null,
    model_type  varchar                     not null,
    model_name  varchar                     not null,
    temperature numeric(10, 2) default 0.00 not null,
    created_at  timestamp      default CURRENT_TIMESTAMP,
    updated_at  timestamp      default CURRENT_TIMESTAMP
);

comment on table bots_with_model is '机器人关联模型设置表';
comment on column bots_with_model.id is '机器人初始设置ID';
comment on column bots_with_model.bot_id is '机器人ID 关联 bots.id';
comment on column bots_with_model.model_type is '模型类型 openai/gemini';
comment on column bots_with_model.model_name is '模型名称';
comment on column bots_with_model.temperature is '温度';
comment on column bots_with_model.created_at is '创建时间';
comment on column bots_with_model.updated_at is '更新时间';

create table bots_with_knowledge
(
    id           bigserial
        primary key,
    bot_id       bigint    default 0 not null,
    knowledge_id bigint    default 0 not null,
    created_at   timestamp default CURRENT_TIMESTAMP,
    updated_at   timestamp default CURRENT_TIMESTAMP
);

comment on table bots_with_knowledge is '机器人关联知识库设置表';
comment on column bots_with_knowledge.id is '机器人初始设置ID';
comment on column bots_with_knowledge.bot_id is '机器人ID 关联 bots.id';
comment on column bots_with_knowledge.knowledge_id is '知识库ID 关联 knowledge.id';
comment on column bots_with_knowledge.created_at is '创建时间';
comment on column bots_with_knowledge.updated_at is '更新时间';
create index bots_with_knowledge_idx_knowledge_id_bot_id on bots_with_knowledge (knowledge_id, bot_id);
