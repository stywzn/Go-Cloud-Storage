package e

const (
	SUCCESS        = 200
	ERROR          = 500
	INVALID_PARAMS = 400
	
	// 业务错误码
	ERROR_UPLOAD_SAVE_FILE_FAIL = 10001
	ERROR_FILE_TOO_LARGE        = 10002
)

var MsgFlags = map[int]string{
	SUCCESS:                     "ok",
	ERROR:                       "fail",
	INVALID_PARAMS:              "请求参数错误",
	ERROR_UPLOAD_SAVE_FILE_FAIL: "保存文件失败",
	ERROR_FILE_TOO_LARGE:        "文件体积过大",
}

func GetMsg(code int) string {
	msg, ok := MsgFlags[code]
	if ok {
		return msg
	}
	return MsgFlags[ERROR]
}