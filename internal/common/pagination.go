package common

const (
	defaultPage     = 1
	defaultPageSize = 50
	maximumPageSize = 100
)

// NormalizePagination 补齐分页默认值并判断每页数量是否有效。
func NormalizePagination(page, pageSize int) (int, int, bool) {
	if page <= 0 {
		page = defaultPage
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	return page, pageSize, pageSize <= maximumPageSize
}
