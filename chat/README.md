# go-zero-demo

## 项目目录

- 遵循 go-zero 社区原则下 ， 项目目录结构如下

```
├── service
│   ├── demo
│   │   ├── api
│   │   │   ├── doc
│   │   │   │── etc   
│   │   │   ├── internal
│   │   │   ├── demo.api
│   │   │   ├── demo.go
│   │   ├── model
```

- 在生成目录下加入 doc 目录，用于存放 api 文件，如 user.api,这些文件会通过 demo.api 进行 import
- model 目录下存放数据库模型，通过 sql 文件自动生成，不需要手动创建
  - 但是目前存在一个问题，生成的方法，可能没有我们所需要的，需要自行新增

## 项目基本功能说明

### 验证器 （本地化支持）
- 目前 `go-zero` 支持的 验证器很有限，我们选择接入第三方的验证器，目前选择的是 `go-playground/validator`
- 使用 `goctl` 的模板功能，替换生成的代码，自动生成验证器的代码

```shell
# 先进行模板初始化
goctl  template init 
# Templates are generated in C:\Users\Administrator\.goctl\1.4.4
# 对于mac 应该实在 ~/.goctl/1.4.4/ 目录下
# 然后对模板进行替换，比如现在我们的验证器，每次都需要手动去写，我们可以把这个写入到模板
# 替换的模板我放在了 项目一级目录 template 下面
# 新增 model 层替换模板
```

### 统一全局响应码 （已实现）
- 在 common 中 加入 response 文件夹，用于存放统一的响应码以及相关的方法
- 加入 xerr 目录，所有的错误响应实现 了 error 接口，这样可以统一处理错误响应

### 日志
- 默认为 console 日志，后续都是容器部署，可以直接采集容器日志，所以不需要文件日志

### 中间件
#### access_log  记录访问&响应日志 （已实现）
#### 默认的 `jwt: Auth` 认证 （已实现）

### 监控（waiting） （待实现）

## 其它可能碰到的问题

- sql 生成 不可以使用字符串拼接，有sql 注入风险，参数统一使用 ? 占位符

## 引入包说明

### model 操作
- sql 引入一个简单的 sql 语句封装库 `github.com/Masterminds/squirrel`

### 验证器

- `github.com/go-playground/validator/v10`  验证器

### 本地编译
`CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app ./service/chat/api/chat.go`

### docker 编译&部署
- build 镜像
```shell
docker build -f ./Dockerfile -t chat .
```
- run container

```shell
docker run -i -t -d \
 --restart=always \
 -p 8990:8888 \
  --name=chat-service  chat
```
