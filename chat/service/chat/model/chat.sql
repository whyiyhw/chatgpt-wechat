SET NAMES 'utf8';
CREATE TABLE `chat`
(
    `id`          bigint unsigned NOT NULL AUTO_INCREMENT,
    `user`        varchar(191) COLLATE utf8mb4_unicode_ci                       NOT NULL COMMENT 'weCom用户标识/customer用户标识',
    `message_id`  varchar(191) COLLATE utf8mb4_unicode_ci                       NOT NULL COMMENT 'message_id customer消息唯一ID',
    `open_kf_id`  varchar(191) COLLATE utf8mb4_unicode_ci                       NOT NULL COMMENT '客服标识',
    `agent_id`    bigint unsigned                                               NOT NULL DEFAULT 0 COMMENT '应用ID',
    `req_content` varchar(500) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '用户发送内容',
    `res_content` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL  COMMENT 'openai响应内容',
    `created_at`  timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`  timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    KEY           `user_agent_idx` (`user`,`agent_id`) USING BTREE,
    KEY           `user_kf_idx` (`user`,`open_kf_id`) USING BTREE,
    KEY           `user_message_idx` (`user`,`message_id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=1 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;