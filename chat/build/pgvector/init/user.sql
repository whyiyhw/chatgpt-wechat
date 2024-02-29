CREATE TABLE "user"
(
    "id"         bigserial PRIMARY KEY,
    "name"       varchar(50)  NOT NULL,
    "email"      varchar(121) NOT NULL,
    "password"   varchar(255) NOT NULL,
    "avatar"     varchar(255) NOT NULL,
    "is_admin"   boolean   DEFAULT false,
    "created_at" timestamp DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamp DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE "user" IS '用户信息表';
COMMENT ON COLUMN "user"."id" IS '用户全局唯一主键';
COMMENT ON COLUMN "user"."name" IS '用户名称';
COMMENT ON COLUMN "user"."email" IS '用户邮箱';
COMMENT ON COLUMN "user"."password" IS '用户密码';
COMMENT ON COLUMN "user"."avatar" IS '用户头像';
COMMENT ON COLUMN "user"."is_admin" IS '是否为管理员';
COMMENT ON COLUMN "user"."created_at" IS '创建时间';
COMMENT ON COLUMN "user"."updated_at" IS '更新时间';

-- 加入唯一索引 email
CREATE UNIQUE INDEX user_email_idx ON "user" ("email");
