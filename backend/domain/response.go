package domain

const AdministratorOnlyCode = 40301

type PWResponse struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Code    int    `json:"code"`
}

type PWResponseErrCode PWResponse

var (
	ErrCodeNil              = PWResponseErrCode{"success", true, nil, 0}
	ErrCodePermissionDenied = PWResponseErrCode{"Permission Denied", false, nil, 40003}
	ErrCodeNotFound         = PWResponseErrCode{"Not Found", false, nil, 40004}
	ErrCodeInternalError    = PWResponseErrCode{"Internal Error", false, nil, 50001}
)
