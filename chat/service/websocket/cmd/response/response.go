package response

import "encoding/json"

type Response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data Data   `json:"data"`
}

type Data struct {
}

func Success(Code int) []byte {
	b := Response{
		Code: Code,
		Msg:  "success",
		Data: Data{},
	}

	s, _ := json.Marshal(b)

	return s
}

func Error(Code int, errInfo string) []byte {
	b := Response{
		Code: Code,
		Msg:  errInfo,
		Data: Data{},
	}

	s, _ := json.Marshal(b)
	return s
}
