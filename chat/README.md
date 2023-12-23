## 安装 gen
```shell
go install gorm.io/gen/tools/gentool@latest
```

## 运行生成 model
```shell
gentool -dsn "chat:Chat-gpt~wechat@tcp(127.0.0.1:43306)/chat?charset=utf8mb4&parseTime=true&loc=Local" -outPath "./service/chat/dao"
```