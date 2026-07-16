package domain

const (
	Bucket = "static-file"
)

type ObjectUploadResp struct {
	Key      string `json:"key"`
	Filename string `json:"filename"`
	URL      string `json:"url,omitempty"`
}

type UploadByUrlReq struct {
	KbId string `json:"kb_id"`
	Url  string `json:"url" validate:"required,url"`
}

type AnydocUploadResp struct {
	Code uint   `json:"code"`
	Err  string `json:"err"`
	Data string `json:"data"`
}

type GenerateFilePreviewReq struct {
	KBID     string `json:"kb_id" validate:"required"`
	FileKey  string `json:"file_key" validate:"required"`
	FileName string `json:"file_name" validate:"required"`
}

type GenerateFilePreviewResp struct {
	PreviewKey string `json:"preview_key"`
}
