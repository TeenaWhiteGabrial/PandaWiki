package v1

import (
	"github.com/google/wire"

	"github.com/chaitin/panda-wiki/handler"
	"github.com/chaitin/panda-wiki/middleware"
	"github.com/chaitin/panda-wiki/usecase"
)

type APIHandlers struct {
	UserHandler          *UserHandler
	KnowledgeBaseHandler *KnowledgeBaseHandler
	NodeHandler          *NodeHandler
	AppHandler           *AppHandler
	MCPHandler           *MCPHandler
	APITokenHandler      *APITokenHandler
	FileHandler          *FileHandler
	ModelHandler         *ModelHandler
	ConversationHandler  *ConversationHandler
	CrawlerHandler       *CrawlerHandler
	CreationHandler      *CreationHandler
	StatHandler          *StatHandler
	CommentHandler       *CommentHandler
	ContributeHandler    *ContributeHandler
	AuthV1Handler        *AuthV1Handler
	NavHandler           *NavHandler
	EnterpriseHandler    *EnterpriseHandler
	CollabHandler        *CollabHandler
}

var ProviderSet = wire.NewSet(
	middleware.ProviderSet,
	usecase.ProviderSet,

	handler.NewBaseHandler,
	NewNodeHandler,
	NewAppHandler,
	NewMCPHandler,
	NewAPITokenHandler,
	NewConversationHandler,
	NewUserHandler,
	NewFileHandler,
	NewModelHandler,
	NewKnowledgeBaseHandler,
	NewCrawlerHandler,
	NewCreationHandler,
	NewStatHandler,
	NewCommentHandler,
	NewContributeHandler,
	NewAuthV1Handler,
	NewNavHandler,
	NewEnterpriseHandler,
	NewCollabHandler,

	wire.Struct(new(APIHandlers), "*"),
)
