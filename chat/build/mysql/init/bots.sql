SET NAMES 'utf8';
CREATE TABLE `bots`
(
    `id`         bigint unsigned NOT NULL AUTO_INCREMENT,
    `name`       varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '机器人名称',
    `avatar`     varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '机器人头像',
    `desc`       varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '机器人描述',
    `user_id`    bigint unsigned NOT NULL DEFAULT 0 COMMENT '创建人用户ID 关联 user.id',
    PRIMARY KEY (`id`),
    `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间'
) ENGINE=InnoDB AUTO_INCREMENT=1 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='机器人基础设置表';
-- 加入索引 id_user_id
ALTER TABLE `bots` ADD INDEX `idx_user_id` (`user_id`) USING BTREE;
