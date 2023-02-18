package wecom

import "github.com/xen0n/go-workwx"

func SendToUser(agentID int64, userID string, msg string, corpID string, corpSecret string) {

	go func() {
		// 然后把数据 发给微信用户
		app := workwx.New(corpID).WithApp(corpSecret, agentID)

		recipient := workwx.Recipient{
			UserIDs: []string{userID},
		}

		_ = app.SendTextMessage(&recipient, msg, false)
	}()
}
