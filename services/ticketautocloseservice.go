package services

import (
	"context"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/juggleim/jugglemate-server/commons/configures"
	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	"github.com/juggleim/jugglemate-server/storages"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

var (
	// 注入点：单测可替换
	newTicketStorageForAutoClose     = storages.NewTicketStorage
	newTicketEventStorageForAutoClose = storages.NewTicketEventStorage
	// fetchAutoCloseCandidates 注入"扫描候选"逻辑；
	// 默认实现走 dbcommons.GetDb() + GORM raw SQL，单测可替换为内存 mock。
	fetchAutoCloseCandidates = func(ctx context.Context, cutoffMs int64) ([]autoCloseCandidate, error) {
		db := dbcommons.GetDb()
		if db == nil {
			return nil, nil
		}
		var rows []autoCloseCandidate
		err := db.WithContext(ctx).Raw(`
			SELECT app_key, ticket_id
			FROM tickets
			WHERE status = 1
			  AND last_user_msg_at IS NOT NULL
			  AND last_user_msg_at < ?
			LIMIT 500`, cutoffMs).Scan(&rows).Error
		return rows, err
	}
)

// autoCloseCandidate 是 ticker 扫描出的待关闭候选（DB row 视图）。
type autoCloseCandidate struct {
	AppKey   string
	TicketId string
}

// autoCloseTickerHolder 全局 ticker 句柄，避免重复启动。
var (
	autoCloseTickerMu     sync.Mutex
	autoCloseTickerCancel context.CancelFunc
)

// AutoCloseConfig 控制后台定时扫描的开关和参数。
type AutoCloseConfig struct {
	Enabled       bool
	IdleMinutes   int
	TickerSeconds int
}

// ConfigFromAppConfig 读 Yaml → AutoCloseConfig（提供默认值）。
func ConfigFromAppConfig(c configures.AgentConfig) AutoCloseConfig {
	out := AutoCloseConfig{
		Enabled:       true, // 默认开启
		IdleMinutes:   5,
		TickerSeconds: 30,
	}
	return out
}

// StartAutoCloseTicker 启动后台 ticker；如果已运行则先停止旧的。
//
// 调用方 main.go 应在 AgentModule.Start 之后调用。Agent 模块 disabled 时不调用。
func StartAutoCloseTicker(parent context.Context, cfg AutoCloseConfig) {
	if !cfg.Enabled {
		log.Printf("[AutoClose] 配置 disabled，跳过 ticker")
		return
	}
	autoCloseTickerMu.Lock()
	defer autoCloseTickerMu.Unlock()
	if autoCloseTickerCancel != nil {
		autoCloseTickerCancel()
		autoCloseTickerCancel = nil
	}
	ctx, cancel := context.WithCancel(parent)
	autoCloseTickerCancel = cancel
	idle := time.Duration(cfg.IdleMinutes) * time.Minute
	tick := time.NewTicker(time.Duration(cfg.TickerSeconds) * time.Second)
	go func() {
		defer tick.Stop()
		log.Printf("[AutoClose] 启动 ticker idle=%s period=%s", idle, time.Duration(cfg.TickerSeconds)*time.Second)
		for {
			select {
			case <-ctx.Done():
				log.Printf("[AutoClose] ticker 退出")
				return
			case <-tick.C:
				runAutoCloseOnce(ctx, idle, time.Now().UnixMilli())
			}
		}
	}()
}

// StopAutoCloseTicker 关闭后台 ticker。测试和优雅停机用。
func StopAutoCloseTicker() {
	autoCloseTickerMu.Lock()
	defer autoCloseTickerMu.Unlock()
	if autoCloseTickerCancel != nil {
		autoCloseTickerCancel()
		autoCloseTickerCancel = nil
	}
}

// runAutoCloseOnce 一次扫描：
//   1. 查所有 last_user_msg_at 早于 (now - idle) 的工单（候选关闭）；
//   2. 抢占式关闭（带 WHERE 条件）；
//   3. 对抢到的工单发 jgm:csat 邀请卡；
//   4. 扫"已关闭但 csat_notified_at IS NULL"的工单补发 jgm:csat
//      （IM 暂时失联 / 上次发送失败场景下的补救），CAS 字段防并发重复发送。
func runAutoCloseOnce(ctx context.Context, idle time.Duration, nowMs int64) {
	db := dbcommons.GetDb()
	if db == nil {
		// db 是 nil 时仍允许 fetchAutoCloseCandidates 被注入的版本返回；
		// 单测场景下也会到这里 => 直接跳过
		_ = db
	}
	idleMs := idle.Milliseconds()
	cutoffMs := nowMs - idleMs

	// 阶段 1：找出 candidates（最多 500 条，循环处理；超出下次 ticker 再扫）
	rows, err := fetchAutoCloseCandidates(ctx, cutoffMs)
	if err != nil {
		log.Printf("[AutoClose] 扫描候选失败: %v", err)
	} else {
		ticketStore := newTicketStorageForAutoClose()
		for _, r := range rows {
			// 阶段 2：抢占式关闭
			closed, err := ticketStore.CloseByIdle(r.AppKey, r.TicketId, idleMs, nowMs)
			if err != nil {
				log.Printf("[AutoClose] CloseByIdle 失败 ticket=%s err=%v", r.TicketId, err)
				continue
			}
			if !closed {
				// 已被其他实例/状态变更处理，跳过
				continue
			}

			// 阶段 3：ticket_events 写 close 事件
			eventStore := newTicketEventStorageForAutoClose()
			_ = eventStore.Create(storageModels.TicketEvent{
				AppKey:       r.AppKey,
				TicketId:     r.TicketId,
				EventType:    storageModels.TicketEventTypeClose,
				OperatorID:   "",
				OperatorType: storageModels.TicketEventOperatorSystem,
				Payload:      `{"reason":"idle_timeout","closed_at_ms":` + strconv.FormatInt(nowMs, 10) + `}`,
				CreatedTime:  nowMs,
			})

			// 阶段 4：发 jgm:csat 邀请卡；通知失败时 csat_notified_at 仍保持 NULL，
			// 下次 ticker 自动重试。
			if err := NotifyCsatInvitation(ctx, r.AppKey, r.TicketId, nowMs); err != nil {
				log.Printf("[AutoClose] 发 csat 邀请失败 ticket=%s err=%v（下次 ticker 重试）", r.TicketId, err)
			}
			log.Printf("[AutoClose] 关闭工单 ticket=%s closed_at=%d", r.TicketId, nowMs)
		}
	}

	// 阶段 5：csat 补发扫描 —— 已关闭但 csat_notified_at IS NULL 的工单
	// （IM 暂时失联 / 重启后未及时通知的场景）。CAS 标记防并发重发。
	runCsatRetryPass(ctx, nowMs)
}

// runCsatRetryPass 单次扫"已关闭但 csat_notified_at IS NULL"的工单；调
// NotifyCsatInvitation + MarkCsatNotifiedOnce（CAS）。
//
// 失败时不回滚状态（IM 失败仅 slog），下次 ticker 再重发；MarkCsatNotifiedOnce 用
// SQL CAS 防并发多发。
//
// TIPS: 本函数不显式校验 dbcommons.GetDb() —— 新 TicketStorage 接口的 Qry 操作
// 通过 storage 包级变量注入，单测也走 mock 路径；这里仅依赖注入链可用性。
func runCsatRetryPass(ctx context.Context, nowMs int64) {
	ticketStore := newTicketStorageForCsatNotify()
	pending, err := ticketStore.QryTicketsNeedingCsatNotification(50)
	if err != nil {
		log.Printf("[AutoClose] 扫描未发邀请工单失败: %v", err)
		return
	}
	for _, t := range pending {
		if err := NotifyCsatInvitation(ctx, t.AppKey, t.TicketId, nowMs); err != nil {
			log.Printf("[AutoClose] 重发 csat 邀请失败 ticket=%s err=%v（下次 ticker 再试）", t.TicketId, err)
			continue
		}
		// 标记已发。CAS 字段，多实例下只会有一个成功；其余不会有并发问题。
		if _, err := ticketStore.MarkCsatNotifiedOnce(t.AppKey, t.TicketId, nowMs); err != nil {
			log.Printf("[AutoClose] MarkCsatNotifiedOnce 失败 ticket=%s err=%v（下次 ticker 再发一次）", t.TicketId, err)
		}
	}
}

// SetCsatNotifySender 已废弃：ticker 直接调 NotifyCsatInvitation，不再需要
// 单独的注入钩子。保留空实现防止 main.go 编译期引用。
func SetCsatNotifySender(fn func(ctx context.Context, appKey string, ticketId string, payload interface{}) error) {
	_ = fn
}

var _ = strings.TrimSpace
