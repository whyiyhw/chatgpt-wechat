const express = require('express');
const path = require('path');
const fs = require('fs');
const app = express();
const axios = require('axios');
const crypto = require('@wecom/crypto');
const readXml = require('xmlreader');

const aes_key = process.env.aes_key;
const aes_token = process.env.aes_token;
const req_host = process.env.req_host;
const req_token = process.env.req_token;

app.get(`/`, (req, res) => {
      // 验证服务是否存在
      if (req.query.hasOwnProperty("msg_signature") && req.method === "GET") {
        // 从 query 中获取相关参数
        const { msg_signature, timestamp, nonce, echostr } = req.query;
        const signature = crypto.getSignature(aes_token, timestamp, nonce, echostr);
        console.log('signature', signature);
        if (msg_signature === signature) {
            console.log('signture ok');
            const { message } = crypto.decrypt(aes_key, echostr);

            resp.setHeader("Content-Type", "text/plain");
            resp.send(message);
            return;
        }
    }
});

app.get(`/logo`, (req, res) => {
  const logo = path.join(__dirname, 'logo.png');
  const content = fs.readFileSync(logo, {
    encoding: 'base64',
  });
  res.set('Content-Type', 'image/png');
  res.send(Buffer.from(content, 'base64'));
  res.status(200).end();
});

app.get('/user', (req, res) => {
  res.send([
    {
      title: 'serverless framework',
      link: 'https://serverless.com',
    },
  ]);
});

app.get('/user/:id', (req, res) => {
  const id = req.params.id;
  res.send({
    id: id,
    title: 'serverless framework',
    link: 'https://serverless.com',
  });
});

app.get('/404', (req, res) => {
  res.status(404).send('Not found');
});

app.get('/500', (req, res) => {
  res.status(500).send('Server Error');
});

// Error handler
app.use(function (err, req, res, next) {
  console.error(err);
  res.status(500).send('Internal Serverless Error');
});

// Web 类型云函数，只能监听 9000 端口
app.listen(9000, () => {
  console.log(`Server start on http://localhost:9000`);
});
