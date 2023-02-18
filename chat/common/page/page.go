package page

var (
	DefaultPageSize = 20
	DefaultPage     = 1
)

type Page struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

func NewPage(opt ...int) *Page {
	var page, limit int
	if len(opt) >= 2 {
		page = opt[0]
		limit = opt[1]
	}
	if len(opt) == 1 {
		page = opt[0]
	}
	if page < 1 {
		page = DefaultPage
	}
	if limit < 1 {
		limit = DefaultPageSize
	}
	return &Page{
		Page:     page,
		PageSize: limit,
	}
}

func (p *Page) Offset() uint64 {
	return uint64((p.Page - 1) * p.PageSize)
}

func (p *Page) Limit() uint64 {
	return uint64(p.PageSize)
}
