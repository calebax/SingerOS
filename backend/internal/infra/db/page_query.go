package db

// PageQuery 通用分页查询参数
type PageQuery struct {
	Filters []Filter
	OrderBy []string
	OrgID   uint
	Uin     uint
	Offset  int
	Limit   int
	ListAll bool
}

// Filter 过滤条件
type Filter struct {
	Field      string
	Value      []string
	ExactMatch bool
}
