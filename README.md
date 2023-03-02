# chatgpt-wechat

微信用户可自建的 ChatGPT 机器人

## 食用前提条件
- 域名（备案/不备案）都可以，
- 云服务器 1h2g 就够了
- 需要去注册一个个人[企业微信](https://work.weixin.qq.com/)
- 阿里云开通云函数服务（新人一年88的额度，绝对够用）
  - 其它腾讯云，华为云 云函数都适用，需要自行修改 js 文件。

(如果你有 已经备案好的域名，下面的服务与函数区域可以不用管)

- 为什么会需要云函数做中转？
  - 因为我不想去备案，已经备案的，可以直接域名解析到服务器，无视云函数这一步，后续我会在后端直接支持企业微信的回调认证

- 为什么需要一台服务器，完全用云函数是否可行？
  - 目前来看，完全用同步的web函数不可能，3秒的执行时间会导致你等不到 openai 响应就整个函数会被kill掉。除非你在web函数中再触发异步函数去执行请求与获取响应，整个下来，费用跟调试成本都比较高，我假设大家都有自己的服务器，是真的不如把自己的服务器利用起来，后续有时间也会尝试用异步函数试试~

## 效果如图

![image1.png](./doc/image1.png)

## 如何使用本项目代码？

### 1. 创建个人企业微信 并获取到对应的 企业id(corp_id)

访问 [管理员页面](https://work.weixin.qq.com/wework_admin/frame#profile) ,
可在 我的企业 > 企业信息 > 底部 看到  企业ID

#### 1.1 创建一个企业微信内部应用，并获取到 AgentId 和 Secret

可在 我的企业 > 应用管理 > 自建  看到创建应用，创建一个名为 **ChatGPT** 的应用，并上传应用头像。创建完成后可以在应用详情页看到到 AgentId 和 Secret
![image3.png](./doc/image3.png)

#### 1.2 配置企业可信IP
- 可以在详情页看到 企业可信IP的配置，把你服务器的公网IP 填入就好，如果没有这个配置项，就说明是老应用，无需处理，这步跳过
![image21.png](./doc/image21.png)

### 2. 点击启用消息

会进入验证步骤, 先不验证 url 我们可以 拿到  Token 跟 EncodingAESKey
![image2.png](./doc/image2.png)

### 3. 访问 [阿里云函数计算 fc ](https://fcnext.console.aliyun.com/cn-hangzhou/services) ，

创建一个新的服务与函数  **重点** 需要选择中国大陆以外的区域，如香港/日本
![image2.png](./doc/image16.png)

登录 [阿里云函数计算 fc](https://fcnext.console.aliyun.com/cn-hangzhou/services) ，创建一个新的 Node.js v16/v14 的服务，服务名可以根据你的需要填写，可以填写 ChatGPT .

![image4.png](./doc/image4.png)

再创建一个函数，函数名也可以随意

![image5.png](./doc/image5.png)

### 4. 复制本项目下的 aliyunfc/index.js 的源码内容，并粘贴到 webide 当中

然后点击顶部的 deploy ，完成第一次部署。

![image6.png](./doc/image6.png)

### 5. 安装所需依赖

这个开发过程中，我们使用了企业微信开放平台官方提供的 SDK，以及 axios 来完成调用。在webide中开启终端，安装 `axios` 和 `@wecom/crypto` 还有 `xmlreader`。

```shell
npm i axios
npm i @wecom/crypto
npm i xmlreader
```

![image7.png](./doc/image7.png)

安装完成后，点击上方的部署，使其生效。

### 5.5 将自己的域名 cname 解析到 云函数

- 进入 fc -> 高级功能 -> 域名管理 
![image17.png](./doc/image17.png)

- 我们可以找到这个cname 的值

- 接下来复制这个值去购买域名的服务商，下配置 cname 

![image18.png](./doc/image18.png)

- 配置好后，等待1分钟，点击保存，我们就成功的把未备案域名 解析到了香港的云函数上

### 6. 配置环境变量

接下来我们回到函数管理来配置环境变量，你需要配置两个个环境变量 `aes_key` 和 `aes_token` `aes_key` 填写你第二步获取到的 EncodingAESKey，`aes_token` 填写你第二步获取到的 Token。
![image8.png](./doc/image8.png)

配置完成，点确认后，再次点击上方的 **Deploy** 按钮部署，使这些环境变量生效。这个时候去 企业微信里面，
![image12.png](./doc/image12.png)
填入函数的 url , 点击保存, 验证就通过了.
![image19.png](./doc/image19.png)

**url 就是你刚刚 cname 解析过去的自己的域名**

### 7. 获取 OpenAI 的 KEY ，并配置环境变量

访问 [Account API Keys - OpenAI API](https://platform.openai.com/account/api-keys) ，点击 `Create new secret key` ，创建一个新的 key ，并保存备用。
![image10.png](./doc/image10.png)

### 8. 在自购服务器上 部署 golang 服务，并开启对外的网络端口
- 前提条件，需要有一个自己的服务器，或者云服务器
- 执行 docker -v 是否有版本号？
- 执行 docker-compose -v 是否有版本号？
- 确认这两个软件都安装后
- 
```shell
# 进入chat 后端目录
cd ./chat

# 从备份生成 配置文件
cp ./service/chat/api/etc/chat-api.yaml.bak ./service/chat/api/etc/chat-api.yaml
vim ./service/chat/api/etc/chat-api.yaml
```
- 修改这三个配置项
![image20.png](./doc/image20.png)

- 前两个是企业微信 的配置
  - corpSecret 就是 步骤一中的 Secret
  - 访问 企业微信-管理员页面 , 可在 我的企业 > 企业信息 > 底部 看到 CorpID

- 最后一个 是 openAPI 生成 KEY 的值

```shell
# 修改好后生成集成应用镜像
sudo docker build -f ./Dockerfile -t chat .

# 启动集成应用
sudo docker-compose up -d
```
- 应用启动成功后 我们需要去拿 req_host 和 req_token
  - `req_host` 就是部署服务器的 `http://{host}:8888/api/msg/push` `{host}` 就是你服务器的ip
  - `req_token` 就是自己注册一个账号,调用登录api获取到的 token ，集体步骤如下
- 调用注册api
```shell
curl --location 'localhost:8888/api/user/register' \
--header 'Content-Type: application/json' \
--data '{"email": "admin@163.com","name": "admin","password": "admin123"}'
```
- 调用登录api
```shell
curl --location 'localhost:8888/api/user/login' \
--header 'Content-Type: application/json' \
--data '{"email": "admin@163.com","password": "admin123"}'
```
- 登录API 响应
```json
{
  "code":200,
  "msg":"成功",
  "data":{
    "token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MDM1Njk0MzgsImlhdCI6MTY3NzY0OTQzOCwidXNlcklkIjoxfQ.mjRJcu3WNaqAYHB1RbG3qoBezzbEsW6weq8amOvGAaU"
  }
}
```
- 所以 `req_token` 就是 `data.token` 的值

- 最后把 `req_host` 和 `req_token` 配置到阿里云函数云的环境变量中
- 🎉🎉 你的机器人就配置好了

### 9. 正式布发布与微信打通

可在 我的企业 > 微信插件 > 下方找到 一个邀请关注二维码，
![image13.png](./doc/image13.png)

微信扫码后，就可以在 微信中看到对应的公司名称，点进企业号应用，我们的机器人，赫然在列。

上述这些都配置完成后，你的机器人就配置好了

如果对您有帮助，也可以扫码我的公众号，感谢关注！

![image99.png](./doc/image99.png)

## FAQ

## 版本更新记录

### v0.1

- add 支持记忆多轮对话与记忆清理
- fix 对非文本格式数据进行回复拒绝

为了支持多轮对话，新增菜单配置  企业微信>应用管理>自定义菜单
![image14.png](./doc/image14.png)
效果如下
![image15.png](./doc/image15.png)

### v0.2

- 后端代码已发布
  - 需要 docker 以及简单的运维操作，实现已经卸载
  - 如需使用，请先配置相关数据库与 redis , 各类 密钥 通过 `chat\service\chat\api\etc\chat-api.yaml` 进行配置
  - over😀
- 增加 阿里云新增 req_token 环境变量来进行验证 请求合法性

### v0.2.1
- 简化后端运维操作，增加 docker-compose 来编排服务

### feature 版本 考虑与执行中
- [ ] 单服务-多应用支持
- [ ] 支持 prompt 自定义配置，以及预定义模板
- [ ] 支持最新的 gpt3.5 与模型可自行切换
- [ ] 支持 openapi 对话 token 累计功能， 余额不足时，支持 token 更换 
- [ ] 支持作图功能（可选）
- [ ] 支持英语语音输入（可选）
- [ ] 支持特定角色对话-如雅思口语练习（可选）
- [ ] 支持web管理页面，配置入库方便修改（可选）
- [ ] 十分期待您的需求，可以提issue...

## QA 
### 我配置好了，发送给openai 的消息有响应，但是企业微信应用没有收到回复
- 请参考 1.2 配置企业可信IP 