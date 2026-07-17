package errs

type IMErrorCode int32

const IMErrorCode_SUCCESS IMErrorCode = 0

// App request and authentication errors.
const (
	IMErrorCode_APP_APPKEY_REQUIRED  IMErrorCode = 17001
	IMErrorCode_APP_NOT_EXISTED      IMErrorCode = 17002
	IMErrorCode_APP_NOT_LOGIN        IMErrorCode = 17003
	IMErrorCode_APP_REQ_BODY_ILLEGAL IMErrorCode = 17004
	IMErrorCode_APP_ParamError       IMErrorCode = 17005
	IMErrorCode_APP_INTERNAL_TIMEOUT IMErrorCode = 17006
)

// App user account errors.
const (
	IMErrorCode_APP_USER_EXISTED      IMErrorCode = 17011
	IMErrorCode_APP_USER_NOT_EXIST    IMErrorCode = 17012
	IMErrorCode_APP_LOGIN_ERR_PASS    IMErrorCode = 17013
	IMErrorCode_APP_CHANNEL_NOT_EXIST IMErrorCode = 17014
	// IMErrorCode_APP_AGENT_NOT_BINDABLE 目标 Agent 不可绑定到 Inbox：不存在、未激活、
	// 不属于当前 AppKey，或没有可用的 active Bot（绑定后 Bot 需要加入 Ticket 群）。
	IMErrorCode_APP_AGENT_NOT_BINDABLE IMErrorCode = 17015
)

// AIBOT errors.
const (
	IMErrorCode_APP_AIBOT_DEFAULT           IMErrorCode = 17601
	IMErrorCode_APP_AIBOT_BotNotFound       IMErrorCode = 17602
	IMErrorCode_APP_AIBOT_NoPermission      IMErrorCode = 17603
	IMErrorCode_APP_AIBOT_AddBotFailed      IMErrorCode = 17604
	IMErrorCode_APP_AIBOT_UpdateBotFailed   IMErrorCode = 17605
	IMErrorCode_APP_AIBOT_DelBotFailed      IMErrorCode = 17606
	IMErrorCode_APP_AIBOT_AddMaterialFailed IMErrorCode = 17607
	IMErrorCode_APP_AIBOT_DelMaterialFailed IMErrorCode = 17608
)

var imCode2ApiErrorMap = map[IMErrorCode]*ApiErrorMsg{
	IMErrorCode_SUCCESS:                newApiErrorMsg(200, IMErrorCode_SUCCESS, "success"),
	IMErrorCode_APP_CHANNEL_NOT_EXIST:  newApiErrorMsg(200, IMErrorCode_APP_CHANNEL_NOT_EXIST, "渠道不存在"),
	IMErrorCode_APP_AGENT_NOT_BINDABLE: newApiErrorMsg(200, IMErrorCode_APP_AGENT_NOT_BINDABLE, "该智能体不可关联到收件箱：不存在、未激活或没有可用的 Bot"),
}

func GetApiErrorByCode(code IMErrorCode) *ApiErrorMsg {
	if err, ok := imCode2ApiErrorMap[code]; ok {
		return err
	}
	return newApiErrorMsg(200, code, "")
}

type ApiErrorMsg struct {
	HttpCode int         `json:"-"`
	Code     IMErrorCode `json:"code"`
	Msg      string      `json:"msg"`
}

func newApiErrorMsg(httpCode int, code IMErrorCode, msg string) *ApiErrorMsg {
	return &ApiErrorMsg{
		HttpCode: httpCode,
		Code:     code,
		Msg:      msg,
	}
}

type SuccHttpResp struct {
	ApiErrorMsg
	Data interface{} `json:"data"`
}
