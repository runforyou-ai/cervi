package common

// OptionalString 把空字符串转换为空指针，用于写入数据库可空列。
func OptionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// StringValue 把可空字符串指针转换为字符串，空指针返回空串。
func StringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
