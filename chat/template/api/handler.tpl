package {{.PkgName}}

import (
    "fmt"
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"

    "chat/common/response"
	"chat/common/validator"
	{{.ImportPackages}}
)

func {{.HandlerName}}(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		{{if .HasRequest}}var req types.{{.RequestType}}
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(r, w, err)
			return
		}
        // validate check
        if err := validator.Validate.StructCtx(r.Context(), req); err != nil {
            errMap := validator.Translate(err, &req)
            for _, errFormat := range errMap {
                response.ParamError(r, w, fmt.Errorf(errFormat))
                return
            }
            response.ParamError(r, w, err)
            return
        }

		{{end}}l := {{.LogicName}}.New{{.LogicType}}(r.Context(), svcCtx)
		{{if .HasResp}}resp, {{end}}err := l.{{.Call}}({{if .HasRequest}}&req{{end}})
        {{if .HasResp}}response.Response(r, w, resp, err){{else}}response.Response(r, w, nil, err){{end}}
	}
}
