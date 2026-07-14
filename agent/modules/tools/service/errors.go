package service

import "fmt"

// Error 表示 Tools v2 可安全返回给调用方的业务错误。
type Error struct {
	Status  int
	Code    string
	Message string
}

// Error 返回业务错误文本。
func (err *Error) Error() string { return fmt.Sprintf("%s: %s", err.Code, err.Message) }

var errorCatalog = map[string]struct {
	status  int
	message string
}{
	"400_PROVIDER_INVALID_CONFIG":      {400, "提供方配置缺少必填项"},
	"404_PROVIDER_NOT_FOUND":           {404, "提供方不存在"},
	"409_PROVIDER_NAME_CONFLICT":       {409, "提供方名称已存在，请更换名称"},
	"409_PROVIDER_NOT_ACTIVE":          {409, "提供方未激活，无法在其下创建工具"},
	"423_PROVIDER_IN_USE":              {423, "请先下线关联工具后再删除"},
	"400_INVALID_SCHEMA":               {400, "Schema 非法"},
	"400_INVALID_EXECUTION_CONFIG":     {400, "执行配置非法"},
	"400_OPENAPI_PARSE_FAILED":         {400, "接口规范解析失败，请检查格式后重试"},
	"404_TOOL_NOT_FOUND":               {404, "工具定义不存在"},
	"409_TOOL_NAME_CONFLICT":           {409, "同 provider 下工具重名"},
	"409_TOOL_PATH_CONFLICT":           {409, "同 provider 下已存在相同 method+path 的工具"},
	"400_CREDENTIAL_SCHEMA_INVALID":    {400, "凭据结构与模板不匹配"},
	"400_AUTH_TYPE_MISMATCH":           {400, "凭据结构与 Provider authType 不匹配"},
	"400_EMPTY_CREDENTIAL_NOT_ALLOWED": {400, "非 none 类型不允许空凭据"},
	"401_TOOL_CREDENTIAL_REVOKED":      {401, "工具授权已失效，请管理员更新凭据"},
	"404_CREDENTIAL_NOT_FOUND":         {404, "凭据不存在"},
	"409_CREDENTIAL_NAME_CONFLICT":     {409, "同 owner+provider 下凭据名重复"},
	"409_TOOL_NOT_TESTED":              {409, "激活前必须有一次成功测试记录"},
	"409_INVALID_STATE_TRANSITION":     {409, "状态转移不被允许"},
	"504_TOOL_TIMEOUT":                 {504, "工具调用超时"},
	"502_TOOL_UPSTREAM_ERROR":          {502, "上游服务返回非 2xx"},
	"500_TOOL_RUNTIME_ERROR":           {500, "执行器内部异常"},
	"400_LIMIT_EXCEEDED":               {400, "已超过工具挂载数量限制"},
	"409_CAPABILITY_NOT_ACTIVE":        {409, "工具未激活，暂不可挂载"},
	"403_TOOL_APPROVAL_REQUIRED":       {403, "该操作需要确认后执行"},
	"404_BINDING_NOT_FOUND":            {404, "工具挂载关系不存在"},
	"429_TOOL_CALL_BUDGET_EXCEEDED":    {429, "超过单轮调用预算"},
	"404_NOT_FOUND":                    {404, "决策记录不存在"},
}

// businessError 按稳定错误码创建业务错误。
func businessError(code string) error {
	descriptor, ok := errorCatalog[code]
	if !ok {
		return &Error{Status: 400, Code: code, Message: code}
	}
	return &Error{Status: descriptor.status, Code: code, Message: descriptor.message}
}
