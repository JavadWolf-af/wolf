package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	tele "gopkg.in/telebot.v3"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
)

var (
	wolfPlusMenu = &tele.ReplyMarkup{}

	pvLockMu         sync.RWMutex
	pvLockSettings   = make(map[int64]bool)
	pvWhitelistCache = make(map[int64]map[int64]bool)
)

func formatTelegramUser(u *tg.User) string {
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name == "" {
		if u.Username != "" {
			return "@" + u.Username
		}
		return fmt.Sprintf("کاربر (%d)", u.ID)
	}
	return name
}

// InitPVLockDB راه‌اندازی دیتابیس و کش قفل پیوی
func InitPVLockDB() {
	if db == nil {
		return
	}
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN is_pv_lock_enabled BOOLEAN DEFAULT FALSE")
	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS pv_whitelist (
		owner_id BIGINT,
		user_id BIGINT,
		PRIMARY KEY (owner_id, user_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	rows, err := db.Query("SELECT id, is_pv_lock_enabled FROM users")
	if err == nil {
		defer rows.Close()
		pvLockMu.Lock()
		for rows.Next() {
			var uid int64
			var enabled bool
			if err := rows.Scan(&uid, &enabled); err == nil {
				pvLockSettings[uid] = enabled
			}
		}
		pvLockMu.Unlock()
	}

	wRows, wErr := db.Query("SELECT owner_id, user_id FROM pv_whitelist")
	if wErr == nil {
		defer wRows.Close()
		pvLockMu.Lock()
		for wRows.Next() {
			var oID, uID int64
			if err := wRows.Scan(&oID, &uID); err == nil {
				if _, ok := pvWhitelistCache[oID]; !ok {
					pvWhitelistCache[oID] = make(map[int64]bool)
				}
				pvWhitelistCache[oID][uID] = true
			}
		}
		pvLockMu.Unlock()
	}
}

func InitWolfPlusDB() {
	InitPVLockDB()
}

func IsPVLockEnabled(uid int64) bool {
	pvLockMu.RLock()
	defer pvLockMu.RUnlock()
	if enabled, ok := pvLockSettings[uid]; ok {
		return enabled
	}
	return false
}

func SetPVLockEnabled(uid int64, enabled bool) {
	pvLockMu.Lock()
	defer pvLockMu.Unlock()
	pvLockSettings[uid] = enabled
	if db != nil {
		_, _ = db.Exec("UPDATE users SET is_pv_lock_enabled = ? WHERE id = ?", enabled, uid)
	}
}

func IsPVWhitelisted(ownerID, targetID int64) bool {
	pvLockMu.RLock()
	defer pvLockMu.RUnlock()
	if list, exists := pvWhitelistCache[ownerID]; exists {
		return list[targetID]
	}
	return false
}

func AddPVWhitelist(ownerID, targetID int64) {
	pvLockMu.Lock()
	defer pvLockMu.Unlock()
	if _, exists := pvWhitelistCache[ownerID]; !exists {
		pvWhitelistCache[ownerID] = make(map[int64]bool)
	}
	pvWhitelistCache[ownerID][targetID] = true
	if db != nil {
		_, _ = db.Exec("INSERT IGNORE INTO pv_whitelist (owner_id, user_id) VALUES (?, ?)", ownerID, targetID)
	}
}

func RemovePVWhitelist(ownerID, targetID int64) {
	pvLockMu.Lock()
	defer pvLockMu.Unlock()
	if list, exists := pvWhitelistCache[ownerID]; exists {
		delete(list, targetID)
	}
	if db != nil {
		_, _ = db.Exec("DELETE FROM pv_whitelist WHERE owner_id = ? AND user_id = ?", ownerID, targetID)
	}
}

// ProcessPVLockIncoming فیلتر سخت‌گیرانه فقط چت خصوصی اشخاص
func ProcessPVLockIncoming(ctx context.Context, client *telegram.Client, userID int64, msg *tg.Message, e tg.Entities) bool {
	if msg.Out {
		return false
	}

	peerUser, isPV := msg.PeerID.(*tg.PeerUser)
	if !isPV {
		return false
	}

	senderID := peerUser.UserID
	if senderID == 0 || senderID == userID || senderID == 777000 || senderID == 42777 {
		return false
	}

	if u, ok := e.Users[senderID]; ok {
		if u.Bot || u.Verified || u.Support {
			return false
		}
	}

	if !IsPVLockEnabled(userID) {
		return false
	}

	if isUserFriend(userID, senderID) || IsPVWhitelisted(userID, senderID) {
		return false
	}

	inputPeer := getInputPeer(msg.PeerID, e, userID)
	if inputPeer != nil {
		go func(p tg.InputPeerClass) {
			dCtx, dCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer dCancel()
			_, _ = client.API().MessagesDeleteHistory(dCtx, &tg.MessagesDeleteHistoryRequest{
				Peer:   p,
				MaxID:  0,
				Revoke: true,
			})
		}(inputPeer)
	}
	return true
}

func ProcessPVLockCommand(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, msg *tg.Message, text string, userID int64) bool {
	if text == "قفل پیوی روشن" {
		SetPVLockEnabled(userID, true)
		if inputPeer != nil {
			go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "🔒 قفل پیوی روشن شد")
		}
		return true
	} else if text == "قفل پیوی خاموش" {
		SetPVLockEnabled(userID, false)
		if inputPeer != nil {
			go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "🔓 قفل پیوی خاموش شد")
		}
		return true
	} else if text == "بازکردن پیوی" || text == "باز کردن پیوی" {
		if pUser, ok := msg.PeerID.(*tg.PeerUser); ok {
			targetID := pUser.UserID
			AddPVWhitelist(userID, targetID)
			if inputPeer != nil {
				go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "✅ پیوی برای این کاربر باز شد")
			}
		} else {
			if inputPeer != nil {
				go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ این دستور فقط در پیوی کاربرد دارد")
			}
		}
		return true
	} else if text == "بستن پیوی" {
		if pUser, ok := msg.PeerID.(*tg.PeerUser); ok {
			targetID := pUser.UserID
			RemovePVWhitelist(userID, targetID)
			if inputPeer != nil {
				go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "❌ پیوی این کاربر مجدداً بسته شد")
			}
		}
		return true
	}
	return false
}

func buildWolfPlusDashboardText(userID int64) string {
	pvStatus := "🔴 خاموش"
	if IsPVLockEnabled(userID) {
		pvStatus = "🟢 روشن"
	}

	return fmt.Sprintf(`🐺 <b>پنل امکانات پیشرفته ولف + (Wolf Plus)</b>
➖➖➖➖➖➖➖➖➖➖
🛡 <b>وضعیت قابلیت‌های امنیتی:</b>
▫️ 🔐 <b>قفل پیوی ضد مزاحم:</b> %s
▫️ 👁‍🗨 <b>حالت روح:</b> آماده به کار
▫️ 📥 <b>دانلود مدیاهای تایمردار:</b> آماده به کار
➖➖➖➖➖➖➖➖➖➖
💡 <i>برای روشن یا خاموش کردن قفل پیوی، از دکمه زیر استفاده کنید:</i>`, pvStatus)
}

func getWolfPlusKeyboard(userID int64) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	pvText := "🔐 قفل پیوی: 🔴 خاموش (کلیک برای روشن)"
	if IsPVLockEnabled(userID) {
		pvText = "🔐 قفل پیوی: 🟢 روشن (کلیک برای خاموش)"
	}

	btnToggle := menu.Data(pvText, "toggle_wp_pvlock")
	menu.Inline(
		menu.Row(btnToggle),
	)
	return menu
}

func RegisterWolfPlusHandlers(bot *tele.Bot) {
	bot.Handle(&tele.Btn{Unique: "toggle_wp_pvlock"}, func(c tele.Context) error {
		userID := c.Sender().ID
		newStatus := !IsPVLockEnabled(userID)
		SetPVLockEnabled(userID, newStatus)

		statusMsg := "🔴 قفل پیوی خاموش شد."
		if newStatus {
			statusMsg = "🟢 قفل پیوی روشن شد و پیام‌های غریبه‌ها دوطرفه حذف خواهند شد."
		}

		_ = c.Respond(&tele.CallbackResponse{Text: statusMsg, ShowAlert: true})
		return c.Edit(buildWolfPlusDashboardText(userID), getWolfPlusKeyboard(userID), tele.ModeHTML)
	})
}

func RegisterWolfPlusDispatcher(dispatcher *tg.UpdateDispatcher, client *telegram.Client, userID int64) {}

func WolfPlusHandleIncoming(ctx context.Context, client *telegram.Client, bot *tele.Bot, userID int64, msg *tg.Message, e tg.Entities) {}

func HandleWolfPlusText(c tele.Context) bool {
	return false
}

func InitUserbotPeerCache(ctx context.Context, client *telegram.Client) {}

func StartTargetTrackerWorker(ctx context.Context, client *telegram.Client, userID int64) {}

func HandleProtectedDownloadByReply(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, msgID int, userID int64) {}

func HandleGhostMarkAsRead(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, msgID int) {}
