package consts

type AutoModeDefaultModel string

const (
	AutoModeDefaultChatModel       AutoModeDefaultModel = "deepseek-chat"
	AutoModeDefaultEmbeddingModel  AutoModeDefaultModel = "bge-m3"
	AutoModeDefaultRerankModel     AutoModeDefaultModel = "bge-reranker-v2-m3"
	AutoModeDefaultAnalysisModel   AutoModeDefaultModel = "qwen2.5-3b-instruct"
	AutoModeDefaultAnalysisVLModel AutoModeDefaultModel = "qwen-vl-max-latest"
)

type ModelSettingMode string

const (
	ModelSettingModeManual ModelSettingMode = "manual"
	ModelSettingModeAuto   ModelSettingMode = "auto"
)

const AutoModeBaseURL = "https://model-square.app.baizhi.cloud/v1"

func GetAutoModeDefaultModel(provider string, modelType string) string {
	switch provider {
	case "BaiZhiCloud", "CompatiblePlatform":
		switch modelType {
		case "chat":
			return string(AutoModeDefaultChatModel)
		case "embedding":
			return string(AutoModeDefaultEmbeddingModel)
		case "rerank":
			return string(AutoModeDefaultRerankModel)
		case "analysis":
			return string(AutoModeDefaultAnalysisModel)
		case "analysis-vl":
			return string(AutoModeDefaultAnalysisVLModel)
		default:
			return string(AutoModeDefaultChatModel)
		}
	case "Xinference", "gpustack":
		switch modelType {
		case "chat":
			return string(AutoModeDefaultChatModel)
		case "embedding":
			return string(AutoModeDefaultEmbeddingModel)
		case "rerank":
			return string(AutoModeDefaultRerankModel)
		case "analysis":
			return string(AutoModeDefaultAnalysisModel)
		case "analysis-vl":
			return string(AutoModeDefaultAnalysisVLModel)
		default:
			return string(AutoModeDefaultChatModel)
		}
	default:
		return ""
	}
}

func GetAutoModeBaseURL(provider string) string {
	switch provider {
	case "BaiZhiCloud":
		return AutoModeBaseURL
	default:
		return ""
	}
}

func SupportsAutoModeProvider(provider string) bool {
	switch provider {
	case "BaiZhiCloud",
		"CompatiblePlatform",
		"Xinference",
		"gpustack":
		return true
	default:
		return false
	}
}
