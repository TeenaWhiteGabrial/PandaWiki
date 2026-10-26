package domain

const AdministratorOnlyCode = 40301

type PWResponse struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Code    int    `json:"code"`
	TraceID string `json:"trace_id,omitempty"`
}

type PWResponseErrCode PWResponse

var (
	ErrCodeNil              = PWResponseErrCode{Message: "success", Success: true}
	ErrCodePermissionDenied = PWResponseErrCode{Message: "Permission Denied", Success: false, Code: 40003}
	ErrCodeNotFound         = PWResponseErrCode{Message: "Not Found", Success: false, Code: 40004}
	ErrCodeInternalError    = PWResponseErrCode{Message: "Internal Error", Success: false, Code: 50001}
)
