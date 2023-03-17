package wecom

import "github.com/xen0n/go-workwx"

func SendToUser(agentID int64, userID string, msg string, corpID string, corpSecret string) {

	go func() {
		// 然后把数据 发给微信用户
		app := workwx.New(corpID).WithApp(corpSecret, agentID)

		recipient := workwx.Recipient{
			UserIDs: []string{userID},
		}
		rs := []rune(msg)

		//当 msg 大于 1000个字符 的时候切割发送
		//3. 企业微信会话发送文字的字符上限
		//（1）会话消息目前支持2000（不确定）？字符，1个汉字=2字符，1个英文、符号=1个字符。
		if len(rs) > 850 {
			msgs := SplitMsg(rs, 850)
			for _, v := range msgs {
				_ = app.SendTextMessage(&recipient, v, false)
			}
			return
		}

		_ = app.SendTextMessage(&recipient, msg, false)
	}()
}

func SplitMsg(rs []rune, i int) []string {
	var msgs []string
	for len(rs) > i {
		msgs = append(msgs, string(rs[:i]))
		rs = rs[i:]
	}
	msgs = append(msgs, string(rs))
	return msgs
}
