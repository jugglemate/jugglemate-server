package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	juggleimsdk "github.com/juggleim/imserver-sdk-go"
	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/imsdk"
	"github.com/juggleim/jugglemate-server/storages"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrInboxNotFound Inbox 不存在或不属于当前 AppKey。
	ErrInboxNotFound = errors.New("Inbox 不存在")
	// ErrAgentNotBindable 目标 Agent 不可绑定：不存在、未激活、AppKey 不匹配，或没有 active Bot。
	//
	// TIPS: 绑定成功后 Bot 需要加入未关闭 Ticket 群，因此没有 Bot 的 Agent（如系统内置
	// Juggle_Agent）无法绑定到 Inbox。
	ErrAgentNotBindable = errors.New("Agent 不存在、未激活、AppKey 不匹配或没有 active Bot")
)

var (
	getImSdkForInboxAgent = imsdk.GetImSdk
	addInboxAgentToGroup  = func(sdk *juggleimsdk.JuggleIMSdk, request juggleimsdk.GroupMembersReq) (juggleimsdk.ApiCode, string, error) {
		return sdk.GroupAddMembers(request)
	}
	removeInboxAgentFromGroup = func(sdk *juggleimsdk.JuggleIMSdk, request juggleimsdk.GroupMembersReq) (juggleimsdk.ApiCode, string, error) {
		return sdk.GroupDelMembers(request)
	}
	newInboxMemberStorageForInboxAgent = storages.NewInboxMemberStorage
	newUserStorageForInboxAgent        = storages.NewUserStorage
)

// InboxAgentBinding 表示 Inbox 当前生效的 Agent 与 Bot 绑定。
type InboxAgentBinding struct {
	ID        string    `gorm:"size:64;primaryKey"`
	AppKey    string    `gorm:"column:app_key;size:64;not null"`
	InboxID   string    `gorm:"column:inbox_id;size:64;not null"`
	AgentID   string    `gorm:"column:agent_id;size:64;not null"`
	BotID     string    `gorm:"column:bot_id;size:64;not null"`
	Status    string    `gorm:"size:16;not null"`
	SyncError *string   `gorm:"column:sync_error;type:text"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 返回 Inbox-Agent 绑定表名。
func (InboxAgentBinding) TableName() string { return "inbox_agent_bindings" }

// InboxAgentDetail 是控制台和建群流程使用的已解析绑定。
type InboxAgentDetail struct {
	InboxID   string
	AgentID   string
	AgentName string
	BotID     string
	BotUserID string
	BotName   string
	SyncError string
}

// GetInboxAgent 查询指定 Inbox 当前绑定的 Agent 和 Bot。
func GetInboxAgent(ctx context.Context, appKey, inboxID string) (*InboxAgentDetail, error) {
	db := dbcommons.GetDb()
	if db == nil {
		return nil, fmt.Errorf("PostgreSQL 尚未初始化")
	}
	return getInboxAgentWithDB(ctx, db, appKey, inboxID)
}

func getInboxAgentWithDB(ctx context.Context, db *gorm.DB, appKey, inboxID string) (*InboxAgentDetail, error) {
	var detail InboxAgentDetail
	err := db.WithContext(ctx).Table("inbox_agent_bindings iab").
		Select("iab.inbox_id,iab.agent_id,a.name agent_name,iab.bot_id,b.bot_user_id,b.bot_name,COALESCE(iab.sync_error,'') sync_error").
		Joins("JOIN agents a ON a.id=iab.agent_id AND a.app_key=iab.app_key").
		Joins("JOIN bots b ON b.id=iab.bot_id AND b.app_key=iab.app_key").
		Where("iab.app_key=? AND iab.inbox_id=? AND iab.status='active'", strings.TrimSpace(appKey), strings.TrimSpace(inboxID)).
		Take(&detail).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &detail, err
}

// BindInboxAgent 绑定 Inbox 与 Agent，并同步全部未关闭 Ticket 群的 Bot 成员。
//
// TIPS: 先同步群成员再落库，保证换绑失败时仍保留旧绑定作为下次重试的补偿依据。
func BindInboxAgent(ctx context.Context, appKey, inboxID, agentID string) (*InboxAgentDetail, error) {
	appKey, inboxID, agentID = strings.TrimSpace(appKey), strings.TrimSpace(inboxID), strings.TrimSpace(agentID)
	if appKey == "" || inboxID == "" || agentID == "" {
		return nil, fmt.Errorf("appKey、inboxID、agentID 不能为空")
	}
	db := dbcommons.GetDb()
	if db == nil {
		return nil, fmt.Errorf("PostgreSQL 尚未初始化")
	}
	var syncErr error
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// TIPS: 同一 Inbox 的外部群操作必须跨实例串行；事务级 advisory lock 会在提交或回滚时自动释放。
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "inbox-agent:"+appKey+":"+inboxID).Error; err != nil {
			return err
		}
		var inboxCount int64
		if err := tx.Table("inboxes").Where("app_key=? AND inbox_id=?", appKey, inboxID).Count(&inboxCount).Error; err != nil {
			return err
		}
		if inboxCount == 0 {
			return ErrInboxNotFound
		}
		// TIPS: 这里必须用 Take 而非 First。First 会按主键追加 ORDER BY，而目标结构体没有
		// 主键，GORM 会退化成按第一个字段排序，生成 `ORDER BY a.agent_id` —— agents 表只有
		// id 没有 agent_id，直接报 42703。绑定唯一（uq_bab_agent_active）本就至多一行，无需排序。
		var target struct {
			AgentID   string
			BotID     string
			BotUserID string
		}
		findErr := tx.Table("agents a").
			Select("a.id agent_id,b.id bot_id,b.bot_user_id").
			Joins("JOIN bot_agent_bindings bab ON bab.agent_id=a.id AND bab.status='active'").
			Joins("JOIN bots b ON b.id=bab.bot_id AND b.app_key=a.app_key AND b.status='active'").
			Where("a.id=? AND a.app_key=? AND a.status='active'", agentID, appKey).
			Take(&target).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: agent=%s", ErrAgentNotBindable, agentID)
		}
		if findErr != nil {
			return findErr
		}
		previous, err := getInboxAgentWithDB(ctx, tx, appKey, inboxID)
		if err != nil {
			return err
		}
		if err := syncOpenTicketAgentBot(ctx, tx, appKey, inboxID, previous, target.BotUserID); err != nil {
			syncErr = err
			if previous != nil {
				message := err.Error()
				return tx.Model(&InboxAgentBinding{}).Where("app_key=? AND inbox_id=?", appKey, inboxID).Updates(map[string]any{"sync_error": message, "updated_at": time.Now().UTC()}).Error
			}
			return nil
		}
		binding := InboxAgentBinding{ID: uuid.NewString(), AppKey: appKey, InboxID: inboxID, AgentID: target.AgentID, BotID: target.BotID, Status: "active"}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "app_key"}, {Name: "inbox_id"}},
			DoUpdates: clause.Assignments(map[string]any{"agent_id": target.AgentID, "bot_id": target.BotID, "status": "active", "sync_error": nil, "updated_at": time.Now().UTC()}),
		}).Create(&binding).Error
	})
	if err != nil {
		return nil, err
	}
	if syncErr != nil {
		return nil, syncErr
	}
	return GetInboxAgent(ctx, appKey, inboxID)
}

// UnbindInboxAgent 解除 Inbox-Agent 绑定，并从所有未关闭 Ticket 群移除旧 Bot。
func UnbindInboxAgent(ctx context.Context, appKey, inboxID string) error {
	db := dbcommons.GetDb()
	if db == nil {
		return fmt.Errorf("PostgreSQL 尚未初始化")
	}
	appKey, inboxID = strings.TrimSpace(appKey), strings.TrimSpace(inboxID)
	var syncErr error
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "inbox-agent:"+appKey+":"+inboxID).Error; err != nil {
			return err
		}
		previous, err := getInboxAgentWithDB(ctx, tx, appKey, inboxID)
		if err != nil || previous == nil {
			return err
		}
		if err := syncOpenTicketAgentBot(ctx, tx, appKey, inboxID, previous, ""); err != nil {
			syncErr = err
			message := err.Error()
			return tx.Model(&InboxAgentBinding{}).Where("app_key=? AND inbox_id=?", appKey, inboxID).Updates(map[string]any{"sync_error": message, "updated_at": time.Now().UTC()}).Error
		}
		return tx.Where("app_key=? AND inbox_id=?", appKey, inboxID).Delete(&InboxAgentBinding{}).Error
	})
	if err != nil {
		return err
	}
	return syncErr
}

// ResolveInboxAgentBot 返回建群时应加入的 Agent Bot；未绑定时返回空字符串。
func ResolveInboxAgentBot(ctx context.Context, appKey, inboxID string) (string, error) {
	detail, err := GetInboxAgent(ctx, appKey, inboxID)
	if err != nil || detail == nil {
		return "", err
	}
	return detail.BotUserID, nil
}

// botConnectionStateProbe 由 Agent 模块启动时注入，返回指定 Bot 长连接的当前状态。
//
// TIPS: services 包不持有 imbot.Manager 实例，用函数变量注入避免反向依赖 Agent 模块，
// 与本文件里 getImSdkForInboxAgent 的做法一致。未注入时诊断日志退化为 unknown，不报错。
var botConnectionStateProbe func(appKey, botUserID string) string

// SetBotConnectionStateProbe 注入 Bot 长连接状态探针，由 Agent 模块启动时调用一次。
//
// @param probe 入参为 appKey 与 botUserID，返回 connected/connecting/disconnected/no_client 等状态描述
func SetBotConnectionStateProbe(probe func(appKey, botUserID string) string) {
	botConnectionStateProbe = probe
}

// LogTicketAgentBotDiagnostics 在客户消息进入 Ticket 群时检查 Agent 回复链路，仅在异常时告警。
//
// 客户消息由 IM 直接投递给群内 Bot 的长连接，不经过本进程的 webhook 分支，因此 Bot 不回复时
// webhook 日志里看不到任何线索。这里在"客户刚发言"这个时间点校验两个必要条件：
//   - 绑定：该 Ticket 所属 Inbox 有没有 active 的 Agent-Bot 绑定，上次群成员同步有没有报错
//   - 连接：该 Bot 在本进程里长连接是否处于 connected
//
// TIPS: 一切正常时不打日志。客户消息是最高频的事件，逐条记录会淹没真正的异常；正常路径的
// 观测点是 imbot 的 "[IMBot] 收到消息" 与 "[Inbound] 回复已发送"，二者缺失即代表链路断了。
//
// @param ctx 请求上下文
// @param appKey 应用 AppKey
// @param inboxID Ticket 所属 Inbox ID
// @param ticketID Ticket 群 ID
// @param msgID 触发检查的消息 ID，用于和 webhook 日志对齐
func LogTicketAgentBotDiagnostics(ctx context.Context, appKey, inboxID, ticketID, msgID string) {
	detail, err := GetInboxAgent(ctx, appKey, inboxID)
	if err != nil {
		log.Printf("[AgentDiag] 绑定查询失败 appkey=%s inbox_id=%s ticket_id=%s msg_id=%s err=%v", appKey, inboxID, ticketID, msgID, err)
		return
	}
	if detail == nil {
		// Inbox 未绑定 Agent 是合法配置（纯人工坐席），降为一次性可查的低频信息即可。
		return
	}
	state := "unknown"
	if botConnectionStateProbe != nil {
		state = botConnectionStateProbe(appKey, detail.BotUserID)
	}
	if state == "connected" && detail.SyncError == "" {
		return
	}
	log.Printf("[AgentDiag] Bot 回复链路异常 appkey=%s inbox_id=%s ticket_id=%s msg_id=%s agent_id=%s bot_user_id=%s conn_state=%s sync_error=%q",
		appKey, inboxID, ticketID, msgID, detail.AgentID, detail.BotUserID, state, detail.SyncError)
}

// SwitchTicketToHuman 把 Ticket 群从 Agent 接待切换为人工接待。
//
// 动作顺序固定为「先加坐席 → 再发通知 → 最后移出 Bot」：
//   - 先加后删与 syncOpenTicketAgentBot 的既有约定一致，中途失败最多造成两者短暂共存，
//     不会出现群里既没有 Bot 也没有坐席的空窗；
//   - 通知在移出 Bot 之前发送，因此可以用 Bot 身份作为发送者，且此时坐席已在群内能收到。
//
// 坐席入群前会逐个补一次 IM 注册：建群时已不再注册坐席（见 prepareTicketGroupMemberIds），
// 而 IM 要求群成员必须是已存在的用户。
//
// @param ctx 请求上下文
// @param appKey 应用 AppKey
// @param ticketID 目标 Ticket 群 ID
// @param botUserID 待移出的 Agent Bot IM 身份，为空时只加坐席不做移除
// @return 任一 IM 操作失败都返回错误；调用方据此决定是否回滚人工状态
func SwitchTicketToHuman(ctx context.Context, appKey, ticketID, botUserID string) error {
	appKey, ticketID, botUserID = strings.TrimSpace(appKey), strings.TrimSpace(ticketID), strings.TrimSpace(botUserID)
	if appKey == "" || ticketID == "" {
		return fmt.Errorf("appKey、ticketID 不能为空")
	}
	db := dbcommons.GetDb()
	if db == nil {
		return fmt.Errorf("PostgreSQL 尚未初始化")
	}
	var inboxID string
	if err := db.WithContext(ctx).Table("tickets").Select("inbox_id").
		Where("app_key=? AND ticket_id=?", appKey, ticketID).Take(&inboxID).Error; err != nil {
		return fmt.Errorf("查询 Ticket 所属 Inbox 失败 ticket=%s: %w", ticketID, err)
	}
	sdk := getImSdkForInboxAgent(appKey)
	if sdk == nil {
		return fmt.Errorf("无法使用 AppKey %s 初始化 IM SDK", appKey)
	}
	seatIDs, err := resolveInboxSeatIDs(appKey, inboxID, sdk)
	if err != nil {
		return err
	}
	log.Printf("[AgentHandoff] 开始转人工 appkey=%s inbox_id=%s ticket_id=%s seats=%d bot=%s",
		appKey, inboxID, ticketID, len(seatIDs), botUserID)
	if len(seatIDs) > 0 {
		code, _, addErr := addInboxAgentToGroup(sdk, juggleimsdk.GroupMembersReq{GroupId: ticketID, MemberIds: seatIDs})
		if addErr != nil || code != juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS) {
			return ticketGroupSyncError("转人工时向 Ticket 群添加坐席失败", ticketID, code, addErr)
		}
	}
	SendTicketHumanTakeoverNtfMsg(appKey, ticketID, botUserID)
	if botUserID != "" {
		code, _, delErr := removeInboxAgentFromGroup(sdk, juggleimsdk.GroupMembersReq{GroupId: ticketID, MemberIds: []string{botUserID}})
		if delErr != nil || code != juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS) {
			return ticketGroupSyncError("转人工时从 Ticket 群移除 Agent Bot 失败", ticketID, code, delErr)
		}
	}
	log.Printf("[AgentHandoff] 转人工完成 appkey=%s ticket_id=%s seats=%d bot_removed=%t",
		appKey, ticketID, len(seatIDs), botUserID != "")
	return nil
}

// resolveInboxSeatIDs 返回 Inbox 下全部坐席的 IM 身份，并确保它们在 IM 侧已注册。
func resolveInboxSeatIDs(appKey, inboxID string, sdk *juggleimsdk.JuggleIMSdk) ([]string, error) {
	members, err := newInboxMemberStorageForInboxAgent().QryByInbox(appKey, inboxID, 0, 1000)
	if err != nil {
		return nil, fmt.Errorf("查询 Inbox 坐席失败 inbox=%s: %w", inboxID, err)
	}
	userStorage := newUserStorageForInboxAgent()
	seatIDs := make([]string, 0, len(members))
	for _, member := range members {
		userID := strings.TrimSpace(member.MemberId)
		if userID == "" {
			continue
		}
		user, findErr := userStorage.FindByUserId(appKey, userID)
		if findErr != nil {
			return nil, fmt.Errorf("查询坐席资料失败 user=%s: %w", userID, findErr)
		}
		if user == nil {
			continue
		}
		if code := registerIMUser(sdk, user.UserId, user.Nickname, user.Avator); code != errs.IMErrorCode_SUCCESS {
			return nil, fmt.Errorf("坐席 IM 注册失败 user=%s code=%d", userID, code)
		}
		seatIDs = append(seatIDs, userID)
	}
	return seatIDs, nil
}

func syncOpenTicketAgentBot(ctx context.Context, db *gorm.DB, appKey, inboxID string, previous *InboxAgentDetail, newBotUserID string) error {
	// TIPS: 这里只取 ticket_id，不要 Find 进 storageModels.Ticket。该模型的 CreatedTime/
	// UpdatedTime 是 MySQL 时代的 int64 毫秒，而 Postgres 的 tickets.created_time 是
	// TIMESTAMPTZ，整表扫描会直接报 “converting driver.Value type time.Time to a int64”。
	// 时间字段的转换只在 DAO 层（storages/dbs/ticketdao.go 的 TicketDao）做，服务层绕过 DAO
	// 直接查表就会踩到这个模型与库结构的错配。
	var ticketIDs []string
	if err := db.WithContext(ctx).Table("tickets").
		Where("app_key=? AND inbox_id=? AND status<>?", appKey, inboxID, int(storageModels.TicketStatusClosed)).
		Pluck("ticket_id", &ticketIDs).Error; err != nil {
		return err
	}
	if len(ticketIDs) == 0 {
		log.Printf("[AgentSync] 无未关闭 Ticket，跳过群成员同步 appkey=%s inbox_id=%s new_bot=%s", appKey, inboxID, newBotUserID)
		return nil
	}
	previousBot := ""
	if previous != nil {
		previousBot = previous.BotUserID
	}
	log.Printf("[AgentSync] 开始同步 Ticket 群 Bot 成员 appkey=%s inbox_id=%s open_tickets=%d new_bot=%s previous_bot=%s",
		appKey, inboxID, len(ticketIDs), newBotUserID, previousBot)
	sdk := getImSdkForInboxAgent(appKey)
	if sdk == nil {
		return fmt.Errorf("无法使用 AppKey %s 初始化 IM SDK", appKey)
	}
	for _, ticketID := range ticketIDs {
		if newBotUserID != "" {
			code, _, err := addInboxAgentToGroup(sdk, juggleimsdk.GroupMembersReq{GroupId: ticketID, MemberIds: []string{newBotUserID}})
			if err != nil || code != juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS) {
				return ticketGroupSyncError("向 Ticket 群添加 Agent Bot 失败", ticketID, code, err)
			}
		}
		// TIPS: 换绑时先加新 Bot 再移除旧 Bot，中途失败最多造成两者短暂共存，不会让群失去可用 Bot。
		if previous != nil && previous.BotUserID != "" && previous.BotUserID != newBotUserID {
			code, _, err := removeInboxAgentFromGroup(sdk, juggleimsdk.GroupMembersReq{GroupId: ticketID, MemberIds: []string{previous.BotUserID}})
			if err != nil || code != juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS) {
				return ticketGroupSyncError("从 Ticket 群移除旧 Agent Bot 失败", ticketID, code, err)
			}
		}
	}
	log.Printf("[AgentSync] Ticket 群 Bot 成员同步完成 appkey=%s inbox_id=%s open_tickets=%d new_bot=%s", appKey, inboxID, len(ticketIDs), newBotUserID)
	return nil
}

func ticketGroupSyncError(action, ticketID string, code juggleimsdk.ApiCode, err error) error {
	if err != nil {
		return fmt.Errorf("%s ticket=%s code=%d: %w", action, ticketID, code, err)
	}
	return fmt.Errorf("%s ticket=%s code=%d", action, ticketID, code)
}
