package main

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	tele "gopkg.in/telebot.v3"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
)

var (
	wolfPlusStatesMu sync.RWMutex
	wolfPlusStates   = make(map[int64]string)

	peerNamesMu sync.RWMutex
	peerNames   = make(map[int64]string)

	controllerBot *tele.Bot

	pvLockMu         sync.RWMutex
	pvLockSettings   = make(map[int64]bool)
	pvWhitelistCache = make(map[int64]map[int64]bool)
)

// منوهای کیبورد ثابت ولف +
var (
	wolfPlusMenu  = &tele.ReplyMarkup{ResizeKeyboard: true}
	antiDelMenu   = &tele.ReplyMarkup{ResizeKeyboard: true}
	editLogMenu   = &tele.ReplyMarkup{ResizeKeyboard: true}
	timerMenu     = &tele.ReplyMarkup{ResizeKeyboard: true}
	groupDelMenu  = &tele.ReplyMarkup{ResizeKeyboard: true}
	targetMenu    = &tele.ReplyMarkup{ResizeKeyboard: true}
	protectedMenu = &tele.ReplyMarkup{ResizeKeyboard: true}
	ghostMenu     = &tele.ReplyMarkup{ResizeKeyboard: true}
	notifyMenu    = &tele.ReplyMarkup{ResizeKeyboard: true}
	pvLockMenu    = &tele.ReplyMarkup{ResizeKeyboard: true}

	btnWP_AntiDel   = wolfPlusMenu.Text("🗑 ضد حذف")
	btnWP_EditLog   = wolfPlusMenu.Text("📝 ادیت لاگر")
	btnWP_Timer     = wolfPlusMenu.Text("📸 رسانه تایمردار")
	btnWP_Group     = wolfPlusMenu.Text("👥 ضد حذف گروه")
	btnWP_Target    = wolfPlusMenu.Text("🎯 ردیاب مخاطب")
	btnWP_Protected = wolfPlusMenu.Text("🔓 دانلودر ضدکپی")
	btnWP_Ghost     = wolfPlusMenu.Text("👻 حالت روح")
	btnWP_Notify    = wolfPlusMenu.Text("🔔 اعلان ربات")
	btnWP_PVLock    = wolfPlusMenu.Text("🔐 قفل پیوی")
	btnWP_Refresh   = wolfPlusMenu.Text("🔄 بروزرسانی وضعیت")
	btnWP_BackMain  = wolfPlusMenu.Text("🔙 بازگشت به منوی اصلی")

	btnAD_On   = antiDelMenu.Text("🟢 روشن کردن ضد حذف")
	btnAD_Off  = antiDelMenu.Text("🔴 خاموش کردن ضد حذف")
	btnAD_Back = antiDelMenu.Text("🔙 بازگشت به ولف +")

	btnEL_On   = editLogMenu.Text("🟢 روشن کردن ادیت لاگر")
	btnEL_Off  = editLogMenu.Text("🔴 خاموش کردن ادیت لاگر")
	btnEL_Back = editLogMenu.Text("🔙 بازگشت به ولف +")

	btnTM_On   = timerMenu.Text("🟢 روشن کردن تایمردار")
	btnTM_Off  = timerMenu.Text("🔴 خاموش کردن تایمردار")
	btnTM_Back = timerMenu.Text("🔙 بازگشت به ولف +")

	btnGD_Add   = groupDelMenu.Text("➕ افزودن گروه به ضد حذف")
	btnGD_Clear = groupDelMenu.Text("🗑 پاکسازی لیست گروه‌ها")
	btnGD_Back  = groupDelMenu.Text("🔙 بازگشت به ولف +")

	btnTG_Add          = targetMenu.Text("➕ افزودن مخاطب")
	btnTG_Delete       = targetMenu.Text("➖ حذف مخاطب")
	btnTG_List         = targetMenu.Text("📋 لیست مخاطبان")
	btnTG_Clear        = targetMenu.Text("🗑 پاکسازی لیست اهداف")
	btnTG_Back         = targetMenu.Text("🔙 بازگشت به ولف +")
	btnTG_BackToTarget = targetMenu.Text("🔙 بازگشت به ردیاب")

	btnPC_On             = protectedMenu.Text("🟢 روشن کردن دانلودر ضدکپی")
	btnPC_Off            = protectedMenu.Text("🔴 خاموش کردن دانلودر ضدکپی")
	btnPC_Add            = protectedMenu.Text("➕ افزودن کانال/گروه")
	btnPC_Delete         = protectedMenu.Text("➖ حذف کانال/گروه")
	btnPC_List           = protectedMenu.Text("📋 لیست کانال‌های قفل")
	btnPC_Clear          = protectedMenu.Text("🗑 پاکسازی لیست")
	btnPC_Back           = protectedMenu.Text("🔙 بازگشت به ولف +")
	btnPC_BackToProtMenu = protectedMenu.Text("🔙 بازگشت به منوی ضدکپی")

	btnGH_On   = ghostMenu.Text("🟢 روشن کردن حالت روح")
	btnGH_Off  = ghostMenu.Text("🔴 خاموش کردن حالت روح")
	btnGH_Back = ghostMenu.Text("🔙 بازگشت به ولف +")

	btnNT_On   = notifyMenu.Text("🟢 روشن کردن اعلان‌ها")
	btnNT_Off  = notifyMenu.Text("🔴 خاموش کردن اعلان‌ها")
	btnNT_Back = notifyMenu.Text("🔙 بازگشت به ولف +")

	btnPV_On   = pvLockMenu.Text("🟢 روشن کردن قفل پیوی")
	btnPV_Off  = pvLockMenu.Text("🔴 خاموش کردن قفل پیوی")
	btnPV_Back = pvLockMenu.Text("🔙 بازگشت به ولف +")
)

func init() {
	wolfPlusMenu.Reply(
		wolfPlusMenu.Row(btnWP_AntiDel, btnWP_EditLog),
		wolfPlusMenu.Row(btnWP_Timer, btnWP_Group),
		wolfPlusMenu.Row(btnWP_Target, btnWP_Protected),
		wolfPlusMenu.Row(btnWP_Ghost, btnWP_Notify),
		wolfPlusMenu.Row(btnWP_PVLock, btnWP_Refresh),
		wolfPlusMenu.Row(btnWP_BackMain),
	)

	antiDelMenu.Reply(
		antiDelMenu.Row(btnAD_On, btnAD_Off),
		antiDelMenu.Row(btnAD_Back),
	)

	editLogMenu.Reply(
		editLogMenu.Row(btnEL_On, btnEL_Off),
		editLogMenu.Row(btnEL_Back),
	)

	timerMenu.Reply(
		timerMenu.Row(btnTM_On, btnTM_Off),
		timerMenu.Row(btnTM_Back),
	)

	groupDelMenu.Reply(
		groupDelMenu.Row(btnGD_Add, btnGD_Clear),
		groupDelMenu.Row(btnGD_Back),
	)

	targetMenu.Reply(
		targetMenu.Row(btnTG_Add, btnTG_Delete),
		targetMenu.Row(btnTG_List, btnTG_Clear),
		targetMenu.Row(btnTG_Back),
	)

	protectedMenu.Reply(
		protectedMenu.Row(btnPC_On, btnPC_Off),
		protectedMenu.Row(btnPC_Add, btnPC_Delete),
		protectedMenu.Row(btnPC_List, btnPC_Clear),
		protectedMenu.Row(btnPC_Back),
	)

	ghostMenu.Reply(
		ghostMenu.Row(btnGH_On, btnGH_Off),
		ghostMenu.Row(btnGH_Back),
	)

	notifyMenu.Reply(
		notifyMenu.Row(btnNT_On, btnNT_Off),
		notifyMenu.Row(btnNT_Back),
	)

	pvLockMenu.Reply(
		pvLockMenu.Row(btnPV_On, btnPV_Off),
		pvLockMenu.Row(btnPV_Back),
	)
}

func getTehranCurrentTime() string {
	return time.Now().In(getTehranLocation()).Format("15:04:05")
}

func formatTelegramUser(u *tg.User) string {
	if u == nil {
		return ""
	}
	fullName := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if fullName != "" && u.Username != "" {
		return fmt.Sprintf("%s (@%s)", fullName, u.Username)
	} else if fullName != "" {
		return fullName
	} else if u.Username != "" {
		return "@" + u.Username
	}
	return ""
}

func PopulatePeerCache(users []tg.UserClass) {
	peerNamesMu.Lock()
	defer peerNamesMu.Unlock()
	for _, uClass := range users {
		if u, ok := uClass.(*tg.User); ok {
			name := formatTelegramUser(u)
			if name != "" {
				peerNames[u.ID] = name
			}
		}
	}
}

func InitUserbotPeerCache(ctx context.Context, client *telegram.Client) {
	res, err := client.API().MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      100,
	})
	if err == nil {
		switch d := res.(type) {
		case *tg.MessagesDialogs:
			PopulatePeerCache(d.Users)
		case *tg.MessagesDialogsSlice:
			PopulatePeerCache(d.Users)
		}
	}
}

func InitWolfPlusDB() {
	if db == nil {
		return
	}

	_, _ = db.Exec("ALTER TABLE users ADD COLUMN is_anti_delete_enabled BOOLEAN DEFAULT FALSE")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN is_edit_logger_enabled BOOLEAN DEFAULT FALSE")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN is_timer_media_enabled BOOLEAN DEFAULT FALSE")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN is_protected_saver_enabled BOOLEAN DEFAULT FALSE")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN is_ghost_mode_enabled BOOLEAN DEFAULT FALSE")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN is_bot_notify_enabled BOOLEAN DEFAULT TRUE")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN is_pv_lock_enabled BOOLEAN DEFAULT FALSE")

	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS wolf_message_cache (
		owner_id BIGINT,
		chat_id BIGINT,
		message_id INT,
		sender_id BIGINT,
		sender_name VARCHAR(255),
		chat_name VARCHAR(255) DEFAULT 'چت خصوصی',
		message_text TEXT,
		media_type VARCHAR(50) DEFAULT 'متن',
		cached_file_path VARCHAR(500) DEFAULT '',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (owner_id, chat_id, message_id),
		KEY idx_owner_msg (owner_id, message_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS wolf_antidel_groups (
		owner_id BIGINT,
		chat_id BIGINT,
		chat_title VARCHAR(255),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (owner_id, chat_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS wolf_targets (
		owner_id BIGINT,
		target_id BIGINT,
		access_hash BIGINT DEFAULT 0,
		first_name VARCHAR(255) DEFAULT '',
		last_name VARCHAR(255) DEFAULT '',
		username VARCHAR(255) DEFAULT '',
		bio VARCHAR(255) DEFAULT '',
		photo_id BIGINT DEFAULT 0,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		PRIMARY KEY (owner_id, target_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS wolf_protected_channels (
		owner_id BIGINT,
		chat_id BIGINT,
		chat_title VARCHAR(255),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (owner_id, chat_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS pv_whitelist (
		owner_id BIGINT,
		user_id BIGINT,
		PRIMARY KEY (owner_id, user_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	// بارگذاری وضعیت قفل پیوی
	rows, err := db.Query("SELECT id, is_pv_lock_enabled FROM users")
	if err == nil {
		pvLockMu.Lock()
		for rows.Next() {
			var uid int64
			var enabled bool
			if err := rows.Scan(&uid, &enabled); err == nil {
				pvLockSettings[uid] = enabled
			}
		}
		pvLockMu.Unlock()
		rows.Close()
	}

	wRows, wErr := db.Query("SELECT owner_id, user_id FROM pv_whitelist")
	if wErr == nil {
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
		wRows.Close()
	}

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		for range ticker.C {
			if db != nil {
				rows, err := db.Query("SELECT cached_file_path FROM wolf_message_cache WHERE created_at < NOW() - INTERVAL 1 DAY AND cached_file_path != ''")
				if err == nil {
					for rows.Next() {
						var path string
						if err := rows.Scan(&path); err == nil && path != "" {
							_ = os.Remove(path)
						}
					}
					rows.Close()
				}
				_, _ = db.Exec("DELETE FROM wolf_message_cache WHERE created_at < NOW() - INTERVAL 1 DAY")
			}
		}
	}()
}

func InitPVLockDB() {
	InitWolfPlusDB()
}

// ==========================================
// توابع کنترل قفل پیوی (PV Lock)
// ==========================================

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

// ارسال اعلان فوری در ربات تلگرام
func sendBotAlert(userID int64, eventType, shortDetail string) {
	if controllerBot == nil {
		return
	}
	var isNotifyEnabled bool
	err := db.QueryRow("SELECT is_bot_notify_enabled FROM users WHERE id = ?", userID).Scan(&isNotifyEnabled)
	if err != nil || !isNotifyEnabled {
		return
	}

	text := fmt.Sprintf(
		"🔔 <b>اعلان جدید ولف +</b>\n"+
			"━━━━━━━━━━━━━━━━━\n"+
			"📌 <b>رویداد :</b> %s\n"+
			"⏰ <b>زمان ثبت :</b> <code>%s</code>\n"+
			"📋 <b>خلاصه :</b> %s\n"+
			"━━━━━━━━━━━━━━━━━\n"+
			"📂 <i>گزارش و فایل کامل در <b>Saved Messages (پیام‌های ذخیره‌شده)</b> شما قرار گرفت.</i>",
		eventType, getTehranCurrentTime(), shortDetail,
	)

	_, _ = controllerBot.Send(&tele.User{ID: userID}, text, tele.ModeHTML)
}

func getWolfPlusStatus(userID int64) (antiDelete, editLogger, timerMedia, protectedSaver, ghostMode, botNotify, pvLock bool, groupCount, targetCount, protectedCount int) {
	_ = db.QueryRow(`
		SELECT is_anti_delete_enabled, is_edit_logger_enabled, is_timer_media_enabled, is_protected_saver_enabled, is_ghost_mode_enabled, is_bot_notify_enabled, is_pv_lock_enabled 
		FROM users WHERE id = ?
	`, userID).Scan(&antiDelete, &editLogger, &timerMedia, &protectedSaver, &ghostMode, &botNotify, &pvLock)

	_ = db.QueryRow("SELECT COUNT(*) FROM wolf_antidel_groups WHERE owner_id = ?", userID).Scan(&groupCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM wolf_targets WHERE owner_id = ?", userID).Scan(&targetCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM wolf_protected_channels WHERE owner_id = ?", userID).Scan(&protectedCount)
	return
}

func buildWolfPlusDashboardText(userID int64) string {
	antiDel, editLog, timerMed, protSaver, ghostMode, botNotify, pvLock, groupCount, targetCount, protCount := getWolfPlusStatus(userID)

	statusIcon := func(b bool) string {
		if b {
			return "🟢 روشن"
		}
		return "🔴 خاموش"
	}

	return fmt.Sprintf(`🐺 <b>پنل امکانات پیشرفته | ولف + (Wolf+)</b>
➖➖➖➖➖➖➖➖➖➖
📊 <b>وضعیت لحظه‌ای امکانات:</b>
▫️ 🗑 <b>ضد حذف پیوی (Anti-Delete):</b> %s
▫️ 📝 <b>لاگر ادیت پیوی (Edit Logger):</b> %s
▫️ 📸 <b>رسانه تایمردار (View-Once):</b> %s
▫️ 👥 <b>ضد حذف گروه:</b> <code>%d گروه</code>
▫️ 🎯 <b>ردیاب مخاطب خاص:</b> <code>%d هدف</code>
▫️ 🔓 <b>دانلودر ضدکپی:</b> %s (<code>%d منبع</code>)
▫️ 👻 <b>حالت روح هوشمند:</b> %s
▫️ 🔔 <b>اعلان‌های ربات:</b> %s
▫️ 🔐 <b>قفل پیوی ضد مزاحم:</b> %s
➖➖➖➖➖➖➖➖➖➖
💡 <i>برای ورود به تنظیمات و راهنمای هر قابلیت، گزینه مورد نظر را از کیبورد ثابت زیر لمس کنید:</i>`,
		statusIcon(antiDel), statusIcon(editLog), statusIcon(timerMed), groupCount, targetCount,
		statusIcon(protSaver), protCount, statusIcon(ghostMode), statusIcon(botNotify), statusIcon(pvLock),
	)
}

func getMonitoredGroupsText(ownerID int64) string {
	rows, err := db.Query("SELECT chat_id, chat_title FROM wolf_antidel_groups WHERE owner_id = ?", ownerID)
	if err != nil {
		return "<i>خطا در دریافت لیست گروه‌ها.</i>"
	}
	defer rows.Close()

	var list []string
	idx := 1
	for rows.Next() {
		var cid int64
		var title string
		if err := rows.Scan(&cid, &title); err == nil {
			list = append(list, fmt.Sprintf("%d. <b>%s</b> (<code>%d</code>)", idx, title, cid))
			idx++
		}
	}

	if len(list) == 0 {
		return "⚠️ <i>در حال حاضر هیچ گروهی ثبت نشده است.</i>"
	}
	return "📋 <b>گروه‌های مانیتور شده فعلی:</b>\n" + strings.Join(list, "\n")
}

func getMonitoredTargetsText(ownerID int64) string {
	rows, err := db.Query("SELECT target_id, first_name, last_name, username FROM wolf_targets WHERE owner_id = ?", ownerID)
	if err != nil {
		return "<i>خطا در خواندن لیست اهداف.</i>"
	}
	defer rows.Close()

	var list []string
	idx := 1
	for rows.Next() {
		var tid int64
		var fn, ln, un string
		if err := rows.Scan(&tid, &fn, &ln, &un); err == nil {
			name := strings.TrimSpace(fn + " " + ln)
			if un != "" {
				name += fmt.Sprintf(" (@%s)", un)
			}
			list = append(list, fmt.Sprintf("%d. <b>%s</b> (<code>%d</code>)", idx, name, tid))
			idx++
		}
	}

	if len(list) == 0 {
		return "⚠️ <i>در حال حاضر هیچ هدفی برای ردیابی ثبت نشده است.</i>"
	}
	return "📋 <b>لیست اهداف تحت نظر:</b>\n" + strings.Join(list, "\n")
}

func getProtectedChannelsText(ownerID int64) string {
	rows, err := db.Query("SELECT chat_id, chat_title FROM wolf_protected_channels WHERE owner_id = ?", ownerID)
	if err != nil {
		return "<i>خطا در خواندن لیست منابع.</i>"
	}
	defer rows.Close()

	var list []string
	idx := 1
	for rows.Next() {
		var cid int64
		var title string
		if err := rows.Scan(&cid, &title); err == nil {
			list = append(list, fmt.Sprintf("%d. <b>%s</b> (<code>%d</code>)", idx, title, cid))
			idx++
		}
	}

	if len(list) == 0 {
		return "⚠️ <i>در حال حاضر هیچ کانال یا گروهی برای دانلود خودکار ثبت نشده است.</i>"
	}
	return "📋 <b>منابع محافظت‌شده ثبت‌شده:</b>\n" + strings.Join(list, "\n")
}

func buildTargetDeleteKeyboard(ownerID int64) (*tele.ReplyMarkup, int) {
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}
	rows, err := db.Query("SELECT target_id, first_name, last_name, username FROM wolf_targets WHERE owner_id = ?", ownerID)
	if err != nil {
		menu.Reply(menu.Row(btnTG_BackToTarget))
		return menu, 0
	}
	defer rows.Close()

	var btnRows []tele.Row
	count := 0
	for rows.Next() {
		var tid int64
		var fn, ln, un string
		if err := rows.Scan(&tid, &fn, &ln, &un); err == nil {
			name := strings.TrimSpace(fn + " " + ln)
			if name == "" {
				if un != "" {
					name = "@" + un
				} else {
					name = "کاربر"
				}
			}
			btn := menu.Text(fmt.Sprintf("❌ %s (%d)", name, tid))
			btnRows = append(btnRows, menu.Row(btn))
			count++
		}
	}

	btnRows = append(btnRows, menu.Row(btnTG_BackToTarget))
	menu.Reply(btnRows...)
	return menu, count
}

func buildProtectedDeleteKeyboard(ownerID int64) (*tele.ReplyMarkup, int) {
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}
	rows, err := db.Query("SELECT chat_id, chat_title FROM wolf_protected_channels WHERE owner_id = ?", ownerID)
	if err != nil {
		menu.Reply(menu.Row(btnPC_BackToProtMenu))
		return menu, 0
	}
	defer rows.Close()

	var btnRows []tele.Row
	count := 0
	for rows.Next() {
		var cid int64
		var title string
		if err := rows.Scan(&cid, &title); err == nil {
			if title == "" {
				title = "کانال/گروه"
			}
			btn := menu.Text(fmt.Sprintf("❌ %s (%d)", title, cid))
			btnRows = append(btnRows, menu.Row(btn))
			count++
		}
	}

	btnRows = append(btnRows, menu.Row(btnPC_BackToProtMenu))
	menu.Reply(btnRows...)
	return menu, count
}

func RegisterWolfPlusHandlers(bot *tele.Bot) {
	controllerBot = bot

	showDashboard := func(c tele.Context) error {
		userID := c.Sender().ID
		if IsUserBlocked(userID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}
		selfStatus := GetUserSelfStatus(userID)
		if selfStatus == "خرید نداشته" || selfStatus == "خروج" {
			return c.Send("❌ <b>دسترسی محدود!</b>\n\nامکانات ویژه ولف + فقط برای کاربرانی که اشتراک سلف را فعال دارند در دسترس است.", getMainKeyboard(userID), tele.ModeHTML)
		}

		wolfPlusStatesMu.Lock()
		delete(wolfPlusStates, userID)
		wolfPlusStatesMu.Unlock()

		return c.Send(buildWolfPlusDashboardText(userID), wolfPlusMenu, tele.ModeHTML)
	}

	bot.Handle(&btnWP_Refresh, showDashboard)
	bot.Handle(&btnAD_Back, showDashboard)
	bot.Handle(&btnEL_Back, showDashboard)
	bot.Handle(&btnTM_Back, showDashboard)
	bot.Handle(&btnGD_Back, showDashboard)
	bot.Handle(&btnTG_Back, showDashboard)
	bot.Handle(&btnPC_Back, showDashboard)
	bot.Handle(&btnGH_Back, showDashboard)
	bot.Handle(&btnNT_Back, showDashboard)
	bot.Handle(&btnPV_Back, showDashboard)

	bot.Handle(&btnWP_BackMain, func(c tele.Context) error {
		userID := c.Sender().ID
		wolfPlusStatesMu.Lock()
		delete(wolfPlusStates, userID)
		wolfPlusStatesMu.Unlock()
		return c.Send("🔙 <b>به منوی اصلی بازگشتید.</b>", getMainKeyboard(userID), tele.ModeHTML)
	})

	// ۱. ضد حذف
	bot.Handle(&btnWP_AntiDel, func(c tele.Context) error {
		userID := c.Sender().ID
		antiDel, _, _, _, _, _, _, _, _, _ := getWolfPlusStatus(userID)
		statusStr := "🔴 خاموش"
		if antiDel {
			statusStr = "🟢 روشن"
		}

		text := fmt.Sprintf(`🗑 <b>مدیریت ضد حذف پیام‌ها (Anti-Delete)</b>
➖➖➖➖➖➖➖➖➖➖
📌 <b>وضعیت فعلی شما:</b> %s
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
پیام‌های متنی و مدیاهای دریافتی در چت‌های خصوصی ذخیره شده و در صورت حذف شدن، به صورت تفکیک‌شده به <b>Saved Messages</b> ارسال می‌شوند.`, statusStr)
		return c.Send(text, antiDelMenu, tele.ModeHTML)
	})

	bot.Handle(&btnAD_On, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("UPDATE users SET is_anti_delete_enabled = TRUE WHERE id = ?", userID)
		return c.Send("🟢 <b>قابلیت ضد حذف پیوی روشن شد.</b>", antiDelMenu, tele.ModeHTML)
	})

	bot.Handle(&btnAD_Off, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("UPDATE users SET is_anti_delete_enabled = FALSE WHERE id = ?", userID)
		return c.Send("🔴 <b>قابلیت ضد حذف خاموش شد.</b>", antiDelMenu, tele.ModeHTML)
	})

	// ۲. ادیت لاگر
	bot.Handle(&btnWP_EditLog, func(c tele.Context) error {
		userID := c.Sender().ID
		_, editLog, _, _, _, _, _, _, _, _ := getWolfPlusStatus(userID)
		statusStr := "🔴 خاموش"
		if editLog {
			statusStr = "🟢 روشن"
		}

		text := fmt.Sprintf(`📝 <b>مدیریت لاگر ویرایش (Edit Logger)</b>
➖➖➖➖➖➖➖➖➖➖
📌 <b>وضعیت فعلی شما:</b> %s
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
هرگونه تغییر در متن پیام‌های خصوصی استخراج شده و به همراه نسخه قبل از ادیت در <b>Saved Messages</b> ثبت می‌گردد.`, statusStr)
		return c.Send(text, editLogMenu, tele.ModeHTML)
	})

	bot.Handle(&btnEL_On, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("UPDATE users SET is_edit_logger_enabled = TRUE WHERE id = ?", userID)
		return c.Send("🟢 <b>قابلیت ادیت لاگر روشن شد.</b>", editLogMenu, tele.ModeHTML)
	})

	bot.Handle(&btnEL_Off, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("UPDATE users SET is_edit_logger_enabled = FALSE WHERE id = ?", userID)
		return c.Send("🔴 <b>قابلیت ادیت لاگر خاموش شد.</b>", editLogMenu, tele.ModeHTML)
	})

	// ۳. رسانه تایمردار
	bot.Handle(&btnWP_Timer, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _, timerMed, _, _, _, _, _, _, _ := getWolfPlusStatus(userID)
		statusStr := "🔴 خاموش"
		if timerMed {
			statusStr = "🟢 روشن"
		}

		text := fmt.Sprintf(`📸 <b>مدیریت رسانه های تایمردار (View-Once)</b>
➖➖➖➖➖➖➖➖➖➖
📌 <b>وضعیت فعلی شما:</b> %s
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
عکس‌ها و ویدیوهای تایمردار قبل از سوختن دانلود شده و مستقیماً به <b>Saved Messages</b> شما تحویل داده می‌شوند.`, statusStr)
		return c.Send(text, timerMenu, tele.ModeHTML)
	})

	bot.Handle(&btnTM_On, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("UPDATE users SET is_timer_media_enabled = TRUE WHERE id = ?", userID)
		return c.Send("🟢 <b>قابلیت ذخیره رسانه تایمردار روشن شد.</b>", timerMenu, tele.ModeHTML)
	})

	bot.Handle(&btnTM_Off, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("UPDATE users SET is_timer_media_enabled = FALSE WHERE id = ?", userID)
		return c.Send("🔴 <b>قابلیت ذخیره رسانه تایمردار خاموش شد.</b>", timerMenu, tele.ModeHTML)
	})

	// ۴. ضد حذف گروه
	bot.Handle(&btnWP_Group, func(c tele.Context) error {
		userID := c.Sender().ID
		groupsList := getMonitoredGroupsText(userID)

		text := fmt.Sprintf(`👥 <b>مدیریت ضد حذف گروه‌ها (Group Anti-Delete)</b>
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
برای مانیتورینگ گروه‌های خاص، آیدی یا یوزرنیم گروه را در این بخش ثبت کنید.
➖➖➖➖➖➖➖➖➖➖
%s`, groupsList)
		return c.Send(text, groupDelMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGD_Add, func(c tele.Context) error {
		userID := c.Sender().ID
		wolfPlusStatesMu.Lock()
		wolfPlusStates[userID] = "waiting_for_group_input"
		wolfPlusStatesMu.Unlock()

		return c.Send("➕ <b>افزودن گروه به لیست ضد حذف:</b>\n\nلطفاً <b>آیدی عددی</b> یا <b>یوزرنیم گروه</b> را ارسال کنید:", groupDelMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGD_Clear, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("DELETE FROM wolf_antidel_groups WHERE owner_id = ?", userID)
		return c.Send("🗑 <b>تمام گروه‌ها از لیست ضد حذف حذف شدند.</b>", groupDelMenu, tele.ModeHTML)
	})

	// ۵. ردیاب مخاطب خاص
	showTargetMenu := func(c tele.Context) error {
		userID := c.Sender().ID
		wolfPlusStatesMu.Lock()
		delete(wolfPlusStates, userID)
		wolfPlusStatesMu.Unlock()

		_, _, _, _, _, _, _, _, targetCount, _ := getWolfPlusStatus(userID)

		text := fmt.Sprintf(`🎯 <b>مدیریت ردیاب مخاطب خاص (Target Tracker)</b>
➖➖➖➖➖➖➖➖➖➖
📊 <b>تعداد اهداف تحت نظر:</b> <code>%d نفر</code>
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
بررسی لحظه‌ای و نامحسوس پروفایل شخص مورد نظر شما (تغییر نام، یوزرنیم، بیوگرافی و عکس پروفایل).`, targetCount)

		return c.Send(text, targetMenu, tele.ModeHTML)
	}

	bot.Handle(&btnWP_Target, showTargetMenu)
	bot.Handle(&btnTG_BackToTarget, showTargetMenu)

	bot.Handle(&btnTG_Add, func(c tele.Context) error {
		userID := c.Sender().ID
		wolfPlusStatesMu.Lock()
		wolfPlusStates[userID] = "waiting_for_target_input"
		wolfPlusStatesMu.Unlock()

		return c.Send("🎯 <b>ثبت مخاطب هدف:</b>\n\nلطفاً <b>یوزرنیم</b> یا <b>آیدی عددی</b> مخاطب هدف را ارسال کنید:", targetMenu, tele.ModeHTML)
	})

	bot.Handle(&btnTG_Delete, func(c tele.Context) error {
		userID := c.Sender().ID
		delMenu, count := buildTargetDeleteKeyboard(userID)
		if count == 0 {
			return c.Send("⚠️ <b>لیست اهداف خالی است!</b> مخاطبی برای حذف وجود ندارد.", targetMenu, tele.ModeHTML)
		}

		wolfPlusStatesMu.Lock()
		wolfPlusStates[userID] = "waiting_for_target_delete"
		wolfPlusStatesMu.Unlock()

		return c.Send("🎯 <b>حذف مخاطب از ردیاب:</b>\n\nروی نام مخاطب در کیبورد پایین کلیک کنید:", delMenu, tele.ModeHTML)
	})

	bot.Handle(&btnTG_List, func(c tele.Context) error {
		userID := c.Sender().ID
		return c.Send(getMonitoredTargetsText(userID), targetMenu, tele.ModeHTML)
	})

	bot.Handle(&btnTG_Clear, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("DELETE FROM wolf_targets WHERE owner_id = ?", userID)
		return c.Send("🗑 <b>لیست اهداف ردیاب به طور کامل پاکسازی شد.</b>", targetMenu, tele.ModeHTML)
	})

	// ۶. دانلودر محتوای قفل‌شده و ضد کپی
	showProtectedMenu := func(c tele.Context) error {
		userID := c.Sender().ID
		wolfPlusStatesMu.Lock()
		delete(wolfPlusStates, userID)
		wolfPlusStatesMu.Unlock()

		_, _, _, protSaver, _, _, _, _, _, protCount := getWolfPlusStatus(userID)
		statusStr := "🔴 خاموش"
		if protSaver {
			statusStr = "🟢 روشن"
		}

		text := fmt.Sprintf(`🔓 <b>دانلودر محتوای قفل‌شده و ضد کپی (Protected Content)</b>
➖➖➖➖➖➖➖➖➖➖
📌 <b>وضعیت فعلی:</b> %s
📊 <b>کانال‌ها و گروه‌های ثبت‌شده:</b> <code>%d منبع</code>
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
۱. کانال‌های مورد نظر را اضافه کنید تا پست‌های قفل‌شده خودکار به <b>Saved Messages</b> شما ارسال شوند.
۲. در هر کانال یا گروه قفل‌شده‌ای، روی پیام مورد نظر ریپلای کنید و بفرستید: <code>دانلود</code> یا <code>سیو</code>`, statusStr, protCount)

		return c.Send(text, protectedMenu, tele.ModeHTML)
	}

	bot.Handle(&btnWP_Protected, showProtectedMenu)
	bot.Handle(&btnPC_BackToProtMenu, showProtectedMenu)

	bot.Handle(&btnPC_On, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("UPDATE users SET is_protected_saver_enabled = TRUE WHERE id = ?", userID)
		return c.Send("🟢 <b>دانلودر محتوای قفل‌شده با موفقیت روشن شد.</b>", protectedMenu, tele.ModeHTML)
	})

	bot.Handle(&btnPC_Off, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("UPDATE users SET is_protected_saver_enabled = FALSE WHERE id = ?", userID)
		return c.Send("🔴 <b>دانلودر محتوای قفل‌شده خاموش شد.</b>", protectedMenu, tele.ModeHTML)
	})

	bot.Handle(&btnPC_Add, func(c tele.Context) error {
		userID := c.Sender().ID
		wolfPlusStatesMu.Lock()
		wolfPlusStates[userID] = "waiting_for_protected_input"
		wolfPlusStatesMu.Unlock()

		return c.Send("🔓 <b>افزودن کانال یا گروه ضدکپی:</b>\n\nلطفاً <b>یوزرنیم کانال/گروه</b> (مثلاً <code>@channel</code>) یا <b>آیدی عددی</b> آن را ارسال کنید:", protectedMenu, tele.ModeHTML)
	})

	bot.Handle(&btnPC_Delete, func(c tele.Context) error {
		userID := c.Sender().ID
		delMenu, count := buildProtectedDeleteKeyboard(userID)
		if count == 0 {
			return c.Send("⚠️ <b>لیست منابع خالی است!</b> منبعی برای حذف وجود ندارد.", protectedMenu, tele.ModeHTML)
		}

		wolfPlusStatesMu.Lock()
		wolfPlusStates[userID] = "waiting_for_protected_delete"
		wolfPlusStatesMu.Unlock()

		return c.Send("🔓 <b>حذف منبع از دانلودر قفل‌شده:</b>\n\nروی نام مورد نظر در کیبورد پایین کلیک کنید:", delMenu, tele.ModeHTML)
	})

	bot.Handle(&btnPC_List, func(c tele.Context) error {
		userID := c.Sender().ID
		return c.Send(getProtectedChannelsText(userID), protectedMenu, tele.ModeHTML)
	})

	bot.Handle(&btnPC_Clear, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("DELETE FROM wolf_protected_channels WHERE owner_id = ?", userID)
		return c.Send("🗑 <b>تمام منابع از لیست دانلودر قفل‌شده حذف شدند.</b>", protectedMenu, tele.ModeHTML)
	})

	// ۷. حالت روح (Ghost Mode)
	bot.Handle(&btnWP_Ghost, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _, _, _, ghostMode, _, _, _, _, _ := getWolfPlusStatus(userID)
		statusStr := "🔴 خاموش"
		if ghostMode {
			statusStr = "🟢 روشن"
		}

		text := fmt.Sprintf(`👻 <b>مدیریت حالت روح واقعی (True Ghost Mode)</b>
➖➖➖➖➖➖➖➖➖➖
📌 <b>وضعیت فعلی:</b> %s
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
با روشن کردن این قابلیت:
۱. کلیه پیام‌های دریافتی پیوی بلافاصله در <b>Saved Messages</b> کپی می‌شوند تا بدون باز کردن چت طرف مقابل، پیام را مخفیانه بخوانید و تیک دوم نخورد!
۲. هر زمان خواستید پیام‌های چتی به صورت دستی خوانده شوند، در همان چت ارسال کنید:
▫️ <code>سین</code> یا <code>سین بزن</code>`, statusStr)

		return c.Send(text, ghostMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGH_On, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("UPDATE users SET is_ghost_mode_enabled = TRUE WHERE id = ?", userID)
		return c.Send("🟢 <b>حالت روح فعال شد! پیام‌های پیوی برای خوانش مخفی در Saved Messages ارسال خواهند شد.</b>", ghostMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGH_Off, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("UPDATE users SET is_ghost_mode_enabled = FALSE WHERE id = ?", userID)
		return c.Send("🔴 <b>حالت روح غیرفعال شد.</b>", ghostMenu, tele.ModeHTML)
	})

	// ۸. اعلان‌های ربات (Bot Notifications)
	bot.Handle(&btnWP_Notify, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _, _, _, _, botNotify, _, _, _, _ := getWolfPlusStatus(userID)
		statusStr := "🔴 خاموش"
		if botNotify {
			statusStr = "🟢 روشن"
		}

		text := fmt.Sprintf(`🔔 <b>سیستم اعلان‌های ربات (Bot Alerts)</b>
➖➖➖➖➖➖➖➖➖➖
📌 <b>وضعیت فعلی:</b> %s
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
تلگرام برای پیام‌های Saved Messages صدای زنگ و پوش‌نوتیفیکیشن نمی‌فرستد.
با فعال‌سازی این بخش، هر زمان اتفاقی (ضد حذف، ادیت، عکس تایمردار، ردیاب، دانلود ضدکپی یا پیام حالت روح) رخ دهد، <b>ربات مدیریت با زنگ هشدار</b> یک اعلان فوری برای شما ارسال می‌کند.`, statusStr)

		return c.Send(text, notifyMenu, tele.ModeHTML)
	})

	bot.Handle(&btnNT_On, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("UPDATE users SET is_bot_notify_enabled = TRUE WHERE id = ?", userID)
		return c.Send("🟢 <b>اعلان‌های ربات با موفقیت فعال شد.</b>", notifyMenu, tele.ModeHTML)
	})

	bot.Handle(&btnNT_Off, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("UPDATE users SET is_bot_notify_enabled = FALSE WHERE id = ?", userID)
		return c.Send("🔴 <b>اعلان‌های ربات خاموش شد.</b>", notifyMenu, tele.ModeHTML)
	})

	// ۹. قفل پیوی ضد مزاحم (PV Lock)
	bot.Handle(&btnWP_PVLock, func(c tele.Context) error {
		userID := c.Sender().ID
		statusStr := "🔴 خاموش"
		if IsPVLockEnabled(userID) {
			statusStr = "🟢 روشن"
		}

		text := fmt.Sprintf(`🔐 <b>مدیریت سیستم قفل پیوی (PV Lock)</b>
➖➖➖➖➖➖➖➖➖➖
📌 <b>وضعیت فعلی شما:</b> %s
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
با روشن کردن این قابلیت، هر فرد غریبه‌ای که در پیوی به شما پیام بدهد (غیر از دوستان، ربات‌ها و افراد تایید شده)، پیام و چت او در کسری از ثانیه <b>به صورت دوطرفه</b> حذف می‌گردد تا مزاحمتی ایجاد نشود.

💬 <b>دستورات چت:</b>
▫️ <code>قفل پیوی روشن</code>
▫️ <code>قفل پیوی خاموش</code>
▫️ <code>بازکردن پیوی</code> (داخل پیوی کاربر)
▫️ <code>بستن پیوی</code>`, statusStr)
		return c.Send(text, pvLockMenu, tele.ModeHTML)
	})

	bot.Handle(&btnPV_On, func(c tele.Context) error {
		userID := c.Sender().ID
		SetPVLockEnabled(userID, true)
		return c.Send("🟢 <b>قفل پیوی فعال شد.</b>\nاز این پس پیام غریبه‌ها در پیوی به صورت دوطرفه حذف خواهد شد.", pvLockMenu, tele.ModeHTML)
	})

	bot.Handle(&btnPV_Off, func(c tele.Context) error {
		userID := c.Sender().ID
		SetPVLockEnabled(userID, false)
		return c.Send("🔴 <b>قفل پیوی غیرفعال شد.</b>", pvLockMenu, tele.ModeHTML)
	})
}

func HandleWolfPlusText(c tele.Context) bool {
	userID := c.Sender().ID
	wolfPlusStatesMu.RLock()
	state, exists := wolfPlusStates[userID]
	wolfPlusStatesMu.RUnlock()

	if !exists {
		return false
	}

	text := strings.TrimSpace(c.Text())
	if strings.HasPrefix(text, "🔙") {
		wolfPlusStatesMu.Lock()
		delete(wolfPlusStates, userID)
		wolfPlusStatesMu.Unlock()
		return false
	}

	if state == "waiting_group_input" || state == "waiting_for_group_input" {
		cleanID := text
		if strings.HasPrefix(cleanID, "-100") || strings.HasPrefix(cleanID, "-") {
			chatID, err := strconv.ParseInt(cleanID, 10, 64)
			if err == nil {
				title := fmt.Sprintf("گروه (%d)", chatID)
				_, _ = db.Exec(`
					INSERT INTO wolf_antidel_groups (owner_id, chat_id, chat_title)
					VALUES (?, ?, ?)
					ON DUPLICATE KEY UPDATE chat_title = VALUES(chat_title)
				`, userID, chatID, title)

				wolfPlusStatesMu.Lock()
				delete(wolfPlusStates, userID)
				wolfPlusStatesMu.Unlock()

				_ = c.Send(fmt.Sprintf("✅ <b>گروه با موفقیت اضافه شد:</b>\n🏷 <b>شناسه:</b> <code>%d</code>", chatID), groupDelMenu, tele.ModeHTML)
				return true
			}
		}

		username := text
		username = strings.TrimPrefix(username, "https://t.me/")
		username = strings.TrimPrefix(username, "http://t.me/")
		username = strings.TrimPrefix(username, "t.me/")
		username = strings.TrimPrefix(username, "@")

		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()

		if ok && ub.Client != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			resolved, err := ub.Client.API().ContactsResolveUsername(ctx, username)
			if err == nil && len(resolved.Chats) > 0 {
				var chatID int64
				var title string
				switch ch := resolved.Chats[0].(type) {
				case *tg.Channel:
					chatID = -1000000000000 - ch.ID
					title = ch.Title
				case *tg.Chat:
					chatID = -ch.ID
					title = ch.Title
				}

				if chatID != 0 {
					_, _ = db.Exec(`
						INSERT INTO wolf_antidel_groups (owner_id, chat_id, chat_title)
						VALUES (?, ?, ?)
						ON DUPLICATE KEY UPDATE chat_title = VALUES(chat_title)
					`, userID, chatID, title)

					wolfPlusStatesMu.Lock()
					delete(wolfPlusStates, userID)
					wolfPlusStatesMu.Unlock()

					_ = c.Send(fmt.Sprintf("✅ <b>گروه با موفقیت اضافه شد:</b>\n🏷 <b>نام:</b> %s\n🆔 <b>آیدی:</b> <code>%d</code>", title, chatID), groupDelMenu, tele.ModeHTML)
					return true
				}
			}
		}

		_ = c.Send("❌ <b>خطا در شناسایی گروه!</b> لطفاً آیدی عددی را ارسال کنید.", groupDelMenu, tele.ModeHTML)
		wolfPlusStatesMu.Lock()
		delete(wolfPlusStates, userID)
		wolfPlusStatesMu.Unlock()
		return true
	}

	if state == "waiting_for_target_input" {
		targetInput := strings.TrimPrefix(text, "@")
		targetInput = strings.TrimPrefix(targetInput, "https://t.me/")
		targetInput = strings.TrimPrefix(targetInput, "t.me/")

		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()

		if !ok || ub.Client == nil {
			_ = c.Send("❌ <b>سلف شما آنلاین نیست!</b>", targetMenu, tele.ModeHTML)
			wolfPlusStatesMu.Lock()
			delete(wolfPlusStates, userID)
			wolfPlusStatesMu.Unlock()
			return true
		}

		go func(input string) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			var targetUser *tg.User
			var targetInputUser tg.InputUserClass

			if numID, err := strconv.ParseInt(input, 10, 64); err == nil {
				if numID == ub.UserID {
					self, sErr := ub.Client.Self(ctx)
					if sErr == nil {
						targetUser = self
						targetInputUser = &tg.InputUserSelf{}
					}
				}
			}

			if targetUser == nil {
				resolved, err := ub.Client.API().ContactsResolveUsername(ctx, input)
				if err == nil && len(resolved.Users) > 0 {
					if u, ok := resolved.Users[0].(*tg.User); ok {
						targetUser = u
						if u.Self || u.ID == ub.UserID {
							targetInputUser = &tg.InputUserSelf{}
						} else {
							targetInputUser = &tg.InputUser{UserID: u.ID, AccessHash: u.AccessHash}
						}
					}
				}
			}

			if targetUser != nil {
				var bio string
				full, fErr := ub.Client.API().UsersGetFullUser(ctx, targetInputUser)
				if fErr == nil {
					bio = strings.TrimSpace(full.FullUser.About)
				}

				var photoID int64
				if p, ok := targetUser.Photo.(*tg.UserProfilePhoto); ok {
					photoID = p.PhotoID
				}

				var aHash int64
				if inp, ok := targetInputUser.(*tg.InputUser); ok {
					aHash = inp.AccessHash
				}

				_, _ = db.Exec(`
					INSERT INTO wolf_targets (owner_id, target_id, access_hash, first_name, last_name, username, bio, photo_id)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)
					ON DUPLICATE KEY UPDATE access_hash=VALUES(access_hash), first_name=VALUES(first_name),
					last_name=VALUES(last_name), username=VALUES(username), bio=VALUES(bio), photo_id=VALUES(photo_id)
				`, userID, targetUser.ID, aHash, targetUser.FirstName, targetUser.LastName, targetUser.Username, bio, photoID)

				_ = c.Send(fmt.Sprintf("✅ <b>مخاطب با موفقیت به ردیاب اضافه شد:</b>\n👤 <b>نام:</b> %s\n🆔 <b>آیدی:</b> <code>%d</code>", formatTelegramUser(targetUser), targetUser.ID), targetMenu, tele.ModeHTML)
				return
			}

			_ = c.Send("❌ <b>مخاطب یافت نشد!</b> یوزرنیم را بررسی کنید.", targetMenu, tele.ModeHTML)
		}(targetInput)

		wolfPlusStatesMu.Lock()
		delete(wolfPlusStates, userID)
		wolfPlusStatesMu.Unlock()
		return true
	}

	if state == "waiting_for_target_delete" {
		var targetID int64
		if strings.Contains(text, "(") && strings.Contains(text, ")") {
			start := strings.LastIndex(text, "(")
			end := strings.LastIndex(text, ")")
			if start != -1 && end != -1 && end > start {
				idStr := text[start+1 : end]
				targetID, _ = strconv.ParseInt(idStr, 10, 64)
			}
		}
		if targetID == 0 {
			clean := strings.TrimSpace(text)
			clean = strings.TrimPrefix(clean, "❌ ")
			if id, err := strconv.ParseInt(clean, 10, 64); err == nil {
				targetID = id
			}
		}

		var res sql.Result
		var err error
		if targetID != 0 {
			res, err = db.Exec("DELETE FROM wolf_targets WHERE owner_id = ? AND target_id = ?", userID, targetID)
		} else {
			cleanUser := strings.TrimPrefix(strings.TrimSpace(text), "@")
			res, err = db.Exec("DELETE FROM wolf_targets WHERE owner_id = ? AND username = ?", userID, cleanUser)
		}

		wolfPlusStatesMu.Lock()
		delete(wolfPlusStates, userID)
		wolfPlusStatesMu.Unlock()

		if err == nil {
			if affected, _ := res.RowsAffected(); affected > 0 {
				_ = c.Send("✅ <b>مخاطب با موفقیت از ردیاب حذف شد.</b>", targetMenu, tele.ModeHTML)
				return true
			}
		}
		_ = c.Send("❌ <b>مخاطب در لیست یافت نشد.</b>", targetMenu, tele.ModeHTML)
		return true
	}

	if state == "waiting_for_protected_input" {
		cleanID := text
		if strings.HasPrefix(cleanID, "-100") || strings.HasPrefix(cleanID, "-") {
			chatID, err := strconv.ParseInt(cleanID, 10, 64)
			if err == nil {
				title := fmt.Sprintf("کانال/گروه (%d)", chatID)
				_, _ = db.Exec(`
					INSERT INTO wolf_protected_channels (owner_id, chat_id, chat_title)
					VALUES (?, ?, ?)
					ON DUPLICATE KEY UPDATE chat_title = VALUES(chat_title)
				`, userID, chatID, title)

				wolfPlusStatesMu.Lock()
				delete(wolfPlusStates, userID)
				wolfPlusStatesMu.Unlock()

				_ = c.Send(fmt.Sprintf("✅ <b>منبع ضدکپی با موفقیت ثبت شد:</b>\n🏷 <b>شناسه:</b> <code>%d</code>", chatID), protectedMenu, tele.ModeHTML)
				return true
			}
		}

		username := text
		username = strings.TrimPrefix(username, "https://t.me/")
		username = strings.TrimPrefix(username, "http://t.me/")
		username = strings.TrimPrefix(username, "t.me/")
		username = strings.TrimPrefix(username, "@")

		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()

		if ok && ub.Client != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			resolved, err := ub.Client.API().ContactsResolveUsername(ctx, username)
			if err == nil && len(resolved.Chats) > 0 {
				var chatID int64
				var title string
				switch ch := resolved.Chats[0].(type) {
				case *tg.Channel:
					chatID = -1000000000000 - ch.ID
					title = ch.Title
				case *tg.Chat:
					chatID = -ch.ID
					title = ch.Title
				}

				if chatID != 0 {
					_, _ = db.Exec(`
						INSERT INTO wolf_protected_channels (owner_id, chat_id, chat_title)
						VALUES (?, ?, ?)
						ON DUPLICATE KEY UPDATE chat_title = VALUES(chat_title)
					`, userID, chatID, title)

					wolfPlusStatesMu.Lock()
					delete(wolfPlusStates, userID)
					wolfPlusStatesMu.Unlock()

					_ = c.Send(fmt.Sprintf("✅ <b>منبع ضدکپی با موفقیت ثبت شد:</b>\n🏷 <b>نام:</b> %s\n🆔 <b>آیدی:</b> <code>%d</code>", title, chatID), protectedMenu, tele.ModeHTML)
					return true
				}
			}
		}

		_ = c.Send("❌ <b>خطا در شناسایی!</b> لطفاً آیدی عددی یا یوزرنیم کانال را ارسال کنید.", protectedMenu, tele.ModeHTML)
		wolfPlusStatesMu.Lock()
		delete(wolfPlusStates, userID)
		wolfPlusStatesMu.Unlock()
		return true
	}

	if state == "waiting_for_protected_delete" {
		var chatID int64
		if strings.Contains(text, "(") && strings.Contains(text, ")") {
			start := strings.LastIndex(text, "(")
			end := strings.LastIndex(text, ")")
			if start != -1 && end != -1 && end > start {
				idStr := text[start+1 : end]
				chatID, _ = strconv.ParseInt(idStr, 10, 64)
			}
		}
		if chatID == 0 {
			clean := strings.TrimSpace(text)
			clean = strings.TrimPrefix(clean, "❌ ")
			if id, err := strconv.ParseInt(clean, 10, 64); err == nil {
				chatID = id
			}
		}

		res, err := db.Exec("DELETE FROM wolf_protected_channels WHERE owner_id = ? AND chat_id = ?", userID, chatID)
		wolfPlusStatesMu.Lock()
		delete(wolfPlusStates, userID)
		wolfPlusStatesMu.Unlock()

		if err == nil {
			if affected, _ := res.RowsAffected(); affected > 0 {
				_ = c.Send("✅ <b>منبع با موفقیت از دانلودر ضدکپی حذف شد.</b>", protectedMenu, tele.ModeHTML)
				return true
			}
		}
		_ = c.Send("❌ <b>منبع در لیست یافت نشد.</b>", protectedMenu, tele.ModeHTML)
		return true
	}

	return false
}

func relayGhostPVMessage(ctx context.Context, client *telegram.Client, userID int64, senderName string, senderID int64, text, mediaType string) {
	report := fmt.Sprintf(
		"👻 ɢʜᴏsᴛ ᴍᴏᴅᴇ | پیام مخفی\n"+
			"━━━━━━━━━━━━━━━━━\n"+
			"👤 فرستنده : %s\n"+
			"🆔 آیدی : %d\n"+
			"⏰ زمان دریافت : %s\n"+
			"📁 نوع پیام : %s",
		senderName, senderID, getTehranCurrentTime(), mediaType,
	)
	if strings.TrimSpace(text) != "" {
		report += fmt.Sprintf("\n━━━━━━━━━━━━━━━━━\n📄 محتوا :\n%s", text)
	}

	sCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	_, _ = client.API().MessagesSendMessage(sCtx, &tg.MessagesSendMessageRequest{
		Peer:     &tg.InputPeerSelf{},
		Message:  report,
		RandomID: rand.Int63(),
	})

	sendBotAlert(userID, "👻 پیام مخفی (حالت روح)", fmt.Sprintf("از: %s (%s)", senderName, mediaType))
}

func WolfPlusHandleIncoming(ctx context.Context, client *telegram.Client, bot *tele.Bot, userID int64, msg *tg.Message, e tg.Entities) {
	if msg.Out {
		return
	}

	var isPV bool
	var chatID int64
	chatName := "چت خصوصی"

	switch p := msg.PeerID.(type) {
	case *tg.PeerUser:
		isPV = true
		chatID = p.UserID
	case *tg.PeerChat:
		chatID = p.ChatID
		chatName = "گروه"
	case *tg.PeerChannel:
		chatID = p.ChannelID
		chatName = "کانال/سوپرگروه"
	}

	if controllerBotID != 0 && (chatID == controllerBotID) {
		return
	}

	if !isPV {
		var isProtEnabled bool
		var protCount int
		_ = db.QueryRow("SELECT is_protected_saver_enabled FROM users WHERE id = ?", userID).Scan(&isProtEnabled)
		if isProtEnabled {
			_ = db.QueryRow(`
				SELECT COUNT(*) FROM wolf_protected_channels 
				WHERE owner_id = ? AND (chat_id = ? OR chat_id = ? OR chat_id = ?)
			`, userID, chatID, -chatID, -1000000000000-chatID).Scan(&protCount)

			if protCount > 0 {
				go relayProtectedMessageToSaved(ctx, client, userID, msg, chatName)
			}
		}

		var count int
		_ = db.QueryRow(`
			SELECT COUNT(*) FROM wolf_antidel_groups 
			WHERE owner_id = ? AND (chat_id = ? OR chat_id = ? OR chat_id = ?)
		`, userID, chatID, -chatID, -1000000000000-chatID).Scan(&count)

		if count == 0 {
			return
		}
	}

	senderID := int64(0)
	senderName := ""

	if fromUser, ok := msg.FromID.(*tg.PeerUser); ok {
		senderID = fromUser.UserID
	} else if peerUser, ok := msg.PeerID.(*tg.PeerUser); ok {
		senderID = peerUser.UserID
	}

	if senderID != 0 {
		if controllerBotID != 0 && senderID == controllerBotID {
			return
		}

		if u, exists := e.Users[senderID]; exists {
			if u.Bot {
				return
			}
			senderName = formatTelegramUser(u)
			if senderName != "" {
				peerNamesMu.Lock()
				peerNames[senderID] = senderName
				peerNamesMu.Unlock()
			}
		}

		if senderName == "" {
			peerNamesMu.RLock()
			senderName = peerNames[senderID]
			peerNamesMu.RUnlock()
		}

		if senderName == "" {
			gCtx, gCancel := context.WithTimeout(context.Background(), 4*time.Second)
			mRes, mErr := client.API().MessagesGetMessages(gCtx, []tg.InputMessageClass{&tg.InputMessageID{ID: msg.ID}})
			gCancel()
			if mErr == nil {
				var usersList []tg.UserClass
				switch mr := mRes.(type) {
				case *tg.MessagesMessages:
					usersList = mr.Users
				case *tg.MessagesMessagesSlice:
					usersList = mr.Users
				case *tg.MessagesChannelMessages:
					usersList = mr.Users
				}
				PopulatePeerCache(usersList)
				peerNamesMu.RLock()
				senderName = peerNames[senderID]
				peerNamesMu.RUnlock()
			}
		}

		if senderName == "" {
			senderName = fmt.Sprintf("کاربر (%d)", senderID)
		}
	}

	if isPV && msg.Media != nil {
		var timerEnabled bool
		_ = db.QueryRow("SELECT is_timer_media_enabled FROM users WHERE id = ?", userID).Scan(&timerEnabled)
		if timerEnabled {
			go func() {
				dCtx, dCancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer dCancel()
				downloadAndRelayTTL(dCtx, client, userID, msg, e)
			}()
		}
	}

	mediaType := "متن"
	var fileLoc tg.InputFileLocationClass
	var fileExt string

	if msg.Media != nil {
		switch m := msg.Media.(type) {
		case *tg.MessageMediaPhoto:
			mediaType = "عکس"
			fileExt = ".jpg"
			if photo, ok := m.Photo.(*tg.Photo); ok {
				thumbSize := "x"
				if len(photo.Sizes) > 0 {
					thumbSize = photo.Sizes[len(photo.Sizes)-1].GetType()
				}
				fileLoc = &tg.InputPhotoFileLocation{
					ID:            photo.ID,
					AccessHash:    photo.AccessHash,
					FileReference: photo.FileReference,
					ThumbSize:     thumbSize,
				}
			}
		case *tg.MessageMediaDocument:
			if doc, ok := m.Document.(*tg.Document); ok {
				fileLoc = &tg.InputDocumentFileLocation{
					ID:            doc.ID,
					AccessHash:    doc.AccessHash,
					FileReference: doc.FileReference,
				}
				mime := strings.ToLower(doc.MimeType)
				if strings.HasPrefix(mime, "audio/") || strings.Contains(mime, "ogg") {
					mediaType = "صوت"
					fileExt = ".ogg"
				} else if strings.HasPrefix(mime, "video/") {
					mediaType = "ویدیو"
					fileExt = ".mp4"
				} else {
					mediaType = "فایل"
					fileExt = ".dat"
				}
			}
		}
	}

	text := strings.TrimSpace(msg.Message)
	if text == "" && mediaType == "متن" {
		return
	}

	if isPV {
		var ghostEnabled bool
		_ = db.QueryRow("SELECT is_ghost_mode_enabled FROM users WHERE id = ?", userID).Scan(&ghostEnabled)
		if ghostEnabled {
			go relayGhostPVMessage(context.Background(), client, userID, senderName, senderID, text, mediaType)
		}
	}

	cachedFilePath := ""
	var antiDelEnabled bool
	_ = db.QueryRow("SELECT is_anti_delete_enabled FROM users WHERE id = ?", userID).Scan(&antiDelEnabled)

	if antiDelEnabled && fileLoc != nil {
		cacheDir := "/tmp/wolf_cache"
		_ = os.MkdirAll(cacheDir, 0755)
		cachedFilePath = filepath.Join(cacheDir, fmt.Sprintf("%d_%d_%d%s", userID, chatID, msg.ID, fileExt))
		go func(target string, loc tg.InputFileLocationClass) {
			dCtx, dCancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer dCancel()
			_ = downloadLocationToFile(dCtx, client, loc, target)
		}(cachedFilePath, fileLoc)
	}

	_, _ = db.Exec(`
		INSERT INTO wolf_message_cache (owner_id, chat_id, message_id, sender_id, sender_name, chat_name, message_text, media_type, cached_file_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE message_text = VALUES(message_text), media_type = VALUES(media_type), cached_file_path = VALUES(cached_file_path)
	`, userID, chatID, msg.ID, senderID, senderName, chatName, text, mediaType, cachedFilePath)
}

func RegisterWolfPlusDispatcher(dispatcher *tg.UpdateDispatcher, client *telegram.Client, userID int64) {
	dispatcher.OnDeleteMessages(func(ctx context.Context, e tg.Entities, u *tg.UpdateDeleteMessages) error {
		go handleDeletedMessages(client, userID, u.Messages)
		return nil
	})

	dispatcher.OnDeleteChannelMessages(func(ctx context.Context, e tg.Entities, u *tg.UpdateDeleteChannelMessages) error {
		go handleDeletedMessages(client, userID, u.Messages)
		return nil
	})

	dispatcher.OnEditMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateEditMessage) error {
		go handleEditedMessage(client, userID, u.Message)
		return nil
	})

	dispatcher.OnEditChannelMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateEditChannelMessage) error {
		go handleEditedMessage(client, userID, u.Message)
		return nil
	})
}

func handleDeletedMessages(client *telegram.Client, ownerID int64, msgIDs []int) {
	var antiDelete bool
	_ = db.QueryRow("SELECT is_anti_delete_enabled FROM users WHERE id = ?", ownerID).Scan(&antiDelete)
	if !antiDelete || len(msgIDs) == 0 {
		return
	}

	for _, msgID := range msgIDs {
		var senderID int64
		var senderName, chatName, msgText, mediaType, cachedPath string

		err := db.QueryRow(`
			SELECT sender_id, sender_name, chat_name, message_text, media_type, cached_file_path 
			FROM wolf_message_cache 
			WHERE owner_id = ? AND message_id = ?
		`, ownerID, msgID).Scan(&senderID, &senderName, &chatName, &msgText, &mediaType, &cachedPath)

		if err == nil {
			if strings.HasPrefix(senderName, "کاربر (") || senderName == "" {
				peerNamesMu.RLock()
				if cached, ok := peerNames[senderID]; ok && cached != "" {
					senderName = cached
				}
				peerNamesMu.RUnlock()
			}

			report := fmt.Sprintf(
				"🐺 ɢᴜᴀʀᴅ | ولف پلاس\n"+
					"━━━━━━━━━━━━━━━━━\n"+
					"👤 فرستنده : %s\n"+
					"🆔 آیدی : %d\n"+
					"💬 چت : %s\n"+
					"⏰ زمان حذف : %s\n"+
					"🗑 حذف شده : %s",
				senderName, senderID, chatName, getTehranCurrentTime(), mediaType,
			)

			if strings.TrimSpace(msgText) != "" {
				report += fmt.Sprintf("\n━━━━━━━━━━━━━━━━━\n📄 محتوای پیام :\n%s", msgText)
			}

			cTimeout, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			sentMedia := false

			if cachedPath != "" {
				if _, statErr := os.Stat(cachedPath); statErr == nil {
					u := uploader.NewUploader(client.API())
					inputFile, upErr := u.FromPath(cTimeout, cachedPath)
					if upErr == nil {
						if mediaType == "عکس" {
							_, err = client.API().MessagesSendMedia(cTimeout, &tg.MessagesSendMediaRequest{
								Peer:     &tg.InputPeerSelf{},
								Media:    &tg.InputMediaUploadedPhoto{File: inputFile},
								Message:  report,
								RandomID: rand.Int63(),
							})
							sentMedia = (err == nil)
						} else {
							mime := "application/octet-stream"
							if mediaType == "صوت" {
								mime = "audio/ogg"
							} else if mediaType == "ویدیو" {
								mime = "video/mp4"
							}
							_, err = client.API().MessagesSendMedia(cTimeout, &tg.MessagesSendMediaRequest{
								Peer:     &tg.InputPeerSelf{},
								Media:    &tg.InputMediaUploadedDocument{File: inputFile, MimeType: mime},
								Message:  report,
								RandomID: rand.Int63(),
							})
							sentMedia = (err == nil)
						}
					}
					_ = os.Remove(cachedPath)
				}
			}

			if !sentMedia {
				_, _ = client.API().MessagesSendMessage(cTimeout, &tg.MessagesSendMessageRequest{
					Peer:     &tg.InputPeerSelf{},
					Message:  report,
					RandomID: rand.Int63(),
				})
			}
			cancel()

			sendBotAlert(ownerID, "🗑 حذف پیام در چت خصوصی", fmt.Sprintf("فرستنده: %s (%s)", senderName, mediaType))

			_, _ = db.Exec("DELETE FROM wolf_message_cache WHERE owner_id = ? AND message_id = ?", ownerID, msgID)
		}
	}
}

func handleEditedMessage(client *telegram.Client, ownerID int64, messageClass tg.MessageClass) {
	var editLogger bool
	_ = db.QueryRow("SELECT is_edit_logger_enabled FROM users WHERE id = ?", ownerID).Scan(&editLogger)
	if !editLogger {
		return
	}

	msg, ok := messageClass.(*tg.Message)
	if !ok || msg.Out || strings.TrimSpace(msg.Message) == "" {
		return
	}

	newText := strings.TrimSpace(msg.Message)
	var oldText, senderName, chatName, mediaType string
	var senderID int64

	err := db.QueryRow(`
		SELECT sender_id, sender_name, chat_name, message_text, media_type 
		FROM wolf_message_cache 
		WHERE owner_id = ? AND message_id = ?
	`, ownerID, msg.ID).Scan(&senderID, &senderName, &chatName, &oldText, &mediaType)

	if err == nil && oldText != "" && oldText != newText {
		if strings.HasPrefix(senderName, "کاربر (") || senderName == "" {
			peerNamesMu.RLock()
			if cached, ok := peerNames[senderID]; ok && cached != "" {
				senderName = cached
			}
			peerNamesMu.RUnlock()
		}

		report := fmt.Sprintf(
			"🐺 ᴇᴅɪᴛ ʟᴏɢɢᴇʀ | ولف پلاس\n"+
				"━━━━━━━━━━━━━━━━━\n"+
				"👤 فرستنده : %s\n"+
				"🆔 آیدی : %d\n"+
				"💬 چت : %s\n"+
				"⏰ زمان ادیت : %s\n"+
				"✏️ نوع محتوا : %s\n"+
				"━━━━━━━━━━━━━━━━━\n"+
				"📌 متن قبلی :\n%s\n\n"+
				"📝 متن جدید :\n%s",
			senderName, senderID, chatName, getTehranCurrentTime(), mediaType,
			oldText, newText,
		)

		cTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, _ = client.API().MessagesSendMessage(cTimeout, &tg.MessagesSendMessageRequest{
			Peer:     &tg.InputPeerSelf{},
			Message:  report,
			RandomID: rand.Int63(),
		})
		cancel()

		sendBotAlert(ownerID, "📝 ویرایش پیام در پیوی", fmt.Sprintf("فرستنده: %s", senderName))

		_, _ = db.Exec("UPDATE wolf_message_cache SET message_text = ? WHERE owner_id = ? AND message_id = ?", newText, ownerID, msg.ID)
	}
}

func StartTargetTrackerWorker(ctx context.Context, client *telegram.Client, ownerID int64) {
	ticker := time.NewTicker(45 * time.Second)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				rows, err := db.Query(`
					SELECT target_id, access_hash, first_name, last_name, username, bio, photo_id 
					FROM wolf_targets WHERE owner_id = ?
				`, ownerID)
				if err != nil {
					continue
				}

				for rows.Next() {
					var targetID, accessHash, oldPhotoID int64
					var oldFirst, oldLast, oldUser, oldBio string
					if err := rows.Scan(&targetID, &accessHash, &oldFirst, &oldLast, &oldUser, &oldBio, &oldPhotoID); err != nil {
						continue
					}

					var inputUser tg.InputUserClass
					if targetID == ownerID {
						inputUser = &tg.InputUserSelf{}
					} else {
						inputUser = &tg.InputUser{UserID: targetID, AccessHash: accessHash}
					}

					reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
					full, err := client.API().UsersGetFullUser(reqCtx, inputUser)
					cancel()

					if err != nil {
						continue
					}

					var u *tg.User
					for _, userClass := range full.Users {
						if usr, ok := userClass.(*tg.User); ok && usr.ID == targetID {
							u = usr
							break
						}
					}
					if u == nil {
						continue
					}

					newFirst := strings.TrimSpace(u.FirstName)
					newLast := strings.TrimSpace(u.LastName)
					newUsername := strings.TrimSpace(u.Username)
					newBio := strings.TrimSpace(full.FullUser.About)

					var newPhotoID int64
					if p, ok := u.Photo.(*tg.UserProfilePhoto); ok {
						newPhotoID = p.PhotoID
					}

					var changes []string

					if oldFirst != newFirst || oldLast != newLast {
						oldName := strings.TrimSpace(oldFirst + " " + oldLast)
						newName := strings.TrimSpace(newFirst + " " + newLast)
						if oldName == "" {
							oldName = "بدون نام"
						}
						if newName == "" {
							newName = "حذف شد"
						}
						changes = append(changes, fmt.Sprintf("👤 تغییر نام :\nاز: %s\nبه: %s", oldName, newName))
					}

					if oldUser != newUsername {
						oldU := "@" + oldUser
						if oldUser == "" {
							oldU = "نداشت"
						}
						newU := "@" + newUsername
						if newUsername == "" {
							newU = "حذف شد"
						}
						changes = append(changes, fmt.Sprintf("🌐 تغییر یوزرنیم :\nاز: %s\nبه: %s", oldU, newU))
					}

					if oldBio != newBio {
						oldB := oldBio
						if oldB == "" {
							oldB = "خالی"
						}
						newB := newBio
						if newB == "" {
							newB = "حذف شد"
						}
						changes = append(changes, fmt.Sprintf("📝 تغییر بیوگرافی :\nاز: %s\nبه: %s", oldB, newB))
					}

					if oldPhotoID != newPhotoID {
						if newPhotoID == 0 {
							changes = append(changes, "📸 عکس پروفایل حذف شد.")
						} else if oldPhotoID == 0 {
							changes = append(changes, "📸 عکس پروفایل جدید تنظیم شد.")
						} else {
							changes = append(changes, "📸 تغییر عکس پروفایل : عکس جدید تنظیم شد.")
						}
					}

					if len(changes) > 0 {
						report := fmt.Sprintf(
							"🐺 ᴛᴀʀɢᴇᴛ ᴛʀᴀᴄᴋᴇʀ | ولف پلاس\n"+
								"━━━━━━━━━━━━━━━━━\n"+
								"🎯 هدف : %s\n"+
								"🆔 آیدی : %d\n"+
								"⏰ زمان ثبت : %s\n"+
								"━━━━━━━━━━━━━━━━━\n"+
								"%s",
							formatTelegramUser(u), targetID, getTehranCurrentTime(),
							strings.Join(changes, "\n\n"),
						)

						sCtx, sCancel := context.WithTimeout(ctx, 10*time.Second)
						_, _ = client.API().MessagesSendMessage(sCtx, &tg.MessagesSendMessageRequest{
							Peer:     &tg.InputPeerSelf{},
							Message:  report,
							RandomID: rand.Int63(),
						})
						sCancel()

						sendBotAlert(ownerID, "🎯 تغییرات مخاطب خاص", fmt.Sprintf("هدف: %s", formatTelegramUser(u)))

						_, _ = db.Exec(`
							UPDATE wolf_targets 
							SET first_name=?, last_name=?, username=?, bio=?, photo_id=? 
							WHERE owner_id=? AND target_id=?
						`, newFirst, newLast, newUsername, newBio, newPhotoID, ownerID, targetID)
					}
				}
				rows.Close()
			}
		}
	}()
}

func downloadLocationToFile(ctx context.Context, client *telegram.Client, loc tg.InputFileLocationClass, targetFile string) error {
	var fileData []byte
	offset := int64(0)
	limit := 1024 * 1024

	for {
		reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		res, err := client.API().UploadGetFile(reqCtx, &tg.UploadGetFileRequest{
			Location: loc,
			Offset:   offset,
			Limit:    limit,
		})
		cancel()
		if err != nil {
			return err
		}

		file, ok := res.(*tg.UploadFile)
		if !ok || len(file.Bytes) == 0 {
			break
		}

		fileData = append(fileData, file.Bytes...)
		if len(file.Bytes) < limit {
			break
		}
		offset += int64(len(file.Bytes))
		if len(fileData) > 25*1024*1024 {
			break
		}
	}

	if len(fileData) == 0 {
		return fmt.Errorf("empty file")
	}

	return os.WriteFile(targetFile, fileData, 0644)
}

func downloadAndRelayTTL(ctx context.Context, client *telegram.Client, targetUserID int64, msg *tg.Message, e tg.Entities) {
	var isTTL bool
	var ttlSeconds int
	var mediaType string

	switch m := msg.Media.(type) {
	case *tg.MessageMediaPhoto:
		if m.TTLSeconds > 0 {
			isTTL = true
			ttlSeconds = m.TTLSeconds
			mediaType = "photo"
		}
	case *tg.MessageMediaDocument:
		if m.TTLSeconds > 0 {
			isTTL = true
			ttlSeconds = m.TTLSeconds
			mediaType = "video"
		}
	}

	if !isTTL {
		return
	}

	senderID := int64(0)
	senderName := "کاربر تلگرام"

	if fromUser, ok := msg.FromID.(*tg.PeerUser); ok {
		senderID = fromUser.UserID
	} else if peerUser, ok := msg.PeerID.(*tg.PeerUser); ok {
		senderID = peerUser.UserID
	}

	if senderID != 0 {
		if u, exists := e.Users[senderID]; exists {
			formatted := formatTelegramUser(u)
			if formatted != "" {
				senderName = formatted
			}
		}
		if senderName == "کاربر تلگرام" {
			peerNamesMu.RLock()
			if cached, ok := peerNames[senderID]; ok && cached != "" {
				senderName = cached
			}
			peerNamesMu.RUnlock()
		}
	}

	var loc tg.InputFileLocationClass
	var fileExt string

	switch m := msg.Media.(type) {
	case *tg.MessageMediaPhoto:
		photo, ok := m.Photo.(*tg.Photo)
		if !ok {
			return
		}
		thumbSize := "x"
		if len(photo.Sizes) > 0 {
			thumbSize = photo.Sizes[len(photo.Sizes)-1].GetType()
		}
		loc = &tg.InputPhotoFileLocation{
			ID:            photo.ID,
			AccessHash:    photo.AccessHash,
			FileReference: photo.FileReference,
			ThumbSize:     thumbSize,
		}
		fileExt = ".jpg"
	case *tg.MessageMediaDocument:
		doc, ok := m.Document.(*tg.Document)
		if !ok {
			return
		}
		loc = &tg.InputDocumentFileLocation{
			ID:            doc.ID,
			AccessHash:    doc.AccessHash,
			FileReference: doc.FileReference,
		}
		fileExt = ".mp4"
	}

	if loc == nil {
		return
	}

	tmpFile := filepath.Join("/tmp", fmt.Sprintf("wolf_ttl_%d%s", time.Now().UnixNano(), fileExt))
	if err := downloadLocationToFile(ctx, client, loc, tmpFile); err != nil {
		return
	}
	defer os.Remove(tmpFile)

	caption := fmt.Sprintf(
		"🐺 رسانه تایمردار (View-Once) | ولف +\n"+
			"━━━━━━━━━━━━━━━━━\n"+
			"👤 فرستنده : %s\n"+
			"🆔 آیدی : %d\n"+
			"⏰ زمان دریافت : %s\n"+
			"⏱ تایمر : %d ثانیه\n"+
			"📸 نوع مدیا : %s",
		senderName, senderID, getTehranCurrentTime(), ttlSeconds,
		func() string {
			if mediaType == "photo" {
				return "عکس"
			}
			return "ویدیو"
		}(),
	)

	u := uploader.NewUploader(client.API())
	inputFile, err := u.FromPath(ctx, tmpFile)
	if err == nil {
		if mediaType == "photo" {
			_, _ = client.API().MessagesSendMedia(ctx, &tg.MessagesSendMediaRequest{
				Peer:     &tg.InputPeerSelf{},
				Media:    &tg.InputMediaUploadedPhoto{File: inputFile},
				Message:  caption,
				RandomID: rand.Int63(),
			})
		} else {
			_, _ = client.API().MessagesSendMedia(ctx, &tg.MessagesSendMediaRequest{
				Peer:     &tg.InputPeerSelf{},
				Media:    &tg.InputMediaUploadedDocument{File: inputFile, MimeType: "video/mp4"},
				Message:  caption,
				RandomID: rand.Int63(),
			})
		}
	}

	sendBotAlert(targetUserID, "📸 رسانه تایمردار ذخیره شد", fmt.Sprintf("فرستنده: %s (%s)", senderName, mediaType))
}

func relayProtectedMessageToSaved(ctx context.Context, client *telegram.Client, userID int64, msg *tg.Message, sourceTitle string) {
	if msg == nil {
		return
	}

	mediaType := "متن"
	var fileLoc tg.InputFileLocationClass
	var fileExt string
	var mimeType string

	if msg.Media != nil {
		switch m := msg.Media.(type) {
		case *tg.MessageMediaPhoto:
			mediaType = "عکس"
			fileExt = ".jpg"
			if photo, ok := m.Photo.(*tg.Photo); ok {
				thumbSize := "x"
				if len(photo.Sizes) > 0 {
					thumbSize = photo.Sizes[len(photo.Sizes)-1].GetType()
				}
				fileLoc = &tg.InputPhotoFileLocation{
					ID:            photo.ID,
					AccessHash:    photo.AccessHash,
					FileReference: photo.FileReference,
					ThumbSize:     thumbSize,
				}
			}
		case *tg.MessageMediaDocument:
			if doc, ok := m.Document.(*tg.Document); ok {
				fileLoc = &tg.InputDocumentFileLocation{
					ID:            doc.ID,
					AccessHash:    doc.AccessHash,
					FileReference: doc.FileReference,
				}
				mimeType = strings.ToLower(doc.MimeType)
				if strings.HasPrefix(mimeType, "audio/") || strings.Contains(mimeType, "ogg") {
					mediaType = "صوت / وویس"
					fileExt = ".ogg"
				} else if strings.HasPrefix(mimeType, "video/") {
					mediaType = "ویدیو"
					fileExt = ".mp4"
				} else {
					mediaType = "فایل سند"
					fileExt = ".dat"
				}
			}
		}
	}

	caption := fmt.Sprintf(
		"🐺 ᴜɴʟᴏᴄᴋᴇʀ | محتوای قفل‌شده\n"+
			"━━━━━━━━━━━━━━━━━\n"+
			"📢 منبع : %s\n"+
			"⏰ زمان : %s\n"+
			"📁 نوع : %s",
		sourceTitle, getTehranCurrentTime(), mediaType,
	)

	if strings.TrimSpace(msg.Message) != "" {
		caption += fmt.Sprintf("\n━━━━━━━━━━━━━━━━━\n📄 محتوا :\n%s", msg.Message)
	}

	if fileLoc != nil {
		tmpFile := filepath.Join("/tmp", fmt.Sprintf("wolf_prot_%d_%d%s", userID, msg.ID, fileExt))
		dCtx, dCancel := context.WithTimeout(ctx, 45*time.Second)
		err := downloadLocationToFile(dCtx, client, fileLoc, tmpFile)
		dCancel()

		if err == nil {
			defer os.Remove(tmpFile)
			u := uploader.NewUploader(client.API())
			uCtx, uCancel := context.WithTimeout(ctx, 45*time.Second)
			inputFile, upErr := u.FromPath(uCtx, tmpFile)
			uCancel()

			if upErr == nil {
				if mediaType == "عکس" {
					_, _ = client.API().MessagesSendMedia(ctx, &tg.MessagesSendMediaRequest{
						Peer:     &tg.InputPeerSelf{},
						Media:    &tg.InputMediaUploadedPhoto{File: inputFile},
						Message:  caption,
						RandomID: rand.Int63(),
					})
				} else {
					if mimeType == "" {
						mimeType = "application/octet-stream"
					}
					_, _ = client.API().MessagesSendMedia(ctx, &tg.MessagesSendMediaRequest{
						Peer:     &tg.InputPeerSelf{},
						Media:    &tg.InputMediaUploadedDocument{File: inputFile, MimeType: mimeType},
						Message:  caption,
						RandomID: rand.Int63(),
					})
				}
				sendBotAlert(userID, "🔓 دانلود خودکار محتوای قفل‌شده", fmt.Sprintf("منبع: %s (%s)", sourceTitle, mediaType))
				return
			}
		}
	}

	if strings.TrimSpace(msg.Message) != "" {
		sCtx, sCancel := context.WithTimeout(ctx, 10*time.Second)
		_, _ = client.API().MessagesSendMessage(sCtx, &tg.MessagesSendMessageRequest{
			Peer:     &tg.InputPeerSelf{},
			Message:  caption,
			RandomID: rand.Int63(),
		})
		sCancel()
		sendBotAlert(userID, "🔓 دانلود خودکار محتوای قفل‌شده", fmt.Sprintf("منبع: %s (متن)", sourceTitle))
	}
}

func HandleProtectedDownloadByReply(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, replyMsgID int, targetUserID int64) {
	dCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	res, err := client.API().MessagesGetMessages(dCtx, []tg.InputMessageClass{&tg.InputMessageID{ID: replyMsgID}})
	if err != nil {
		return
	}

	var targetMsg *tg.Message
	switch mSlice := res.(type) {
	case *tg.MessagesMessages:
		if len(mSlice.Messages) > 0 {
			targetMsg, _ = mSlice.Messages[0].(*tg.Message)
		}
	case *tg.MessagesMessagesSlice:
		if len(mSlice.Messages) > 0 {
			targetMsg, _ = mSlice.Messages[0].(*tg.Message)
		}
	case *tg.MessagesChannelMessages:
		if len(mSlice.Messages) > 0 {
			targetMsg, _ = mSlice.Messages[0].(*tg.Message)
		}
	}

	if targetMsg != nil {
		relayProtectedMessageToSaved(ctx, client, targetUserID, targetMsg, "ریپلای دستی")
	}
}

func HandleGhostMarkAsRead(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, msgID int) {
	gCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if ch, ok := inputPeer.(*tg.InputPeerChannel); ok {
		_, _ = client.API().ChannelsReadHistory(gCtx, &tg.ChannelsReadHistoryRequest{
			Channel: &tg.InputChannel{ChannelID: ch.ChannelID, AccessHash: ch.AccessHash},
			MaxID:   msgID,
		})
		return
	}

	_, _ = client.API().MessagesReadHistory(gCtx, &tg.MessagesReadHistoryRequest{
		Peer:  inputPeer,
		MaxID: msgID,
	})
}
