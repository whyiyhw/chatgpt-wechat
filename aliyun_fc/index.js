var getRawBody = require('raw-body');
var getFormBody = require('body/form');
var body = require('body');
var axios = require('axios');
const crypto = require('@wecom/crypto');
var readXml = require('xmlreader');

/*
To enable the initializer feature (https://help.aliyun.com/document_detail/156876.html)
please implement the initializer function as below：
exports.initializer = (context, callback) => {
  console.log('initializing');
  callback(null, '');
};
*/

// 服务器验证 aeskey
const aes_key = process.env.aes_key;
const aes_token = process.env.aes_token;
const req_host = process.env.req_host;
const req_token = process.env.req_token;

// 获取的 回复消息
async function replyMsgToUser(userID, text, agentID, channel = "wecom") {

    const data = JSON.stringify({
        "agent_id": parseInt(agentID),
        "channel": channel,
        "user_id": userID,
        "msg": text,
    });

    const config = {
        method: 'post',
        maxBodyLength: Infinity,
        url: req_host,
        headers: {
            'Content-Type': 'application/json',
            'Authorization': "Bearer " + req_token
        },
        data: data
    };

    console.log(data);

    await axios(config)
        .then(function (response) {
            console.log("reply ok:", JSON.stringify(response.data));
        })
        .catch(function (error) {
            console.log("reply error:", error);
        });
}

exports.handler = async (req, resp, context) => {

    const params = {
        path: req.path,
        queries: req.queries,
        headers: req.headers,
        method: req.method,
        requestURI: req.url,
        body: req.body,
        clientIP: req.clientIP,
    };

    console.log(params);

    // 验证服务是否存在
    if (req.queries.hasOwnProperty("msg_signature") && params.method === "GET") {
        // 从 query 中获取相关参数
        const {msg_signature, timestamp, nonce, echostr} = req.queries;
        const signature = crypto.getSignature(aes_token, timestamp, nonce, echostr);
        console.log('signature', signature);
        if (msg_signature === signature) {
            console.log('signture ok');
            const {message} = crypto.decrypt(aes_key, echostr);

            resp.setHeader("Content-Type", "text/plain");
            resp.send(message);
            return;
        }
    }

    // 用户消息回调事件
    if (req.queries.hasOwnProperty("msg_signature") && params.method === "POST") {
        // 从 query 中获取相关参数
        const {msg_signature, timestamp, nonce} = req.queries;

        let echostr = "";
        readXml.read(params.body.toString(), (errors, xmlResponse) => {
            if (null !== errors) {
                console.log(errors)
                return;
            }
            console.log(xmlResponse);
            echostr = xmlResponse.xml.Encrypt.text();
        })

        const signature = crypto.getSignature(aes_token, timestamp, nonce, echostr);
        console.log('signature', signature);
        if (msg_signature === signature) {
            console.log('content signture ok');
            const {message} = crypto.decrypt(aes_key, echostr);

            console.log(message);
            let userSendContent = "";
            let userID = "";
            let agentID = "";
            let error = false;
            readXml.read(message, (errors, xmlResponse) => {
                if (null !== errors) {
                    console.log(errors)
                    return;
                }
                console.log(xmlResponse);

                let msgType = xmlResponse.xml.MsgType.text();
                userID = xmlResponse.xml.FromUserName.text();
                agentID = xmlResponse.xml.AgentID.text();

                if (msgType === "event" && xmlResponse.xml.Event.text() === "click") {
                    userSendContent = "#clear"
                    return;
                }
                //支持进入事件
                if (msgType === "event" && xmlResponse.xml.Event.text() === "enter_agent") {
                    userSendContent = "#welcome"
                    return;
                }
                //支持图片消息
                if (msgType === "image" && xmlResponse.xml.PicUrl.text() !== "") {
                    userSendContent = "#image:" + xmlResponse.xml.PicUrl.text();
                    return;
                }

                if (msgType !== "text") {
                    error = true;
                    return;
                }

                userSendContent = xmlResponse.xml.Content.text();
            });

            // 非文本消息进行错误提示
            if (error) {
                await replyMsgToUser(userID, "暂不支持，此类型的数据", agentID);
            } else {
                await replyMsgToUser(userID, userSendContent, agentID, "openai");
            }

            resp.setHeader("Content-Type", "text/plain");
            resp.send("");
            return;
        }
    }

    getRawBody(req, function (err, body) {
        for (const key in req.queries) {
            const value = req.queries[key];
            resp.setHeader(key, value);
        }
        resp.setHeader("Content-Type", "text/plain");
        params.body = body.toString();
        resp.send(JSON.stringify(params, null, '    '));
    });
}