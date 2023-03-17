SET NAMES 'utf8';
CREATE TABLE `user`
(
    `id`         bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '用户全局唯一主键',
    `name`       varchar(50) COLLATE utf8mb4_unicode_ci                        NOT NULL DEFAULT '' COMMENT '用户名称',
    `email`      varchar(191) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '用户邮箱',
    `password`   varchar(255) COLLATE utf8mb4_unicode_ci                       NOT NULL DEFAULT '' COMMENT '用户密码',
    `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=1 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;