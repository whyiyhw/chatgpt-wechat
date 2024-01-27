CREATE TABLE knowledge
(
    id         bigserial PRIMARY KEY,
    user_id    bigint    default 0 not null,
    name       varchar(191) NOT NULL,
    avatar     varchar(255) NOT NULL,
    "desc"     varchar(255) NOT NULL,
    created_at timestamp DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE knowledge IS '知识库';
comment on column knowledge.id is '主键';
comment on column knowledge.user_id is '用户ID';
comment on column knowledge.name is '知识库名称';
comment on column knowledge.avatar is '知识库头像';
comment on column knowledge.desc is '知识库描述';
-- 加入索引 user_id_idx
CREATE INDEX user_id_idx ON knowledge ("user_id");

CREATE TABLE knowledge_unit
(
    id           bigserial PRIMARY KEY,
    knowledge_id bigint       NOT NULL,
    name         varchar(191) NOT NULL,
    type         varchar(191) NOT NULL,
    source       varchar(191) NOT NULL,
    hit_counts   int          NOT NULL DEFAULT 0,
    enable       boolean      NOT NULL DEFAULT true,
    created_at   timestamp             DEFAULT CURRENT_TIMESTAMP,
    updated_at   timestamp             DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE knowledge_unit IS '知识单元';
comment on column knowledge_unit.id is '主键';
comment on column knowledge_unit.knowledge_id is '知识库ID';
comment on column knowledge_unit.name is '知识单元名称';
comment on column knowledge_unit.type is '知识单元类型(txt)';
comment on column knowledge_unit.source is '知识单元来源';
comment on column knowledge_unit.hit_counts is '命中次数';
comment on column knowledge_unit.enable is '是否启用';
-- 加入索引 knowledge_id_idx
CREATE INDEX knowledge_id_idx ON knowledge_unit ("knowledge_id");


-- 开启向量索引
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE knowledge_unit_segments
(
    id                bigserial PRIMARY KEY,
    knowledge_id      bigint        NOT NULL,
    knowledge_unit_id bigint        NOT NULL,
    value             varchar(2000) NOT NULL,
    embedding         vector(768)   NOT NULL,
    created_at        timestamp DEFAULT CURRENT_TIMESTAMP,
    updated_at        timestamp DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE knowledge_unit_segments IS '知识单元分段';
comment on column knowledge_unit_segments.id is '主键';
comment on column knowledge_unit_segments.knowledge_id is '知识库ID';
comment on column knowledge_unit_segments.knowledge_unit_id is '知识单元ID';
comment on column knowledge_unit_segments.value is '知识单元分段内容';
comment on column knowledge_unit_segments.embedding is '知识单元分段内容向量';

-- 加入索引 knowledge_id_idx
CREATE INDEX knowledge_id_unit_id_idx ON knowledge_unit_segments ("knowledge_id", "knowledge_unit_id");
CREATE INDEX ON knowledge_unit_segments USING hnsw(embedding vector_l2_ops);