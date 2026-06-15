package domain

import (
	"time"
)

type StatPageScene int

const (
	StatPageSceneWelcome StatPageScene = iota + 1
	StatPageSceneNodeDetail
	StatPageSceneChat
	StatPageSceneLogin
)

var (
	StatPageSceneNames = []string{"欢迎页", "问答页", "登录页"}
)

type StatPage struct {
	ID          int64         `json:"id" gorm:"primaryKey;autoIncrement"`
	KBID        string        `json:"kb_id"`
	NodeID      string        `json:"node_id"`
	UserID      string        `json:"user_id"`
	SessionID   string        `json:"session_id"`
	Scene       StatPageScene `json:"scene"` // 1: welcome, 2: detail, 3: chat, 4: login
	IP          string        `json:"ip"`
	UA          string        `json:"ua"`
	BrowserName string        `json:"browser_name"`
	BrowserOS   string        `json:"browser_os"`
	Referer     string        `json:"referer"`
	RefererHost string        `json:"referer_host"`
	CreatedAt   time.Time     `json:"created_at"`
}

type StatPageReq struct {
	Scene  StatPageScene `json:"scene" validate:"required,oneof=1 2 3 4"`
	NodeID string        `json:"node_id"`
}

type HotPage struct {
	Scene    StatPageScene `json:"scene"`
	NodeID   string        `json:"node_id"`
	NodeName string        `json:"node_name" gorm:"-"`
	Count    int64         `json:"count"`
}

type HotRefererHost struct {
	RefererHost string `json:"referer_host"`
	Count       int64  `json:"count"`
}

type HotBrowser struct {
	OS      []BrowserCount `json:"os"`
	Browser []BrowserCount `json:"browser"`
}

type BrowserCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type InstantCountResp struct {
	Time  string `json:"time"`
	Count int64  `json:"count"`
}

type InstantPageResp struct {
	Scene     StatPageScene `json:"scene"`
	NodeID    string        `json:"node_id"`
	NodeName  string        `json:"node_name" gorm:"-"`
	IP        string        `json:"ip"`
	IPAddress IPAddress     `json:"ip_address" gorm:"-"`
	CreatedAt time.Time     `json:"created_at"`

	UserID string       `json:"user_id"`
	Info   *UserSummary `json:"info"`
}

type ConversationDistribution struct {
	AppType AppType `json:"app_type"`
	AppID   string  `json:"-"`
	Count   int64   `json:"count"`
}

// StatPageHour 按小时聚合的统计数据
type StatPageHour struct {
	ID                       int64       `json:"id" gorm:"primaryKey;autoIncrement"`
	KbID                     string      `json:"kb_id" gorm:"index"`
	Hour                     time.Time   `json:"hour" gorm:"index"` // 按小时截断的时间
	IPCount                  int64       `json:"ip_count"`
	SessionCount             int64       `json:"session_count"`
	PageVisitCount           int64       `json:"page_visit_count"`
	ConversationCount        int64       `json:"conversation_count"`
	GeoCount                 MapStrInt64 `json:"geo_count" gorm:"type:jsonb"`
	ConversationDistribution MapStrInt64 `json:"conversation_distribution" gorm:"type:jsonb"`
	HotRefererHost           MapStrInt64 `json:"hot_referer_host" gorm:"type:jsonb"`
	HotPage                  MapStrInt64 `json:"hot_page" gorm:"type:jsonb"`
	HotBrowser               MapStrInt64 `json:"hot_browser" gorm:"type:jsonb"`
	HotOS                    MapStrInt64 `json:"hot_os" gorm:"type:jsonb"`

	CreatedAt time.Time `json:"created_at"`
}

func (StatPageHour) TableName() string {
	return "stat_page_hours"
}

// NodeStats node表统计数据
type NodeStats struct {
	ID     int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeID string `json:"node_id" gorm:"uniqueIndex"`
	PV     int64  `json:"pv"`
}

func (NodeStats) TableName() string {
	return "node_stats"
}
