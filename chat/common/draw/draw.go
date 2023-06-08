package draw

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/logx"
)

const SD = "stable_diffusion"
const OPENAI = "openai"
const TranslatePrompt = "StableDiffusion是一款利用深度学习的文生图模型，支持通过使用提示词来产生新的图像，描述要包含或省略的元素。我在这里引入StableDiffusion算法中的Prompt概念，又被称为提示符。下面的prompt是用来指导AI绘画模型创作图像的。它们包含了图像的各种细节，如人物的外观、背景、颜色和光线效果，以及图像的主题和风格。这些prompt的格式经常包含括号内的加权数字，用于指定某些细节的重要性或强调。例如，'(masterpiece:1.5)'表示作品质量是非常重要的，多个括号也有类似作用。此外，如果使用中括号，如'{blue hair:white hair:0.3}'，这代表将蓝发和白发加以融合，蓝发占比为0.3。以下是用prompt帮助AI模型生成图像的例子：masterpiece,(bestquality),highlydetailed,ultra-detailed,cold,solo,(1girl),detailedeyes,shinegoldeneyes) (longliverhair) expressionless,( long sleeves,puffy sleeves),(white wings),shinehalo,(heavymetal:1.2),(metaljewelry),cross-lacedfootwear(chain),(Whitedoves:1.2 )可以选择的prompt包括：颜色,天气 时间,建筑物,山,海。基于以上规则，请将【%s】【】中的内容，翻译为 prompt"

type Draw interface {
	Txt2Img(prompt string, path chan string) error // 文字转图片
}

// SdDraw Stable diffusion draw
type SdDraw struct {
	Host     string
	Username string
	Password string
}

func NewSdDraw(host, name, password string) *SdDraw {
	return &SdDraw{
		Host:     host,
		Username: name,
		Password: password,
	}
}

func (sd *SdDraw) Txt2Img(prompt string, ch chan string) error {
	url := sd.Host + "/sdapi/v1/txt2img"

	// 对 prompt 比如 进行解析
	reqPayload := ParsePrompt(prompt)

	client := &http.Client{}
	body, _ := json.Marshal(reqPayload)
	logx.Info("draw request body", string(body))
	drawReq, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		logx.Info("draw request client build fail", err)
		return errors.New("构建绘画请求失败，请重新尝试~")
	}
	logx.Info("draw request client build success")
	drawReq.Header.Add("Content-Type", "application/json")
	if sd.Username != "" && sd.Password != "" {
		drawReq.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(sd.Username+":"+sd.Password)))
	}
	ch <- "start"

	res, err := client.Do(drawReq)
	if err != nil {
		logx.Info("draw request fail", err)
		return errors.New("绘画请求失败，请重新尝试~")
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		logx.Info("draw request fail", err)
		return errors.New("绘画请求响应失败，请重新尝试~")
	}

	logx.Info("draw client request success body:", string(resBody))

	var resPayload map[string]interface{}
	err = json.Unmarshal(resBody, &resPayload)

	// 加入异常处理
	if err != nil {
		logx.Info("draw request fail", err)
		if strings.Contains(string(resBody), "504 Gateway Time-out") {
			return errors.New("绘画请求超时，请重新尝试~")
		}
		reqError := new(ReqError)
		err = json.Unmarshal(resBody, reqError)
		if err == nil {
			return errors.New(reqError.Error + ":" + reqError.Detail)
		}
		return errors.New("绘画请求响应解析失败，响应信息为\n" + string(resBody) + "\n请稍后重试~")
	}

	images, ok := resPayload["images"].([]interface{})
	if !ok {
		return errors.New("绘画请求响应解析失败，请重新尝试~")
	}
	for _, image := range images {
		s, ok := image.(string)
		if !ok {
			return errors.New("绘画请求响应解析失败 image 不为字符类型，请重新尝试~")
		}
		// 将解密后的信息写入到本地
		imageBase64 := strings.Split(s, ",")[0]
		decodeBytes, err := base64.StdEncoding.DecodeString(imageBase64)
		if err != nil {
			logx.Info("draw request fail", err)
			return errors.New("绘画请求响应Decode失败，请重新尝试~")
		}

		// 判断目录是否存在
		_, err = os.Stat("/tmp/image")
		if err != nil {
			err := os.MkdirAll("/tmp/image", os.ModePerm)
			if err != nil {
				fmt.Println("mkdir err:", err)
				return errors.New("绘画请求响应保存至目录失败，请重新尝试~")
			}
		}

		path := fmt.Sprintf("/tmp/image/%s.png", uuid.New().String())

		err = os.WriteFile(path, decodeBytes, os.ModePerm)

		if err != nil {
			logx.Info("draw save fail", err)
			return errors.New("绘画请求响应保存失败，请重新尝试~")
		}

		// 再将 image 信息发送到用户
		ch <- path
	}

	ch <- "stop"

	return nil
}

// TXT2IMGReq TXT2IMG Req
type TXT2IMGReq struct {
	Prompt            string  `json:"prompt"`               // 正面提示
	NegativePrompt    string  `json:"negative_prompt"`      // 负面提示
	Steps             int     `json:"steps"`                // 生成图片的步数z
	Width             int     `json:"width"`                // 生成图片的宽
	Height            int     `json:"height"`               // 生成图片的高
	SamplerName       string  `json:"sampler_name"`         // 采样器名称
	BatchSize         int     `json:"batch_size"`           // 批量出图数量,默认为1
	CfgScale          int     `json:"cfg_scale"`            // 提示词相关性,默认为7
	Seed              int64   `json:"seed"`                 // 随机种子,默认为-1
	DenoisingStrength float64 `json:"denoising_strength"`   // 去噪强度,默认为0
	EnableHr          bool    `json:"enable_hr"`            // 是否开启高分辨率,默认为false
	HrScale           int     `json:"hr_scale"`             // 高分辨率倍数,默认为2
	HrUpscaler        string  `json:"hr_upscaler"`          // 高分辨率倍数,默认为2
	HrSecondPassSteps int     `json:"hr_second_pass_steps"` // 高分辨率倍数,默认为0
	HrResizeX         int     `json:"hr_resize_x"`          // 高分辨率倍数,默认为0
	HrResizeY         int     `json:"hr_resize_y"`          // 高分辨率倍数,默认为0
}

func getDefaultDataTXT2IMGReq() TXT2IMGReq {
	t := TXT2IMGReq{
		//Prompt:            "masterpiece, best quality,Amazing,finely detail,Depth of field,extremely detailed CG unity 8k wallpaper,",
		Prompt:            "masterpiece, best quality,Amazing,finely detail,",
		NegativePrompt:    "(worst quality:1.25), (low quality:1.25), (lowres:1.1), (monochrome:1.1), (greyscale), multiple views, comic, sketch, (blurry:1.05),",
		Steps:             20,
		Width:             512,
		Height:            512,
		SamplerName:       "DPM++ SDE Karras",
		BatchSize:         1,
		CfgScale:          7,
		Seed:              -1,
		DenoisingStrength: 0,
		EnableHr:          false,
	}
	return t
}

type ReqError struct {
	Error  string `json:"error"`
	Detail string `json:"detail"`
	Body   string `json:"body"`
	Errors string `json:"errors"`
}

func ParsePrompt(prompt string) TXT2IMGReq {
	reqPayload := getDefaultDataTXT2IMGReq()
	// 替换   为 空格
	prompt = strings.Replace(prompt, " ", " ", -1)
	// 先对"\n"进行分段
	if !strings.Contains(prompt, "\n") {
		reqPayload.Prompt += prompt
		return reqPayload
	}

	//通过 "\n"  分段
	for k, val := range strings.Split(prompt, "\n") {
		// 正面提示
		if k == 0 {
			reqPayload.Prompt = val
			continue
		}
		// 负面提示配置
		if strings.Contains(val, "Negative prompt:") {
			reqPayload.NegativePrompt = strings.TrimSpace(strings.Replace(val, "Negative prompt:", "", -1))
			continue
		}
		// 其它配置
		if strings.Contains(val, "Steps:") {
			for _, v := range strings.Split(val, ", ") {
				if strings.HasPrefix(v, "Steps:") {
					s := strings.TrimSpace(strings.Replace(v, "Steps:", "", -1))
					reqPayload.Steps, _ = strconv.Atoi(s)
				}

				if strings.Contains(v, "Sampler:") {
					reqPayload.SamplerName = strings.TrimSpace(strings.Replace(v, "Sampler:", "", -1))
				}
				if strings.Contains(v, "CFG scale:") {
					s := strings.TrimSpace(strings.Replace(v, "CFG scale:", "", -1))
					// 转 int
					reqPayload.CfgScale, _ = strconv.Atoi(s)
				}
				if strings.Contains(v, "Seed:") {
					s := strings.TrimSpace(strings.Replace(v, "Seed:", "", -1))
					// 转 int64
					seed, err := strconv.ParseInt(s, 10, 64)
					if err == nil {
						fmt.Printf("%T, %v\n", s, s)
						reqPayload.Seed = seed
					}
				}
				if strings.Contains(v, "Size:") {
					s := strings.TrimSpace(strings.Replace(v, "Size:", "", -1))
					// 转 int
					// Size: 512x768, => 512, 768
					size := strings.Split(s, "x")
					if len(size) == 2 {
						reqPayload.Width, _ = strconv.Atoi(size[0])
						reqPayload.Height, _ = strconv.Atoi(size[1])
					}
				}
				//Denoising strength: 0.52,
				if strings.Contains(v, "Denoising strength:") {
					s := strings.TrimSpace(strings.Replace(v, "Denoising strength:", "", -1))
					// 转 float64
					strength, err := strconv.ParseFloat(s, 64)
					if err == nil {
						reqPayload.DenoisingStrength = strength
					}
				}
			}
		}
	}

	return reqPayload
}
