package errors

const (
	CodeOK           = 0     // 成功
	CodeInvalidParam = 10001 // 参数错误
	CodeNotFound     = 10002 // 资源不存在
	CodeUnauthorized = 10003 // 未登录/无权限
	CodeInternal     = 10004 // 服务器内部错误
	CodePythonDown   = 20001 // Python 分析服务不可用
)
