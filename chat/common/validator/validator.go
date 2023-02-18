package validator

import (
	"fmt"
	"os"
	"reflect"

	"github.com/go-playground/locales/zh"
	translator "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	translational "github.com/go-playground/validator/v10/translations/zh"
)

// Validate 验证器
var Validate *validator.Validate
var trans translator.Translator

func init() {
	uni := translator.New(zh.New())
	trans, _ = uni.GetTranslator("zh")
	Validate = validator.New()
	//注册一个函数，获取struct tag里自定义的label作为字段名--重点1
	Validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		label := fld.Tag.Get("label")
		if label == "" {
			return fld.Name
		}
		return label
	})
	//注册翻译器
	err := translational.RegisterDefaultTranslations(Validate, trans)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(0) //无法初始化验证器，退出应用
	}
}

// Translate 翻译工具
func Translate(err error, s interface{}) map[string]string {
	r := make(map[string]string)
	t := reflect.TypeOf(s).Elem()
	for _, err := range err.(validator.ValidationErrors) {
		//使用反射方法获取struct种的json标签作为key --重点2
		var k string
		if field, ok := t.FieldByName(err.StructField()); ok {
			k = field.Tag.Get("json")
		}
		if k == "" {
			k = err.StructField()
		}
		r[k] = err.Translate(trans)
	}
	return r
}
