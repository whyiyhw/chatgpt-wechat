
```yaml
Name: chat-api                                      # 项目名称
Host: 0.0.0.0                                       # 项目监听地址
Port: 8888                                          # 项目监听端口

Mysql:                                              # mysql配置
  DataSource: chat:123456@tcp(mysql57:3306)/chat?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai # 数据库连接地址 自建最好修改下密码

RedisCache:                                         # redis缓存配置
  - Host: redis7:6379                               # redis缓存地址
    Pass: "123456"                                  # redis缓存密码 自建最好修改下密码

Auth:                                               # jwt配置（可选）自建最好修改下
  AccessSecret: "xxxxxxxxxxxxxxx"                   # jwt加密密钥(可选) 默认为 xxxxxxxxxxxx
  AccessExpire: 25920000                            # jwt过期时间(可选) 默认为 25920000

WeCom:                                              # 企业微信配置
  Port:                                             # 企业微信回调监听端口（可选）默认为8887
  CustomerServiceSecret: "xxxx-xxxx-xxxx"           # 企业微信客服消息 Secret
  CorpID: "wwxxxxxxxxxxxxxxxxxxxx"                  # 企业微信 CorpID
  DefaultAgentSecret: "55sO-xxxxxxxxxxxxxxxxxx"     # 企业微信应用 Secret
  BasePrompt: "你是 ChatGPT， 一个由 OpenAI 训练的大型语言模型， 你旨在回答并解决人们的任何问题，并且可以使用多种语言与人交流。"                                  # openai 基础设定（可选）
  Model: "text-davinci-003"                                                                                                                     # openai 模型（可选）默认为 gpt-3.5-turbo-0301
  Welcome: "您好！我是 ChatGPT，一个由 OpenAI 训练的大型语言模型，我可以回答您的问题和进行交流。请告诉我您需要了解些什么，我会尽力为您提供答案。发送#help 查看更多功能"  # 进入应用时的欢迎语（可选）
  Token: "xxxxxxxxxx"                               # 企业微信应用/客服消息 Token
  EncodingAESKey: "xxxxxxxxxxxxxxxx"                # 企业微信应用/客服消息 EncodingAESKey
  MultipleApplication:                              # 多应用配置（可选）
    - AgentID: 1000002                                # 企业微信应用ID
      AgentSecret: "55sO-xxxxxxxxxxxxxxxxxxxxxxx"     # 企业微信应用 Secret
      Model: "gpt-3.5-turbo"                          # openai 模型（可选）默认为 gpt-3.5-turbo-0301
      BasePrompt: "你是 ChatGPT， 一个由 OpenAI 训练的大型语言模型，你旨在回答并解决人们的任何问题，并且可以使用多种语言与人交流。" # openai 基础设定（可选）
      Welcome: "您好！我是 ChatGPT，一个由 OpenAI 训练的大型语言模型，我可以回答您的问题和进行交流。请告诉我您需要了解些什么，我会尽力为您提供答案。发送#help 查看更多功能"  # 进入应用时的欢迎语

OpenAi:                                             # openai配置
  Key: "xxxxxxxxxxxxxxxxxxxxx"                      # openai key
  Host: "https://api.openai.com"                    # openai host （可选，使用cf进行反向代理时，修改可用）
  Origin: "open_ai"                                 # 默认为 调用 open_ai 也支持 azure , azure_ad (可选 默认为 open_ai)
  Engine: "deployment_name"                         # engine = "deployment_name"(当 Origin 为 azure, azure_ad  时必填)

Proxy:                                              # 代理配置 （可选）
  Enable: false                                     # 是否启用代理，默认为 false（可选）
  Socket5: "127.0.0.1:1080"                         # 代理地址 默认为 127.0.0.1:1080（可选）
  Http: "http://127.0.0.1:1080"                     # 代理地址 默认为空（可选）

OCR:                                                # OCR配置 ,开启图片识别（可选）
  Company: "ali"                                    # 识别公司，目前支持阿里云（可选）
  AliYun:                                           # 阿里云配置
    AccessKeyId: ""                                 # 阿里云 key
    AccessKeySecret: ""                             # 阿里云 secret

Embeddings:
  Enable: true
  Mlvus:
    Host: "192.168.1.202:19530"
    KeyWords:
      - "xx"

Response:                                           # 回复配置
  Stream: true                                    # 是否开启流式回复,自动断句推荐（可选）

Plugins:
  Enable: true
  List:
    - NameForHuman: "date_shell"
      NameForModel: "date_shell"
      DescForHuman: "这个插件可以提供日期相关的信息"
      DescModel: "This plugin can execute shell commands used to get the date."
      Auth:
        Type: "none"
      API:
        URL: "http://192.168.1.202:8886/api/webhook"

Draw:                                               # 绘画配置
  Enable: false                                     # 是否开启绘画功能（可选）
  StableDiffusion:                                  # 绘画配置
    Host: "http://xx.xxx.xxx.xxx:7860"              # 绘画服务地址
    Auth:                                           # 绘画服务认证
      Username: "xxxxxxxx"                          # 绘画服务用户名
      Password: "xxxxxxxx"                          # 绘画服务密码
```