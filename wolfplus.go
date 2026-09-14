package main

import (
	"context"
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
)

// منوهای کیبورد ثابت ولف +
var (
	wolfPlusMenu = &tele.ReplyMarkup{ResizeKeyboard: true}
	antiDelMenu  = &tele.ReplyMarkup{ResizeKeyboard: true}
	editLogMenu  = &tele.ReplyMarkup{ResizeKeyboard: true}
	timerMenu    = &tele.ReplyMarkup{ResizeKeyboard: true}
	groupDelMenu = &tele.ReplyMarkup{ResizeKeyboard: true}
	targetMenu   = &tele.ReplyMarkup{ResizeKeyboard: true}

	btnWP_AntiDel  = wolfPlusMenu.Text("🗑 ضد حذف")
	btnWP_EditLog  = wolfPlusMenu.Text("📝 ادیت لاگر")
	btnWP_Timer    = wolfPlusMenu.Text("📸 رسانه تایمردار")
	btnWP_Group    = wolfPlusMenu.Text("👥 ضد حذف گروه")
	btnWP_Target   = wolfPlusMenu.Text("🎯 ردیاب مخاطب")
	btnWP_Refresh  = wolfPlusMenu.Text("🔄 بروزرسانی وضعیت")
	btnWP_BackMain = wolfPlusMenu.Text("🔙 بازگشت به منوی اصلی")

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

	btnTG_Add   = targetMenu.Text("➕ افزودن مخاطب")
	btnTG_List  = targetMenu.Text("📋 لیست مخاطبان")
	btnTG_Clear = targetMenu.Text("🗑 پاکسازی لیست اهداف")
	btnTG_Back  = targetMenu.Text("🔙 بازگشت به ولف +")
)

func init() {
	wolfPlusMenu.Reply(
		wolfPlusMenu.Row(btnWP_AntiDel, btnWP_EditLog),
		wolfPlusMenu.Row(btnWP_Timer, btnWP_Group),
		wolfPlusMenu.Row(btnWP_Target, btnWP_Refresh),
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
		targetMenu.Row(btnTG_Add, btnTG_List),
		targetMenu.Row(btnTG_Clear),
		targetMenu.Row(btnTG_Back),
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

func getWolfPlusStatus(userID int64) (antiDelete, editLogger, timerMedia bool, groupCount, targetCount int) {
	_ = db.QueryRow(`
		SELECT is_anti_delete_enabled, is_edit_logger_enabled, is_timer_media_enabled 
		FROM users WHERE id = ?
	`, userID).Scan(&antiDelete, &editLogger, &timerMedia)

	_ = db.QueryRow("SELECT COUNT(*) FROM wolf_antidel_groups WHERE owner_id = ?", userID).Scan(&groupCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM wolf_targets WHERE owner_id = ?", userID).Scan(&targetCount)
	return
}

func buildWolfPlusDashboardText(userID int64) string {
	antiDel, editLog, timerMed, groupCount, targetCount := getWolfPlusStatus(userID)

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
➖➖➖➖➖➖➖➖➖➖
💡 <i>برای ورود به تنظیمات و راهنمای هر قابلیت، گزینه مورد نظر را از کیبورد ثابت زیر لمس کنید:</i>`,
		statusIcon(antiDel), statusIcon(editLog), statusIcon(timerMed), groupCount, targetCount,
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
		return "⚠️ <i>در حال حاضر هیچ گروهی ثبت نشده و ضد حذف فقط در پیوی‌های خصوصی فعال است.</i>"
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

func RegisterWolfPlusHandlers(bot *tele.Bot) {
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

	bot.Handle(&btnWP_BackMain, func(c tele.Context) error {
		userID := c.Sender().ID
		wolfPlusStatesMu.Lock()
		delete(wolfPlusStates, userID)
		wolfPlusStatesMu.Unlock()
		return c.Send("🔙 <b>به منوی اصلی بازگشتید.</b>", getMainKeyboard(userID), tele.ModeHTML)
	})

	bot.Handle(&btnWP_AntiDel, func(c tele.Context) error {
		userID := c.Sender().ID
		antiDel, _, _, _, _ := getWolfPlusStatus(userID)
		statusStr := "🔴 خاموش"
		if antiDel {
			statusStr = "🟢 روشن"
		}

		text := fmt.Sprintf(`🗑 <b>مدیریت ضد حذف پیام‌ها (Anti-Delete)</b>
➖➖➖➖➖➖➖➖➖➖
📌 <b>وضعیت فعلی شما:</b> %s
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
این قابلیت کلیه پیام‌های متنی، عکس، صوت، ویدیو و فایل‌های دریافتی در <b>چت‌های خصوصی (پیوی)</b> را ذخیره می‌کند.
اگر فردی پیام خود را پاک کند، نسخه کامل آن بدون متن‌های اضافه به <b>Saved Messages</b> شما ارسال می‌شود.

⚠️ <i>پیام‌های ارسالی خود شما و ربات‌ها هرگز لاگ نمی‌شوند.</i>`, statusStr)
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

	bot.Handle(&btnWP_EditLog, func(c tele.Context) error {
		userID := c.Sender().ID
		_, editLog, _, _, _ := getWolfPlusStatus(userID)
		statusStr := "🔴 خاموش"
		if editLog {
			statusStr = "🟢 روشن"
		}

		text := fmt.Sprintf(`📝 <b>مدیریت لاگر ویرایش (Edit Logger)</b>
➖➖➖➖➖➖➖➖➖➖
📌 <b>وضعیت فعلی شما:</b> %s
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
اگر کاربری در گفتگوی خصوصی متنی را تغییر دهد، متن اولیه و متن جدید بلافاصله در <b>Saved Messages</b> ثبت می‌شود.`, statusStr)
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

	bot.Handle(&btnWP_Timer, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _, timerMed, _, _ := getWolfPlusStatus(userID)
		statusStr := "🔴 خاموش"
		if timerMed {
			statusStr = "🟢 روشن"
		}

		text := fmt.Sprintf(`📸 <b>مدیریت رسانه های تایمردار (View-Once)</b>
➖➖➖➖➖➖➖➖➖➖
📌 <b>وضعیت فعلی شما:</b> %s
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
عکس‌ها و ویدیوهای تایمردار دریافتی در پیوی به طور خودکار قبل از انقضا دانلود شده و یک نسخه دائمی از آن مستقیماً در <b>Saved Messages</b> شما ارسال می‌شود.`, statusStr)
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

	bot.Handle(&btnWP_Group, func(c tele.Context) error {
		userID := c.Sender().ID
		groupsList := getMonitoredGroupsText(userID)

		text := fmt.Sprintf(`👥 <b>مدیریت ضد حذف گروه‌ها (Group Anti-Delete)</b>
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
برای جلوگیری از شلوغی و ترافیک سرور، سلف در گروه‌ها خاموش است. در صورتی که می‌خواهید گروه خاصی بررسی شود، آیدی آن را ثبت کنید.
➖➖➖➖➖➖➖➖➖➖
%s`, groupsList)
		return c.Send(text, groupDelMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGD_Add, func(c tele.Context) error {
		userID := c.Sender().ID
		wolfPlusStatesMu.Lock()
		wolfPlusStates[userID] = "waiting_for_group_input"
		wolfPlusStatesMu.Unlock()

		return c.Send("➕ <b>افزودن گروه به لیست ضد حذف</b>\n\nلطفاً <b>آیدی عددی گروه</b> (مثلاً <code>-1001234567890</code>) یا <b>یوزرنیم گروه</b> (مثلاً <code>@MyGroup</code>) را ارسال کنید:", groupDelMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGD_Clear, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("DELETE FROM wolf_antidel_groups WHERE owner_id = ?", userID)
		return c.Send("🗑 <b>تمام گروه‌ها از لیست ضد حذف حذف شدند.</b>", groupDelMenu, tele.ModeHTML)
	})

	// ردیاب مخاطب خاص
	bot.Handle(&btnWP_Target, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _, _, _, targetCount := getWolfPlusStatus(userID)

		text := fmt.Sprintf(`🎯 <b>مدیریت ردیاب مخاطب خاص (Target Tracker)</b>
➖➖➖➖➖➖➖➖➖➖
📊 <b>تعداد اهداف تحت نظر:</b> <code>%d نفر</code>
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
این سیستم به طور مداوم و نامحسوس پروفایل شخص مورد نظر شما را بررسی می‌کند.
در صورت وقوع هر یک از تغییرات زیر، بلافاصله گزارشی به <b>Saved Messages</b> شما ارسال می‌شود:
▫️ تغییر نام یا نام خانوادگی
▫️ تغییر یوزرنیم (@username)
▫️ تغییر بیوگرافی (Bio)
▫️ تعویض عکس پروفایل`, targetCount)

		return c.Send(text, targetMenu, tele.ModeHTML)
	})

	bot.Handle(&btnTG_Add, func(c tele.Context) error {
		userID := c.Sender().ID
		wolfPlusStatesMu.Lock()
		wolfPlusStates[userID] = "waiting_for_target_input"
		wolfPlusStatesMu.Unlock()

		return c.Send("🎯 <b>ثبت مخاطب هدف:</b>\n\nلطفاً <b>یوزرنیم فرد مورد نظر</b> (با @ یا بدون @) را ارسال کنید:", targetMenu, tele.ModeHTML)
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

	if state == "waiting_for_group_input" {
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

		_ = c.Send("❌ <b>خطا در شناسایی گروه!</b>\nلطفاً <b>آیدی عددی</b> گروه (مثلاً <code>-100...</code>) را ارسال کنید.", groupDelMenu, tele.ModeHTML)
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
			_ = c.Send("❌ <b>سلف شما آنلاین نیست!</b> ابتدا سلف را فعال کنید.", targetMenu, tele.ModeHTML)
			wolfPlusStatesMu.Lock()
			delete(wolfPlusStates, userID)
			wolfPlusStatesMu.Unlock()
			return true
		}

		go func(input string) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			resolved, err := ub.Client.API().ContactsResolveUsername(ctx, input)
			if err == nil && len(resolved.Users) > 0 {
				if u, ok := resolved.Users[0].(*tg.User); ok {
					var bio string
					full, fErr := ub.Client.API().UsersGetFullUser(ctx, &tg.InputUser{
						UserID:     u.ID,
						AccessHash: u.AccessHash,
					})
					if fErr == nil {
						bio = full.FullUser.About
					}

					var photoID int64
					if p, ok := u.ProfilePhoto.(*tg.UserProfilePhoto); ok {
						photoID = p.PhotoID
					}

					_, _ = db.Exec(`
						INSERT INTO wolf_targets (owner_id, target_id, access_hash, first_name, last_name, username, bio, photo_id)
						VALUES (?, ?, ?, ?, ?, ?, ?, ?)
						ON DUPLICATE KEY UPDATE access_hash=VALUES(access_hash), first_name=VALUES(first_name),
						last_name=VALUES(last_name), username=VALUES(username), bio=VALUES(bio), photo_id=VALUES(photo_id)
					`, userID, u.ID, u.AccessHash, u.FirstName, u.LastName, u.Username, bio, photoID)

					_ = c.Send(fmt.Sprintf("✅ <b>مخاطب با موفقیت به ردیاب اضافه شد:</b>\n👤 <b>نام:</b> %s\n🆔 <b>آیدی:</b> <code>%d</code>", formatTelegramUser(u), u.ID), targetMenu, tele.ModeHTML)
					return
				}
			}
			_ = c.Send("❌ <b>مخاطب یافت نشد!</b> اطمینان حاصل کنید که یوزرنیم صحیح است.", targetMenu, tele.ModeHTML)
		}(targetInput)

		wolfPlusStatesMu.Lock()
		delete(wolfPlusStates, userID)
		wolfPlusStatesMu.Unlock()
		return true
	}

	return false
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
		chatName = "سوپرگروه"
	}

	if controllerBotID != 0 && (chatID == controllerBotID) {
		return
	}

	if !isPV {
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

	// ذخیره مستقیم مدیاهای تایمردار در پیام‌های ذخیره‌شده (Saved Messages)
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

		_, _ = db.Exec("UPDATE wolf_message_cache SET message_text = ? WHERE owner_id = ? AND message_id = ?", newText, ownerID, msg.ID)
	}
}

// موتور ردیاب تغییرات پروفایل مخاطب خاص
func StartTargetTrackerWorker(ctx context.Context, client *telegram.Client, ownerID int64) {
	ticker := time.NewTicker(3 * time.Minute)
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

					time.Sleep(time.Duration(rand.Intn(3)+2) * time.Second)

					reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
					full, err := client.API().UsersGetFullUser(reqCtx, &tg.InputUser{
						UserID:     targetID,
						AccessHash: accessHash,
					})
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

					newBio := full.FullUser.About
					var newPhotoID int64
					if p, ok := u.ProfilePhoto.(*tg.UserProfilePhoto); ok {
						newPhotoID = p.PhotoID
					}

					var changes []string

					if oldFirst != u.FirstName || oldLast != u.LastName {
						changes = append(changes, fmt.Sprintf("👤 تغییر نام:\nاز: %s %s\nبه: %s %s", oldFirst, oldLast, u.FirstName, u.LastName))
					}
					if oldUser != u.Username {
						changes = append(changes, fmt.Sprintf("🌐 تغییر یوزرنیم:\nاز: @%s\nبه: @%s", oldUser, u.Username))
					}
					if oldBio != newBio {
						changes = append(changes, fmt.Sprintf("📝 تغییر بیوگرافی:\nاز: %s\nبه: %s", oldBio, newBio))
					}
					if oldPhotoID != 0 && newPhotoID != 0 && oldPhotoID != newPhotoID {
						changes = append(changes, "📸 تغییر عکس پروفایل : عکس جدید تنظیم شد.")
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

						_, _ = db.Exec(`
							UPDATE wolf_targets 
							SET first_name=?, last_name=?, username=?, bio=?, photo_id=? 
							WHERE owner_id=? AND target_id=?
						`, u.FirstName, u.LastName, u.Username, newBio, newPhotoID, ownerID, targetID)
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
		if len(fileData) > 15*1024*1024 {
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
}
