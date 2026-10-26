package usecase

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	v1 "github.com/chaitin/panda-wiki/api/share/v1"
	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/pkg/bot"
	"github.com/chaitin/panda-wiki/pkg/bot/dingtalk"
	"github.com/chaitin/panda-wiki/pkg/bot/discord"
	"github.com/chaitin/panda-wiki/pkg/bot/feishu"
	"github.com/chaitin/panda-wiki/pkg/bot/lark"
	"github.com/chaitin/panda-wiki/repo/pg"
	"github.com/chaitin/panda-wiki/store/cache"
)

type AppUsecase struct {
	repo          *pg.AppRepository
	authRepo      *pg.AuthRepo
	nodeRepo      *pg.NodeRepository
	navRepo       *pg.NavRepository
	kbRepo        *pg.KnowledgeBaseRepository
	nodeUsecase   *NodeUsecase
	chatUsecase   *ChatUsecase
	logger        *log.Logger
	config        *config.Config
	cache         *cache.Cache
	dingTalkBots  map[string]*dingtalk.DingTalkClient
	dingTalkMutex sync.RWMutex
	feishuBots    map[string]*feishu.FeishuClient
	feishuMutex   sync.RWMutex
	larkBots      map[string]*lark.LarkClient
	larkMutex     sync.RWMutex
	discordBots   map[string]*discord.DiscordClient
	discordMutex  sync.RWMutex
}

func NewAppUsecase(
	repo *pg.AppRepository,
	authRepo *pg.AuthRepo,
	navRepo *pg.NavRepository,
	nodeRepo *pg.NodeRepository,
	kbRepo *pg.KnowledgeBaseRepository,
	nodeUsecase *NodeUsecase,
	logger *log.Logger,
	config *config.Config,
	chatUsecase *ChatUsecase,
	cache *cache.Cache,
) *AppUsecase {
	u := &AppUsecase{
		repo:         repo,
		nodeUsecase:  nodeUsecase,
		chatUsecase:  chatUsecase,
		authRepo:     authRepo,
		navRepo:      navRepo,
		nodeRepo:     nodeRepo,
		kbRepo:       kbRepo,
		logger:       logger.WithModule("usecase.app"),
		config:       config,
		cache:        cache,
		dingTalkBots: make(map[string]*dingtalk.DingTalkClient),
		feishuBots:   make(map[string]*feishu.FeishuClient),
		larkBots:     make(map[string]*lark.LarkClient),
		discordBots:  make(map[string]*discord.DiscordClient),
	}

	// Initialize all valid DingTalkBot, FeishuBot, LarkBot and DiscordBot instances
	apps, err := u.repo.GetAppsByTypes(context.Background(), []domain.AppType{domain.AppTypeDingTalkBot, domain.AppTypeFeishuBot, domain.AppTypeLarkBot, domain.AppTypeDisCordBot})
	if err != nil {
		u.logger.Error("failed to get dingtalk bot apps", log.Error(err))
		return u
	}

	for _, app := range apps {
		switch app.Type {
		case domain.AppTypeDingTalkBot:
			u.updateDingTalkBot(app)
		case domain.AppTypeFeishuBot:
			u.updateFeishuBot(app)
		case domain.AppTypeLarkBot:
			u.updateLarkBot(app)
		case domain.AppTypeDisCordBot:
			u.updateDisCordBot(app)
		}
	}

	return u
}

func (u *AppUsecase) GetDefaultKnowledgeBaseByUser(ctx context.Context) (*domain.KnowledgeBaseListItem, error) {
	knowledgeBases, err := u.kbRepo.GetKnowledgeBaseListByUserId(ctx)
	if err != nil {
		return nil, err
	}
	if len(knowledgeBases) == 0 {
		return nil, fmt.Errorf("knowledge base not found")
	}
	return knowledgeBases[0], nil
}

func (u *AppUsecase) GetDefaultKnowledgeBase(ctx context.Context) (*domain.KnowledgeBaseListItem, error) {
	knowledgeBases, err := u.kbRepo.GetKnowledgeBaseList(ctx)
	if err != nil {
		return nil, err
	}
	if len(knowledgeBases) == 0 {
		return nil, fmt.Errorf("knowledge base not found")
	}
	return knowledgeBases[0], nil
}

func (u *AppUsecase) ValidateUpdateApp(ctx context.Context, id string, req *domain.UpdateAppReq) error {
	app, err := u.repo.GetAppDetail(ctx, id)
	if err != nil {
		return err
	}

	limitation := domain.GetBaseEditionLimitation(ctx)
	if !limitation.AllowCopyProtection && app.Settings.CopySetting != req.Settings.CopySetting {
		return domain.ErrPermissionDenied
	}

	if !limitation.AllowWatermark {
		if app.Settings.WatermarkSetting != req.Settings.WatermarkSetting || app.Settings.WatermarkContent != req.Settings.WatermarkContent {
			return domain.ErrPermissionDenied
		}
	}

	if !limitation.AllowAdvancedBot {
		if !slices.Equal(app.Settings.WechatServiceContainKeywords, req.Settings.WechatServiceContainKeywords) ||
			!slices.Equal(app.Settings.WechatServiceEqualKeywords, req.Settings.WechatServiceEqualKeywords) ||
			app.Settings.WechatServiceLogo != req.Settings.WechatServiceLogo {
			return domain.ErrPermissionDenied
		}

		if app.Settings.WeChatAppAdvancedSetting.FeedbackEnable != req.Settings.WeChatAppAdvancedSetting.FeedbackEnable ||
			app.Settings.WeChatAppAdvancedSetting.TextResponseEnable != req.Settings.WeChatAppAdvancedSetting.TextResponseEnable ||
			app.Settings.WeChatAppAdvancedSetting.Prompt != req.Settings.WeChatAppAdvancedSetting.Prompt ||
			!slices.Equal(app.Settings.WeChatAppAdvancedSetting.FeedbackType, req.Settings.WeChatAppAdvancedSetting.FeedbackType) ||
			app.Settings.WeChatAppAdvancedSetting.DisclaimerContent != req.Settings.WeChatAppAdvancedSetting.DisclaimerContent {
			return domain.ErrPermissionDenied
		}
	} else {
		if req.Settings.WeChatAppAdvancedSetting.Prompt == "" {
			req.Settings.WeChatAppAdvancedSetting.Prompt = domain.SystemDefaultPrompt
		}
	}

	if !limitation.AllowOpenAIBotSettings {
		if app.Settings.OpenAIAPIBotSettings.IsEnabled != req.Settings.OpenAIAPIBotSettings.IsEnabled || app.Settings.OpenAIAPIBotSettings.SecretKey != req.Settings.OpenAIAPIBotSettings.SecretKey {
			return domain.ErrPermissionDenied
		}
	}

	if !limitation.AllowCustomCopyright {
		if app.Settings.WidgetBotSettings.CopyrightHideEnabled != req.Settings.WidgetBotSettings.CopyrightHideEnabled || app.Settings.WidgetBotSettings.CopyrightInfo != req.Settings.WidgetBotSettings.CopyrightInfo {
			return domain.ErrPermissionDenied
		}
	}

	if !limitation.AllowMCPServer {
		if app.Settings.MCPServerSettings.IsEnabled != req.Settings.MCPServerSettings.IsEnabled {
			return domain.ErrPermissionDenied
		}
	}

	return nil
}

func (u *AppUsecase) UpdateApp(ctx context.Context, id string, appRequest *domain.UpdateAppReq) error {
	if appRequest.KbID == "" {
		kb, err := u.GetDefaultKnowledgeBaseByUser(ctx)
		if err != nil {
			return err
		}
		appRequest.KbID = kb.ID
	}

	if err := u.handleBotAuths(ctx, id, appRequest.Settings); err != nil {
		return err
	}

	if err := u.repo.UpdateApp(ctx, id, appRequest.KbID, appRequest); err != nil {
		return err
	}

	if appRequest.Settings != nil {
		app, err := u.repo.GetAppDetail(ctx, id)
		if err != nil {
			return err
		}
		switch app.Type {
		case domain.AppTypeDingTalkBot:
			u.updateDingTalkBot(app)
		case domain.AppTypeFeishuBot:
			u.updateFeishuBot(app)
		case domain.AppTypeLarkBot:
			u.updateLarkBot(app)
		case domain.AppTypeDisCordBot:
			u.updateDisCordBot(app)
		}
	}
	return nil
}

func (u *AppUsecase) getQAFunc(kbID string, appType domain.AppType) bot.GetQAFun {
	return func(ctx context.Context, msg string, info domain.ConversationInfo, ConversationID string) (chan string, error) {
		auth, err := u.authRepo.GetAuthByKBIDAndSourceType(ctx, kbID, appType.ToSourceType())
		if err != nil {
			u.logger.Error("get auth failed", log.Error(err))
			return nil, err
		}
		info.UserInfo.AuthUserID = auth.ID

		eventCh, err := u.chatUsecase.Chat(ctx, &domain.ChatRequest{
			Message:        msg,
			KBID:           kbID,
			AppType:        appType,
			RemoteIP:       "",
			ConversationID: ConversationID,
			Info:           info,
		})
		if err != nil {
			return nil, err
		}
		// check ai feedback. --> default is open
		appinfo, err := u.GetAppDetailByKBIDAndAppType(ctx, kbID, domain.AppTypeWeb)
		if err != nil {
			u.logger.Error("wechat GetAppDetailByKBIDAndAppType failed", log.Error(err))
		}

		var feedback = "\n\n---  \n\n本回答 基于 AI 生成，仅供参考。\n[👍 满意](%s) | [👎 不满意](%s)"
		var likeUrl = "%s/feedback?score=1&message_id=%s"
		var dislikeUrl = "%s/feedback?score=-1&message_id=%s"
		var messageId string
		var kb *domain.KnowledgeBase

		if appinfo.Settings.AIFeedbackSettings.AIFeedbackIsEnabled == nil || *appinfo.Settings.AIFeedbackSettings.AIFeedbackIsEnabled { // open
			kb, err = u.chatUsecase.llmUsecase.kbRepo.GetKnowledgeBaseByID(ctx, kbID)
			if err != nil {
				u.logger.Error("wechat GetKnowledgeBaseByID failed", log.Error(err))
			}

		}

		contentCh := make(chan string, 10)
		go func() {
			defer close(contentCh)
			for event := range eventCh {
				if event.Type == "done" || event.Type == "error" {
					break
				}
				if event.Type == "data" {
					contentCh <- event.Content
				}
				if event.Type == "message_id" {
					messageId = event.Content
				}
			}
			// check again
			// contact --> send
			if kb != nil && (appinfo.Settings.AIFeedbackSettings.AIFeedbackIsEnabled == nil || *appinfo.Settings.AIFeedbackSettings.AIFeedbackIsEnabled) { // open
				like := fmt.Sprintf(likeUrl, kb.AccessSettings.BaseURL, messageId)
				dislike := fmt.Sprintf(dislikeUrl, kb.AccessSettings.BaseURL, messageId)
				feedback_data := fmt.Sprintf(feedback, like, dislike)
				contentCh <- feedback_data
			}
		}()
		return contentCh, nil
	}
}

func (u *AppUsecase) updateFeishuBot(app *domain.App) {
	u.feishuMutex.Lock()
	defer u.feishuMutex.Unlock()

	if bot, exists := u.feishuBots[app.ID]; exists {
		if bot != nil {
			bot.Stop()
			delete(u.feishuBots, app.ID)
		}
	}

	if (app.Settings.FeishuBotIsEnabled != nil && !*app.Settings.FeishuBotIsEnabled) || app.Settings.FeishuBotAppID == "" || app.Settings.FeishuBotAppSecret == "" {
		return
	}

	getQA := u.getQAFunc(app.KBID, app.Type)

	botCtx, cancel := context.WithCancel(context.Background())
	feishuClient := feishu.NewFeishuClient(
		botCtx,
		cancel,
		app.Settings.FeishuBotAppID,
		app.Settings.FeishuBotAppSecret,
		u.logger,
		getQA,
	)

	go func() {
		u.logger.Info("feishu bot is starting", log.String("app_id", app.Settings.FeishuBotAppID))
		err := feishuClient.Start()
		if err != nil {
			u.logger.Error("failed to start feishu client", log.Error(err))
			cancel()
			return
		}
	}()

	u.feishuBots[app.ID] = feishuClient
}

func (u *AppUsecase) updateLarkBot(app *domain.App) {
	u.larkMutex.Lock()
	defer u.larkMutex.Unlock()

	if bot, exists := u.larkBots[app.ID]; exists {
		if bot != nil {
			bot.Stop()
			delete(u.larkBots, app.ID)
		}
	}

	if (app.Settings.LarkBotSettings.IsEnabled != nil && !*app.Settings.LarkBotSettings.IsEnabled) || app.Settings.LarkBotSettings.AppID == "" || app.Settings.LarkBotSettings.AppSecret == "" {
		return
	}

	getQA := u.getQAFunc(app.KBID, app.Type)

	botCtx, cancel := context.WithCancel(context.Background())
	larkClient, err := lark.NewLarkClient(
		botCtx,
		cancel,
		app.Settings.LarkBotSettings.AppID,
		app.Settings.LarkBotSettings.AppSecret,
		app.Settings.LarkBotSettings.VerifyToken,
		app.Settings.LarkBotSettings.EncryptKey,
		u.logger,
		getQA,
	)
	if err != nil {
		u.logger.Error("failed to create lark client", log.Error(err))
		return
	}

	go func() {
		u.logger.Info("lark bot is starting", log.String("app_id", app.Settings.LarkBotSettings.AppID))
		err := larkClient.Start()
		if err != nil {
			u.logger.Error("failed to start lark client", log.Error(err))
			cancel()
			return
		}
	}()

	u.larkBots[app.ID] = larkClient
}

func (u *AppUsecase) updateDingTalkBot(app *domain.App) {
	u.dingTalkMutex.Lock()
	defer u.dingTalkMutex.Unlock()

	if bot, exists := u.dingTalkBots[app.ID]; exists {
		if bot != nil {
			bot.Stop()
			delete(u.dingTalkBots, app.ID)
		}
	}

	if (app.Settings.DingTalkBotIsEnabled != nil && !*app.Settings.DingTalkBotIsEnabled) || app.Settings.DingTalkBotClientID == "" || app.Settings.DingTalkBotClientSecret == "" {
		return
	}

	getQA := u.getQAFunc(app.KBID, app.Type)

	botCtx, cancel := context.WithCancel(context.Background())
	dingTalkClient, err := dingtalk.NewDingTalkClient(
		botCtx,
		cancel,
		app.Settings.DingTalkBotClientID,
		app.Settings.DingTalkBotClientSecret,
		app.Settings.DingTalkBotTemplateID,
		u.logger,
		getQA,
	)
	if err != nil {
		u.logger.Error("failed to create dingtalk client", log.Error(err))
		return
	}

	go func() {
		u.logger.Info("dingtalk bot is starting", log.String("client_id", app.Settings.DingTalkBotClientID))
		err := dingTalkClient.Start()
		if err != nil {
			u.logger.Error("failed to start dingtalk bot", log.Error(err))
			cancel()
			return
		}
	}()

	u.dingTalkBots[app.ID] = dingTalkClient
}

func (u *AppUsecase) updateDisCordBot(app *domain.App) {
	u.discordMutex.Lock()
	defer u.discordMutex.Unlock()

	if bot, exists := u.discordBots[app.ID]; exists {
		if bot != nil {
			if err := bot.Stop(); err != nil {
				u.logger.Error("failed to stop discord bot", log.Error(err))
			}
			delete(u.discordBots, app.ID)
		}
	}
	token := app.Settings.DiscordBotToken
	if (app.Settings.DiscordBotIsEnabled != nil && !*app.Settings.DiscordBotIsEnabled) || token == "" {
		return
	}

	getQA := u.getQAFunc(app.KBID, app.Type)

	discordBots, err := discord.NewDiscordClient(
		u.logger, token, getQA,
	)
	if err != nil {
		u.logger.Error("failed to create discord client", log.Error(err))
		return
	}

	if err := discordBots.Start(); err != nil {
		u.logger.Error("failed to start discord bot", log.Error(err))
		return
	}

	u.logger.Info("discord bot is starting", log.String("token", token))
	u.discordBots[app.ID] = discordBots
}

func (u *AppUsecase) DeleteApp(ctx context.Context, id, kbID string) error {
	return u.repo.DeleteApp(ctx, id, kbID)
}

// GetLarkBotClient returns the Lark bot client for a given app ID
// This is used to access the event handler for HTTP callbacks
func (u *AppUsecase) GetLarkBotClient(appID string) (*lark.LarkClient, bool) {
	u.larkMutex.RLock()
	defer u.larkMutex.RUnlock()
	client, ok := u.larkBots[appID]
	return client, ok
}

func (u *AppUsecase) GetAppDetailByKBIDAndAppType(ctx context.Context, kbID string, appType domain.AppType) (*domain.AppDetailResp, error) {
	app, err := u.repo.GetOrCreateAppByKBIDAndType(ctx, kbID, appType)
	if err != nil {
		return nil, err
	}
	appDetailResp := &domain.AppDetailResp{
		ID:   app.ID,
		KBID: app.KBID,
		Name: app.Name,
		Type: app.Type,
	}
	var webAppLandingConfigs []domain.WebAppLandingConfigResp
	for i := range app.Settings.WebAppLandingConfigs {
		webAppLandingConfigResp := domain.WebAppLandingConfigResp{
			Type:            app.Settings.WebAppLandingConfigs[i].Type,
			BannerConfig:    app.Settings.WebAppLandingConfigs[i].BannerConfig,
			BasicDocConfig:  app.Settings.WebAppLandingConfigs[i].BasicDocConfig,
			DirDocConfig:    app.Settings.WebAppLandingConfigs[i].DirDocConfig,
			NavDocConfig:    app.Settings.WebAppLandingConfigs[i].NavDocConfig,
			SimpleDocConfig: app.Settings.WebAppLandingConfigs[i].SimpleDocConfig,
			CarouselConfig:  app.Settings.WebAppLandingConfigs[i].CarouselConfig,
			FaqConfig:       app.Settings.WebAppLandingConfigs[i].FaqConfig,
			TextConfig:      app.Settings.WebAppLandingConfigs[i].TextConfig,
			CaseConfig:      app.Settings.WebAppLandingConfigs[i].CaseConfig,
			MetricsConfig:   app.Settings.WebAppLandingConfigs[i].MetricsConfig,
			CommentConfig:   app.Settings.WebAppLandingConfigs[i].CommentConfig,
			FeatureConfig:   app.Settings.WebAppLandingConfigs[i].FeatureConfig,
			ImgTextConfig:   app.Settings.WebAppLandingConfigs[i].ImgTextConfig,
			TextImgConfig:   app.Settings.WebAppLandingConfigs[i].TextImgConfig,
			QuestionConfig:  app.Settings.WebAppLandingConfigs[i].QuestionConfig,
			BlockGridConfig: app.Settings.WebAppLandingConfigs[i].BlockGridConfig,
			ComConfigOrder:  app.Settings.WebAppLandingConfigs[i].ComConfigOrder,
			NodeIds:         app.Settings.WebAppLandingConfigs[i].NodeIds,
		}
		webAppLandingConfigs = append(webAppLandingConfigs, webAppLandingConfigResp)
	}
	appDetailResp.Settings = domain.AppSettingsResp{
		Title:              app.Settings.Title,
		Icon:               app.Settings.Icon,
		Favicon:            app.Settings.Favicon,
		AdminTitle:         app.Settings.AdminTitle,
		AdminLogo:          app.Settings.AdminLogo,
		AdminFavicon:       app.Settings.AdminFavicon,
		Btns:               app.Settings.Btns,
		WelcomeStr:         app.Settings.WelcomeStr,
		SearchPlaceholder:  app.Settings.SearchPlaceholder,
		RecommendQuestions: app.Settings.RecommendQuestions,
		RecommendNodeIDs:   app.Settings.RecommendNodeIDs,
		Desc:               app.Settings.Desc,
		Keyword:            app.Settings.Keyword,
		HeadCode:           app.Settings.HeadCode,
		BodyCode:           app.Settings.BodyCode,
		// DingTalkBot
		DingTalkBotIsEnabled:    app.Settings.DingTalkBotIsEnabled,
		DingTalkBotClientID:     app.Settings.DingTalkBotClientID,
		DingTalkBotClientSecret: app.Settings.DingTalkBotClientSecret,
		DingTalkBotTemplateID:   app.Settings.DingTalkBotTemplateID,
		// FeishuBot
		FeishuBotIsEnabled: app.Settings.FeishuBotIsEnabled,
		FeishuBotAppID:     app.Settings.FeishuBotAppID,
		FeishuBotAppSecret: app.Settings.FeishuBotAppSecret,
		// LarkBot
		LarkBotSettings: app.Settings.LarkBotSettings,
		// WechatBot
		WeChatAppIsEnabled:       app.Settings.WeChatAppIsEnabled,
		WeChatAppToken:           app.Settings.WeChatAppToken,
		WeChatAppCorpID:          app.Settings.WeChatAppCorpID,
		WeChatAppEncodingAESKey:  app.Settings.WeChatAppEncodingAESKey,
		WeChatAppSecret:          app.Settings.WeChatAppSecret,
		WeChatAppAgentID:         app.Settings.WeChatAppAgentID,
		WeChatAppAdvancedSetting: app.Settings.WeChatAppAdvancedSetting,
		// WechatServiceBot
		WeChatServiceIsEnabled:       app.Settings.WeChatServiceIsEnabled,
		WeChatServiceToken:           app.Settings.WeChatServiceToken,
		WeChatServiceEncodingAESKey:  app.Settings.WeChatServiceEncodingAESKey,
		WeChatServiceCorpID:          app.Settings.WeChatServiceCorpID,
		WeChatServiceSecret:          app.Settings.WeChatServiceSecret,
		WechatServiceContainKeywords: app.Settings.WechatServiceContainKeywords,
		WechatServiceEqualKeywords:   app.Settings.WechatServiceEqualKeywords,
		WechatServiceLogo:            app.Settings.WechatServiceLogo,
		// Discord
		DiscordBotIsEnabled: app.Settings.DiscordBotIsEnabled,
		DiscordBotToken:     app.Settings.DiscordBotToken,
		// WechatOfficialAccount
		WechatOfficialAccountIsEnabled:      app.Settings.WechatOfficialAccountIsEnabled,
		WechatOfficialAccountAppID:          app.Settings.WechatOfficialAccountAppID,
		WechatOfficialAccountAppSecret:      app.Settings.WechatOfficialAccountAppSecret,
		WechatOfficialAccountToken:          app.Settings.WechatOfficialAccountToken,
		WechatOfficialAccountEncodingAESKey: app.Settings.WechatOfficialAccountEncodingAESKey,
		// theme
		ThemeMode:     app.Settings.ThemeMode,
		ThemeAndStyle: app.Settings.ThemeAndStyle,
		// catalog settings
		CatalogSettings: app.Settings.CatalogSettings,
		// footer settings
		FooterSettings: app.Settings.FooterSettings,
		// widget bot settings
		WidgetBotSettings: app.Settings.WidgetBotSettings,
		// webapp comment settings
		WebAppCommentSettings: app.Settings.WebAppCommentSettings,
		// document feedback
		DocumentFeedBackIsEnabled: app.Settings.DocumentFeedBackIsEnabled,
		// AI Feedback
		AIFeedbackSettings: app.Settings.AIFeedbackSettings,
		// WebApp Custom Settings
		WebAppCustomSettings: app.Settings.WebAppCustomSettings,
		// openai api settings
		OpenAIAPIBotSettings: app.Settings.OpenAIAPIBotSettings,
		// disclaimer settings
		DisclaimerSettings: app.Settings.DisclaimerSettings,
		// webapp landing settings
		WebAppLandingConfigs: webAppLandingConfigs,
		WebAppLandingTheme:   app.Settings.WebAppLandingTheme,

		WatermarkContent:    app.Settings.WatermarkContent,
		WatermarkSetting:    app.Settings.WatermarkSetting,
		CopySetting:         app.Settings.CopySetting,
		ContributeSettings:  app.Settings.ContributeSettings,
		HomePageSetting:     app.Settings.HomePageSetting,
		ConversationSetting: app.Settings.ConversationSetting,

		WecomAIBotSettings: app.Settings.WecomAIBotSettings,

		MCPServerSettings: app.Settings.MCPServerSettings,
		StatsSetting:      app.Settings.StatsSetting,
	}

	// init ai feedback string
	if app.Settings.AIFeedbackSettings.AIFeedbackType == nil {
		appDetailResp.Settings.AIFeedbackSettings.AIFeedbackType = []string{"内容不准确", "没有帮助", "其他"}
	}
	if appDetailResp.Settings.HomePageSetting == "" {
		appDetailResp.Settings.HomePageSetting = consts.HomePageSettingDoc
	}

	// get recommend nodes
	if len(app.Settings.RecommendNodeIDs) > 0 {
		nodes, err := u.nodeUsecase.GetRecommendNodeList(ctx, &domain.GetRecommendNodeListReq{
			KBID:    kbID,
			NodeIDs: app.Settings.RecommendNodeIDs,
		})
		if err != nil {
			return nil, err
		}
		appDetailResp.RecommendNodes = nodes
	}
	return appDetailResp, nil
}

func (u *AppUsecase) SanitizeAppDetailForDocManage(app *domain.AppDetailResp) *domain.AppDetailResp {
	if app == nil {
		return nil
	}

	sanitized := &domain.AppDetailResp{
		ID:   app.ID,
		KBID: app.KBID,
		Name: app.Name,
		Type: app.Type,
	}

	if app.Type != domain.AppTypeWeb {
		return sanitized
	}

	sanitized.Settings = domain.AppSettingsResp{
		Title:               app.Settings.Title,
		Icon:                app.Settings.Icon,
		Favicon:             app.Settings.Favicon,
		AdminTitle:          app.Settings.AdminTitle,
		AdminLogo:           app.Settings.AdminLogo,
		AdminFavicon:        app.Settings.AdminFavicon,
		ThemeMode:           app.Settings.ThemeMode,
		ThemeAndStyle:       app.Settings.ThemeAndStyle,
		CatalogSettings:     app.Settings.CatalogSettings,
		WatermarkContent:    app.Settings.WatermarkContent,
		WatermarkSetting:    app.Settings.WatermarkSetting,
		CopySetting:         app.Settings.CopySetting,
		ContributeSettings:  app.Settings.ContributeSettings,
		ConversationSetting: app.Settings.ConversationSetting,
		HomePageSetting:     app.Settings.HomePageSetting,
	}

	return sanitized
}

func (u *AppUsecase) GetMCPServerAppInfo(ctx context.Context, kbID string) (*domain.AppInfoResp, error) {
	apiApp, err := u.repo.GetOrCreateAppByKBIDAndType(ctx, kbID, domain.AppTypeMcpServer)
	if err != nil {
		return nil, err
	}
	appInfo := &domain.AppInfoResp{
		Settings: domain.AppSettingsResp{
			MCPServerSettings: apiApp.Settings.MCPServerSettings,
		},
	}
	return appInfo, nil
}

func (u *AppUsecase) GetWebAppBrand(ctx context.Context, kbID string) (*domain.AppBrandResp, error) {
	app, err := u.repo.GetOrCreateAppByKBIDAndType(ctx, kbID, domain.AppTypeWeb)
	if err != nil {
		return nil, err
	}
	return &domain.AppBrandResp{Settings: domain.AppBrandSettings{
		Title:        app.Settings.Title,
		Icon:         app.Settings.Icon,
		Favicon:      app.Settings.Favicon,
		AdminTitle:   app.Settings.AdminTitle,
		AdminLogo:    app.Settings.AdminLogo,
		AdminFavicon: app.Settings.AdminFavicon,
	}}, nil
}

func (u *AppUsecase) ShareGetWebAppInfo(ctx context.Context, kbID string, userID string) (*domain.AppInfoResp, error) {
	kb, err := u.kbRepo.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		u.logger.Error("get kb failed", log.Error(err), log.String("kb_id", kbID))
		return nil, err
	}

	app, err := u.repo.GetOrCreateAppByKBIDAndType(ctx, kbID, domain.AppTypeWeb)
	if err != nil {
		return nil, err
	}
	var webAppLandingConfigs []domain.WebAppLandingConfigResp
	for i := range app.Settings.WebAppLandingConfigs {
		webAppLandingConfigResp := domain.WebAppLandingConfigResp{
			Type:            app.Settings.WebAppLandingConfigs[i].Type,
			BannerConfig:    app.Settings.WebAppLandingConfigs[i].BannerConfig,
			BasicDocConfig:  app.Settings.WebAppLandingConfigs[i].BasicDocConfig,
			DirDocConfig:    app.Settings.WebAppLandingConfigs[i].DirDocConfig,
			NavDocConfig:    app.Settings.WebAppLandingConfigs[i].NavDocConfig,
			SimpleDocConfig: app.Settings.WebAppLandingConfigs[i].SimpleDocConfig,
			CarouselConfig:  app.Settings.WebAppLandingConfigs[i].CarouselConfig,
			FaqConfig:       app.Settings.WebAppLandingConfigs[i].FaqConfig,
			TextConfig:      app.Settings.WebAppLandingConfigs[i].TextConfig,
			CaseConfig:      app.Settings.WebAppLandingConfigs[i].CaseConfig,
			CommentConfig:   app.Settings.WebAppLandingConfigs[i].CommentConfig,
			FeatureConfig:   app.Settings.WebAppLandingConfigs[i].FeatureConfig,
			ImgTextConfig:   app.Settings.WebAppLandingConfigs[i].ImgTextConfig,
			TextImgConfig:   app.Settings.WebAppLandingConfigs[i].TextImgConfig,
			MetricsConfig:   app.Settings.WebAppLandingConfigs[i].MetricsConfig,
			QuestionConfig:  app.Settings.WebAppLandingConfigs[i].QuestionConfig,
			BlockGridConfig: app.Settings.WebAppLandingConfigs[i].BlockGridConfig,
			ComConfigOrder:  app.Settings.WebAppLandingConfigs[i].ComConfigOrder,
			NodeIds:         app.Settings.WebAppLandingConfigs[i].NodeIds,
		}
		if app.Settings.WebAppLandingConfigs[i].NavDocConfig != nil {
			navNodes, err := u.GetRecommendNodesByNavIds(ctx, kbID, app.Settings.WebAppLandingConfigs[i].NavDocConfig.NavIds, userID)
			if err != nil {
				return nil, err
			}
			webAppLandingConfigResp.Nodes = navNodes
		} else {
			nodes, err := u.GetRecommendNodesByIds(ctx, kbID, app.Settings.WebAppLandingConfigs[i].NodeIds, userID)
			if err != nil {
				return nil, err
			}
			webAppLandingConfigResp.Nodes = nodes
		}
		webAppLandingConfigs = append(webAppLandingConfigs, webAppLandingConfigResp)
	}
	appInfo := &domain.AppInfoResp{
		Name:    app.Name,
		BaseUrl: kb.AccessSettings.BaseURL,
		Settings: domain.AppSettingsResp{
			Title:              app.Settings.Title,
			Icon:               app.Settings.Icon,
			Favicon:            app.Settings.Favicon,
			AdminTitle:         app.Settings.AdminTitle,
			AdminLogo:          app.Settings.AdminLogo,
			AdminFavicon:       app.Settings.AdminFavicon,
			Btns:               app.Settings.Btns,
			WelcomeStr:         app.Settings.WelcomeStr,
			SearchPlaceholder:  app.Settings.SearchPlaceholder,
			RecommendQuestions: app.Settings.RecommendQuestions,
			RecommendNodeIDs:   app.Settings.RecommendNodeIDs,
			Desc:               app.Settings.Desc,
			Keyword:            app.Settings.Keyword,
			HeadCode:           app.Settings.HeadCode,
			BodyCode:           app.Settings.BodyCode,
			// theme
			ThemeMode:     app.Settings.ThemeMode,
			ThemeAndStyle: app.Settings.ThemeAndStyle,
			// catalog settings
			CatalogSettings: app.Settings.CatalogSettings,
			// footer settings
			FooterSettings: app.Settings.FooterSettings,
			// widget bot settings
			WebAppCommentSettings: app.Settings.WebAppCommentSettings,
			// document feedback
			DocumentFeedBackIsEnabled: app.Settings.DocumentFeedBackIsEnabled,
			// AI Feedback
			AIFeedbackSettings: app.Settings.AIFeedbackSettings,
			// WebApp Custom Settings
			WebAppCustomSettings: app.Settings.WebAppCustomSettings,
			// Disclaimer Settings
			DisclaimerSettings: app.Settings.DisclaimerSettings,
			// WebApp Landing Settings
			WebAppLandingConfigs: webAppLandingConfigs,
			WebAppLandingTheme:   app.Settings.WebAppLandingTheme,

			WatermarkContent:    app.Settings.WatermarkContent,
			WatermarkSetting:    app.Settings.WatermarkSetting,
			CopySetting:         app.Settings.CopySetting,
			ContributeSettings:  app.Settings.ContributeSettings,
			HomePageSetting:     app.Settings.HomePageSetting,
			ConversationSetting: app.Settings.ConversationSetting,
			StatsSetting:        app.Settings.StatsSetting,
		},
	}
	// init ai feedback string
	if app.Settings.AIFeedbackSettings.AIFeedbackType == nil {
		appInfo.Settings.AIFeedbackSettings.AIFeedbackType = []string{"内容不准确", "没有帮助", "其他"}
	}
	if app.Settings.HomePageSetting == "" {
		appInfo.Settings.HomePageSetting = consts.HomePageSettingDoc
	}
	showBrand := true
	defaultDisclaimer := "本回答 基于 AI 生成，仅供参考。"

	if !domain.GetBaseEditionLimitation(ctx).AllowCustomCopyright {
		appInfo.Settings.WebAppCustomSettings.ShowBrandInfo = &showBrand
		appInfo.Settings.DisclaimerSettings.Content = &defaultDisclaimer
	} else {
		if appInfo.Settings.DisclaimerSettings.Content == nil {
			appInfo.Settings.DisclaimerSettings.Content = &defaultDisclaimer
		}
	}

	return appInfo, nil
}

func (u *AppUsecase) GetWidgetAppInfo(ctx context.Context, kbID string) (*domain.AppInfoResp, error) {
	webApp, err := u.repo.GetOrCreateAppByKBIDAndType(ctx, kbID, domain.AppTypeWeb)
	if err != nil {
		return nil, err
	}
	widgetApp, err := u.repo.GetOrCreateAppByKBIDAndType(ctx, kbID, domain.AppTypeWidget)
	if err != nil {
		return nil, err
	}
	appInfo := &domain.AppInfoResp{
		Settings: domain.AppSettingsResp{
			Title:              webApp.Settings.Title,
			Icon:               webApp.Settings.Icon,
			WelcomeStr:         webApp.Settings.WelcomeStr,
			SearchPlaceholder:  webApp.Settings.SearchPlaceholder,
			RecommendQuestions: widgetApp.Settings.WidgetBotSettings.RecommendQuestions,
			WidgetBotSettings:  widgetApp.Settings.WidgetBotSettings,
		},
	}
	if len(widgetApp.Settings.WidgetBotSettings.RecommendNodeIDs) > 0 {
		nodes, err := u.nodeUsecase.GetRecommendNodeList(ctx, &domain.GetRecommendNodeListReq{
			KBID:    kbID,
			NodeIDs: widgetApp.Settings.WidgetBotSettings.RecommendNodeIDs,
		})
		if err != nil {
			return nil, err
		}
		appInfo.RecommendNodes = nodes
	}

	if !domain.GetBaseEditionLimitation(ctx).AllowCustomCopyright {
		appInfo.Settings.WidgetBotSettings.CopyrightHideEnabled = false
		appInfo.Settings.WidgetBotSettings.CopyrightInfo = domain.SettingCopyrightInfo
	}

	return appInfo, nil
}

func (u *AppUsecase) GetWechatAppInfo(ctx context.Context, kbID string) (*v1.WechatAppInfoResp, error) {
	wechatApp, err := u.repo.GetOrCreateAppByKBIDAndType(ctx, kbID, domain.AppTypeWechatBot)
	if err != nil {
		return nil, err
	}

	resp := &v1.WechatAppInfoResp{}

	if wechatApp.Settings.WeChatAppIsEnabled != nil {
		resp.WeChatAppIsEnabled = *wechatApp.Settings.WeChatAppIsEnabled
	}

	if domain.GetBaseEditionLimitation(ctx).AllowAdvancedBot {
		resp.FeedbackEnable = wechatApp.Settings.WeChatAppAdvancedSetting.FeedbackEnable
		resp.FeedbackType = wechatApp.Settings.WeChatAppAdvancedSetting.FeedbackType
		resp.DisclaimerContent = wechatApp.Settings.WeChatAppAdvancedSetting.DisclaimerContent
	}

	return resp, nil
}

func (u *AppUsecase) handleBotAuths(ctx context.Context, id string, newSettings *domain.AppSettings) error {

	currentApp, err := u.repo.GetAppDetail(ctx, id)
	if err != nil {
		return err
	}

	switch currentApp.Type {

	}
	// Handle Widget Bot
	if currentApp.Settings.WidgetBotSettings.IsOpen != newSettings.WidgetBotSettings.IsOpen {
		if err := u.handleBotAuth(ctx, currentApp.KBID, currentApp.ID, &currentApp.Settings.WidgetBotSettings.IsOpen,
			&newSettings.WidgetBotSettings.IsOpen, consts.SourceTypeWidget); err != nil {
			u.logger.Error("failed to handle widget auth", log.Error(err))
		}
	}

	// Handle DingTalk Bot
	if currentApp.Settings.DingTalkBotIsEnabled != newSettings.DingTalkBotIsEnabled {
		if err := u.handleBotAuth(ctx, currentApp.KBID, currentApp.ID, currentApp.Settings.DingTalkBotIsEnabled,
			newSettings.DingTalkBotIsEnabled, consts.SourceTypeDingtalkBot); err != nil {
			u.logger.Error("failed to handle dingtalk bot auth", log.Error(err))
		}
	}

	// Handle Feishu Bot
	if currentApp.Settings.FeishuBotIsEnabled != newSettings.FeishuBotIsEnabled {
		if err := u.handleBotAuth(ctx, currentApp.KBID, currentApp.ID, currentApp.Settings.FeishuBotIsEnabled,
			newSettings.FeishuBotIsEnabled, consts.SourceTypeFeishuBot); err != nil {
			u.logger.Error("failed to handle feishu bot auth", log.Error(err))
		}
	}

	// Handle Lark Bot
	if currentApp.Settings.LarkBotSettings.IsEnabled != newSettings.LarkBotSettings.IsEnabled {
		if err := u.handleBotAuth(ctx, currentApp.KBID, currentApp.ID, currentApp.Settings.LarkBotSettings.IsEnabled,
			newSettings.LarkBotSettings.IsEnabled, consts.SourceTypeLarkBot); err != nil {
			u.logger.Error("failed to handle lark bot auth", log.Error(err))
		}
	}

	// Handle WeChat Bot
	if currentApp.Settings.WeChatAppIsEnabled != newSettings.WeChatAppIsEnabled {
		if err := u.handleBotAuth(ctx, currentApp.KBID, currentApp.ID, currentApp.Settings.WeChatAppIsEnabled,
			newSettings.WeChatAppIsEnabled, consts.SourceTypeWechatBot); err != nil {
			u.logger.Error("failed to handle wechat bot auth", log.Error(err))
		}
	}

	// Handle WeChat Service Bot
	if currentApp.Settings.WeChatServiceIsEnabled != newSettings.WeChatServiceIsEnabled {
		if err := u.handleBotAuth(ctx, currentApp.KBID, currentApp.ID, currentApp.Settings.WeChatServiceIsEnabled,
			newSettings.WeChatServiceIsEnabled, consts.SourceTypeWechatServiceBot); err != nil {
			u.logger.Error("failed to handle wechat service bot auth", log.Error(err))
		}
	}

	// Handle Discord Bot
	if currentApp.Settings.DiscordBotIsEnabled != newSettings.DiscordBotIsEnabled {
		if err := u.handleBotAuth(ctx, currentApp.KBID, currentApp.ID, currentApp.Settings.DiscordBotIsEnabled,
			newSettings.DiscordBotIsEnabled, consts.SourceTypeDiscordBot); err != nil {
			u.logger.Error("failed to handle discord bot auth", log.Error(err))
		}
	}

	// Handle WeChat Official Account
	if currentApp.Settings.WechatOfficialAccountIsEnabled != newSettings.WechatOfficialAccountIsEnabled {
		if err := u.handleBotAuth(ctx, currentApp.KBID, currentApp.ID, currentApp.Settings.WechatOfficialAccountIsEnabled,
			newSettings.WechatOfficialAccountIsEnabled, consts.SourceTypeWechatOfficialAccount); err != nil {
			u.logger.Error("failed to handle wechat official account auth", log.Error(err))
		}
	}

	// Handle OpenAI API BOT Account
	if currentApp.Settings.OpenAIAPIBotSettings.IsEnabled != newSettings.OpenAIAPIBotSettings.IsEnabled {
		if err := u.handleBotAuth(ctx, currentApp.KBID, currentApp.ID, &currentApp.Settings.OpenAIAPIBotSettings.IsEnabled,
			&newSettings.OpenAIAPIBotSettings.IsEnabled, consts.SourceTypeOpenAIAPI); err != nil {
			u.logger.Error("failed to handle openai api bot auth", log.Error(err))
		}
	}

	// Handle Wecom AI Bot
	if currentApp.Settings.WecomAIBotSettings.IsEnabled != newSettings.WecomAIBotSettings.IsEnabled {
		if err := u.handleBotAuth(ctx, currentApp.KBID, currentApp.ID, &currentApp.Settings.WecomAIBotSettings.IsEnabled,
			&newSettings.WecomAIBotSettings.IsEnabled, consts.SourceTypeWecomAIBot); err != nil {
			u.logger.Error("failed to handle wecom ai bot account auth", log.Error(err))
		}
	}

	return nil
}

func (u *AppUsecase) handleBotAuth(ctx context.Context, kbID, appId string, currentEnabled, newEnabled *bool, sourceType consts.SourceType) error {
	wasEnabled := currentEnabled != nil && *currentEnabled
	isEnabled := newEnabled != nil && *newEnabled

	if !wasEnabled && isEnabled {
		rdsKey := fmt.Sprintf("handleBotAuth:%s:%s", kbID, sourceType)
		if !u.cache.AcquireLock(ctx, rdsKey) {
			return fmt.Errorf("bot auth creation is in progress, please try again later")
		}
		defer u.cache.ReleaseLock(ctx, rdsKey)

		existingAuth, _ := u.authRepo.GetAuthByKBIDAndSourceType(ctx, kbID, sourceType)
		if existingAuth != nil {
			return nil
		}

		auth := &domain.Auth{
			KBID:          kbID,
			UnionID:       fmt.Sprintf("bot_%s_%s", appId, sourceType),
			SourceType:    sourceType,
			LastLoginTime: time.Now(),
			UserInfo: domain.AuthUserInfo{
				Username: sourceType.Name(),
			},
		}

		if err := u.authRepo.CreateAuth(ctx, auth); err != nil {
			return fmt.Errorf("failed to create auth for %s: %w", sourceType, err)
		}

	}

	return nil
}

func (u *AppUsecase) GetOpenAIAPIAppInfo(ctx context.Context, kbID string) (*domain.AppInfoResp, error) {
	apiApp, err := u.repo.GetOrCreateAppByKBIDAndType(ctx, kbID, domain.AppTypeOpenAIAPI)
	if err != nil {
		return nil, err
	}
	appInfo := &domain.AppInfoResp{
		Settings: domain.AppSettingsResp{
			OpenAIAPIBotSettings: apiApp.Settings.OpenAIAPIBotSettings,
		},
	}
	return appInfo, nil
}

// filterNodesByPermissions 对节点列表进行权限过滤
func (u *AppUsecase) filterNodesByPermissions(ctx context.Context, nodes []*domain.RecommendNodeListResp, userID string) ([]*domain.RecommendNodeListResp, error) {
	shareAccess, err := u.nodeUsecase.enterprise.BuildShareAccessContext(ctx, userID)
	if err != nil {
		return nil, err
	}

	filteredNodes := make([]*domain.RecommendNodeListResp, 0)

	for i, node := range nodes {
		resolvedPermissions, resolveErr := u.nodeUsecase.enterprise.ResolveShareNodePermissions(ctx, node.ID, node.ParentID, node.Permissions)
		if resolveErr != nil {
			continue
		}
		if node.NavId != "" {
			nav, navErr := u.navRepo.GetById(ctx, node.NavId)
			if navErr != nil {
				continue
			}
			resolvedPermissions = u.nodeUsecase.enterprise.MergeNodeAndNavPermissions(resolvedPermissions, nav.Permissions)
		}
		node.Permissions = resolvedPermissions

		if u.nodeUsecase.enterprise.CanVisible(node.Permissions, shareAccess) {
			filteredNodes = append(filteredNodes, nodes[i])
		} else {
			nodes[i].Summary = ""
		}

		// 如果是文件夹类型，处理其子节点的权限
		if node.Type == domain.NodeTypeFolder {
			newFileNodes := make([]*domain.RecommendNodeListResp, 0)

			for i2 := range node.RecommendNodes {
				node.RecommendNodes[i2].Summary = ""
				resolvedChildPermissions, resolveErr := u.nodeUsecase.enterprise.ResolveShareNodePermissions(
					ctx,
					node.RecommendNodes[i2].ID,
					node.RecommendNodes[i2].ParentID,
					node.RecommendNodes[i2].Permissions,
				)
				if resolveErr != nil {
					continue
				}
				if node.RecommendNodes[i2].NavId != "" {
					nav, navErr := u.navRepo.GetById(ctx, node.RecommendNodes[i2].NavId)
					if navErr != nil {
						continue
					}
					resolvedChildPermissions = u.nodeUsecase.enterprise.MergeNodeAndNavPermissions(resolvedChildPermissions, nav.Permissions)
				}
				node.RecommendNodes[i2].Permissions = resolvedChildPermissions
				if u.nodeUsecase.enterprise.CanVisible(node.RecommendNodes[i2].Permissions, shareAccess) {
					newFileNodes = append(newFileNodes, node.RecommendNodes[i2])
				}
			}
			node.RecommendNodes = newFileNodes
		}
	}

	return filteredNodes, nil
}

func (u *AppUsecase) GetRecommendNodesByIds(ctx context.Context, kbId string, nodeIds []string, userID string) ([]*domain.RecommendNodeListResp, error) {
	nodes, err := u.nodeUsecase.GetRecommendNodeList(ctx, &domain.GetRecommendNodeListReq{
		KBID:    kbId,
		NodeIDs: nodeIds,
	})
	if err != nil {
		return nil, err
	}

	return u.filterNodesByPermissions(ctx, nodes, userID)
}

func (u *AppUsecase) GetRecommendNodesByNavIds(ctx context.Context, kbId string, navIds []string, userID string) ([]*domain.RecommendNodeListResp, error) {
	navList, err := u.navRepo.GetListByIds(ctx, kbId, navIds)
	if err != nil {
		return nil, err
	}

	// 构建 navId -> navName 的 map
	navMap := make(map[string]string)
	for _, nav := range navList {
		navMap[nav.ID] = nav.Name
	}

	allNodes, err := u.nodeUsecase.GetRecommendNodeList(ctx, &domain.GetRecommendNodeListReq{
		KBID:   kbId,
		NavIds: navIds,
	})
	if err != nil {
		return nil, err
	}

	filteredAll, err := u.filterNodesByPermissions(ctx, allNodes, userID)
	if err != nil {
		return nil, err
	}

	// 按 navId 分组，保持 navIds 的原始顺序
	nodesByNav := make(map[string][]*domain.RecommendNodeListResp, len(navIds))
	for _, node := range filteredAll {
		nodesByNav[node.NavId] = append(nodesByNav[node.NavId], node)
	}

	recommendNodes := make([]*domain.RecommendNodeListResp, 0, len(navIds))
	for _, navId := range navIds {
		recommendNodes = append(recommendNodes, &domain.RecommendNodeListResp{
			NavId:          navId,
			NavName:        navMap[navId],
			RecommendNodes: nodesByNav[navId],
		})
	}

	return recommendNodes, nil
}
