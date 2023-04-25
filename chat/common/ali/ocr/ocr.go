package ocr

import (
	"encoding/json"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	ocrapi20210707 "github.com/alibabacloud-go/ocr-api-20210707/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
)

func CreateClient(accessKeyId *string, accessKeySecret *string) (_result *ocrapi20210707.Client, _err error) {
	config := &openapi.Config{
		// 必填，您的 AccessKey ID
		AccessKeyId: accessKeyId,
		// 必填，您的 AccessKey Secret
		AccessKeySecret: accessKeySecret,
	}
	// 访问的域名
	config.Endpoint = tea.String("ocr-api.cn-hangzhou.aliyuncs.com")
	_result = &ocrapi20210707.Client{}
	_result, _err = ocrapi20210707.NewClient(config)
	return _result, _err
}

func Image2Txt(arg string, client *ocrapi20210707.Client) (res string, _err error) {
	recognizeGeneralRequest := &ocrapi20210707.RecognizeGeneralRequest{
		Url: tea.String(arg),
	}
	runtime := &util.RuntimeOptions{}
	txt, tryErr := func() (res string, _e error) {
		defer func() {
			if r := tea.Recover(recover()); r != nil {
				_e = r
			}
		}()
		// 复制代码运行请自行打印 API 的返回值
		result, _err := client.RecognizeGeneralWithOptions(recognizeGeneralRequest, runtime)
		if _err != nil {
			return "", _err
		}

		res = *result.Body.Data
		type Content struct {
			Content string `json:"content"`
		}
		c := &Content{}
		err := json.Unmarshal([]byte(res), c)
		if err != nil {
			return "", err
		}

		return c.Content, nil
	}()

	if tryErr != nil {
		var sdkError = &tea.SDKError{}
		if _t, ok := tryErr.(*tea.SDKError); ok {
			sdkError = _t
		} else {
			sdkError.Message = tea.String(tryErr.Error())
		}
		// 如有需要，请打印 error
		r, _err := util.AssertAsString(sdkError.Message)
		if _err != nil {
			return *r, _err
		}
	}
	return txt, _err
}
