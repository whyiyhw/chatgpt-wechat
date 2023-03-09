package xerr

var message map[uint32]string

func init() {
	message = make(map[uint32]string)
	message[SUCCESS] = "成功"
	message[ServerFail] = "调用失败请稍后重试"
	message[RequestParamError] = "参数错误"
	message[UNAUTHORIZED] = "无效的token"
	message[FORBIDDEN] = "权限不足"
	message[PasswordIncorrect] = "密码错误"
	message[RouteNotFound] = "请求资源不存在"
	message[RouteNotMatch] = "请求方式错误"
	message[DBError] = "数据库繁忙,请稍后再试"
}

func MapErrMsg(errCode uint32) string {
	if msg, ok := message[errCode]; ok {
		return msg
	} else {
		return "服务器开小差啦,稍后再来试一试"
	}
}

func IsCodeErr(errCode uint32) bool {
	if _, ok := message[errCode]; ok {
		return true
	} else {
		return false
	}
}
