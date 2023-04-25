## 使用前提条件
- 需要去注册一个个人[企业微信](https://work.weixin.qq.com/)
- 云服务器 1h2g
    - 如果是自己注册的企业微信，那么其实是不需要域名的，直接ip访问
    - 如果是企业微信已经关联了备案主体，那么需要开一个备案的二级域名解析到服务器，nginx 做下转发就行
- 其它情况，如我域名没备案，但是我就是想用这个域名解析到我的服务器，
  - 那就就可以考虑下面这种 [云函数/网关转发思路，点击查看](./cloudfc.md) 
- 相关网络与服务示意图
  ![system02.png](./system02.png)

## 如何使用本项目代码？

### 1. 创建个人企业微信 并获取到对应的 **企业id(corp_id)**

访问 [管理员页面](https://work.weixin.qq.com/wework_admin/frame#profile) ,
可在 我的企业 > 企业信息 > 底部 看到  企业ID
![image3.png](./image38.png)

#### 1.1 创建一个企业微信内部应用，并获取到 AgentId 和 Secret

可在 我的企业 > 应用管理 > 自建  看到创建应用，创建一个名为 **ChatGPT** 的应用，并上传应用头像。创建完成后可以在应用详情页看到到 AgentId 和 Secret
![image3.png](./image3.png)

#### 1.2 配置企业可信IP
- 可以在详情页看到 企业可信IP的配置，把你服务器的公网IP 填入就好，如果没有这个配置项，就说明是老应用，无需处理，这步跳过
  ![image21.png](./image21.png)

### 2. 获取 OpenAI 的 KEY

访问 [Account API Keys - OpenAI API](https://platform.openai.com/account/api-keys) ，点击 `Create new secret key` ，创建一个新的 key ，并保存备用。
![image10.png](./image10.png)

### 3. 点击启用消息

会进入验证步骤, 先不验证 url 我们可以 拿到  Token 跟 EncodingAESKey
![image2.png](./image2.png)


### 4. 在自购服务器上 部署 golang 服务，并开启对外的网络端口

**（本项目不提供宝塔面板安装咨询，请各位大佬自行摸索）**

- **前提条件，需要有一个自己的服务器，或者云服务器**
- 执行 docker -v 是否有版本号？
- 执行 docker-compose -v 是否有版本号？

  ![image29](./image29.png)

- 确认这两个软件都安装后

```shell
# 进入chat 后端目录
cd ./chat

# 从备份生成 配置文件
cp ./service/chat/api/etc/chat-api.yaml.bak ./service/chat/api/etc/chat-api.yaml
vim ./service/chat/api/etc/chat-api.yaml
```

- 修改这5个配置项
  ![image25.png](./image25.png)

- 前两个是企业微信 的配置
    - 访问 企业微信-管理员页面 , 可在 我的企业 > 企业信息 > 底部 看到 CorpID
    - DefaultAgentSecret 就是 步骤一中的 Secret
    - Token 跟 EncodingAESKey 可以在步骤三中拿到

- 最后一个 是 openAPI 生成 KEY 的值

---

#### 4.1 重点，因为 openai 对于大陆地区的封锁，如果你的服务器在国内，这边提供了两个方案
1. 自建 代理服务器，然后在 chat-api.yaml 中配置代理服务器的地址，相关的参数在 `chat-api.yaml.complete.bak`
```yaml
Proxy:                                         # 代理配置 （可选）
  Enable: false                                # 是否启用代理，默认为 false（可选）
  Socket5: "{host}:{port}"                     # 代理地址 默认为 127.0.0.1:1080（可选）
  Http: "http://{host}:{port}"                 # 代理地址 默认为空（可选）

# host 是代指你实际代理应用的IP
# 因为本项目使用 docker-compose 搭建，所以一般应该填入代理应用所在宿主机的内网IP
```
如何自建代理，点击查看 [自建代理](./proxy.md)

2. 使用 cf 自建反向域名代理，然后用的代理域名替换掉,OpenAi 的 Host 即可
```yaml
OpenAi:                                             # openai配置
  Key: "xxxxxxxxxxxxxxxxxxxxx"                      # openai key
  Host: "https://api.openai.com"                    # openai host （可选，使用cf进行反向代理时，修改可用）
```
如何自建反向域名代理，点击查看 [自建反向域名代理](./cf.md)

```shell
# 修改好后生成集成应用镜像
sudo docker-compose build

# 启动集成应用
sudo docker-compose up -d
```
- 最后在 企业微信的配置中，把 服务器地址 `http://{host}:8887` 填入，如下图
  ![image26.png](./image26.png)

- 🎉🎉 你的机器人就配置好了

### 5. 正式布发布与微信打通

可在 我的企业 > 微信插件 > 下方找到 一个邀请关注二维码，
![image13.png](./image13.png)

微信扫码后，就可以在 微信中看到对应的公司名称，点进企业号应用，我们的机器人，赫然在列。

上述这些都配置完成后，你的机器人就配置好了


如果此项目对您有帮助，也可以扫码我的公众号，感谢关注！
- 
- 如果需要企业自定义方案，也可以wx我 `whyiyhwxy`

![image99.png](./image99.png)
