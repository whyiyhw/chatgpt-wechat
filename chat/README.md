## 安装 gen
```shell
go install gorm.io/gen/tools/gentool@latest
```

## 运行生成 model
```shell
gentool -db postgres -dsn "host=127.0.0.1 user=chat password=Chat-gpt~wechat dbname=chat port=43306 sslmode=disable TimeZone=Asia/Shanghai" -outPath "./service/chat/dao"
```

## 工具版本
- `goctl 1.6.1`
- `gentool@latest`