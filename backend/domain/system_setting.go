package domain

import (
	"time"

	"github.com/chaitin/panda-wiki/consts"
)

// table: settings
type SystemSetting struct {
	ID          int                     `json:"id" gorm:"primary_key"`
	Key         consts.SystemSettingKey `json:"key"`
	Value       []byte                  `json:"value" gorm:"type:jsonb"` // JSON string
	Description string                  `json:"description"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
}

func (SystemSetting) TableName() string {
	return "system_settings"
}

// ModelModeSetting 模型配置结构体
type ModelModeSetting struct {
	EmbeddingIndexConfig     *EmbeddingIndexConfig     `json:"embedding_index_config,omitempty"` // Non-secret configuration last applied to the index
	Mode                     consts.ModelSettingMode   `json:"mode"`                             // 模式: manual 或 auto
	AutoProviderConfigs      []AutoProviderSettingItem `json:"auto_provider_configs"`            // 兼容上一版自动模式多供应商配置
	ActiveAutoProvider       string                    `json:"active_auto_provider"`             // 兼容上一版自动模式当前供应商 ID
	AutoProvider             *AutoModeProviderSetting  `json:"auto_provider"`                    // 自动模式统一供应商配置
	AutoModeAPIKey           string                    `json:"auto_mode_api_key"`                // 兼容旧百智云自动模式字段
	ChatModel                string                    `json:"chat_model"`                       // 兼容旧自动模式字段
	IsManualEmbeddingUpdated bool                      `json:"is_manual_embedding_updated"`      // 手动模式下嵌入模型是否更新
}

// Credentials and endpoint addresses do not define a vector space. Keep only
// model identity and supported parameters that affect indexed content here.
type EmbeddingIndexConfig struct {
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	ContextWindow int    `json:"context_window"`
	MaxTokens     int    `json:"max_tokens"`
}

type AutoProviderSettingItem struct {
	ID          string                 `json:"id"`
	Provider    string                 `json:"provider"`
	BaseURL     string                 `json:"base_url"`
	APIKey      string                 `json:"api_key"`
	APIHeader   string                 `json:"api_header"`
	APIVersion  string                 `json:"api_version"`
	ExtraConfig map[string]interface{} `json:"extra_config,omitempty"`
}

type AutoModeProviderSetting struct {
	Provider ModelProvider          `json:"provider"`
	BaseURL  string                 `json:"base_url"`
	BaseURLs AutoModeModelBaseURLs  `json:"base_urls,omitempty"`
	APIKey   string                 `json:"api_key"`
	Models   AutoModeModelSelection `json:"models"`

	// RerankBaseURL 兼容旧字段，读取后会迁移到 BaseURLs.Rerank。
	RerankBaseURL string `json:"rerank_base_url,omitempty"`
}

type AutoModeModelSelection struct {
	Chat       string `json:"chat"`
	Embedding  string `json:"embedding"`
	Rerank     string `json:"rerank"`
	Analysis   string `json:"analysis"`
	AnalysisVL string `json:"analysis_vl"`
}

type AutoModeModelBaseURLs struct {
	Chat       string `json:"chat,omitempty"`
	Embedding  string `json:"embedding,omitempty"`
	Rerank     string `json:"rerank,omitempty"`
	Analysis   string `json:"analysis,omitempty"`
	AnalysisVL string `json:"analysis_vl,omitempty"`
}

// UploadDeniedExtensionsSetting 上传禁止扩展名配置
// INSERT INTO "public"."system_settings" ("key", "value") VALUES ('upload', '{"denied_extensions": ["jsp"]}')
type UploadDeniedExtensionsSetting struct {
	DeniedExtensions []string `json:"denied_extensions"` // 禁止上传的文件扩展名列表，不带点，如 ["jsp", "php", "exe"]
}
