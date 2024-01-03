SET NAMES 'utf8';
-- 机器人开发配置表 prompts & model config & skills
CREATE TABLE `bots_prompt`
(
    `id`         bigint unsigned NOT NULL AUTO_INCREMENT,
    `bot_id`     bigint unsigned NOT NULL DEFAULT 0 COMMENT '机器人ID 关联 bots.id',
    `prompt`     text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '机器人初始设置',
    `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`)
) ENGINE=InnoDB COMMENT="机器人prompt设置表" AUTO_INCREMENT=1 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
Alter TABLE `bots_prompt` ADD INDEX `idx_bot_id` (`bot_id`) USING BTREE;