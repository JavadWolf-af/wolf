package main

import (
	"context"
	"fmt"
	"html"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
)

// InitWolfPlusDB ساخت جداول و فیلدهای دیتابیس برای ولف +
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
		message_text TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (owner_id, chat_id, message_id),
		KEY idx_owner_msg (owner_id, message_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	// پاک‌سازی دوره‌ای کش پیام‌های قدیمی‌تر از ۲۴ ساعت
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		for range ticker.C {
			if db != nil {
				_, _ = db.Exec("DELETE FROM wolf_message_cache WHERE created_at < NOW() - INTERVAL 1 DAY")
			}
		}
	}()
}

// دریافت وضعیت فعال بودن هر ۳ قابلیت برای کاربر
func getWolfPlusStatus(userID int64) (antiDelete, editLogger, timerMedia bool) {
	_ = db.QueryRow(`
		SELECT is_anti_delete_enabled, is_edit_logger_enabled, is_timer_media_enabled 
		FROM users WHERE id = ?
	`, userID).Scan(&antiDelete, &editLogger, &timerMedia)
	return
}

// متن داشبورد ولف +
func buildWolfPlusDashboardText(userID int64) string {
	antiDel, editLog, timerMed := getWolfPlusStatus(userID)

	statusIcon := func(b bool) string {
		if b {
			return "🟢 روشن"
		}
		return "🔴 خاموش"
	}

	return fmt.Sprintf(`🐺 <b>پنل امکانات پیشرفته | ولف + (Wolf+)</b>
➖➖➖➖➖➖➖➖➖➖
📊 <b>وضعیت لحظه‌ای امکانات:</b>
▫️ 🗑 <b>ضد حذف (Anti-Delete):</b> %s
▫️ 📝 <b>لاگر ادیت (Edit Logger):</b> %s
▫️ 📸 <b>رسانه تایمردار (View-Once):</b> %s
➖➖➖➖➖➖➖➖➖➖
💡 <i>برای روشن/خاموش کردن هر قابلیت یا مطالعه راهنمای آن، از دکمه‌های شیشه‌ای زیر استفاده کنید:</i>`,
		statusIcon(antiDel), statusIcon(editLog), statusIcon(timerMed),
	)
}

// کیبورد شیشه‌ای منوی ولف +
func buildWolfPlusKeyboard(userID int64) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	antiDel, editLog, timerMed := getWolfPlusStatus(userID)

	btnAntiDelText := "🔴 ضد حذف: خاموش"
	if antiDel {
		btnAntiDelText = "🟢 ضد حذف: روشن"
	}

	btnEditLogText := "🔴 ادیت لاگر: خاموش"
	if editLog {
		btnEditLogText = "🟢 ادیت لاگر: روشن"
	}

	btnTimerText := "🔴 تایمردار: خاموش"
	if timerMed {
		btnTimerText = "🟢 تایمردار: روشن"
	}

	btnToggleAntiDel := menu.Data(btnAntiDelText, "wp_toggle", "antidel")
	btnGuideAntiDel := menu.Data("📖 راهنما", "wp_guide", "antidel")

	btnToggleEditLog := menu.Data(btnEditLogText, "wp_toggle", "editlog")
	btnGuideEditLog := menu.Data("📖 راهنما", "wp_guide", "editlog")

	btnToggleTimer := menu.Data(btnTimerText, "wp_toggle", "timer")
	btnGuideTimer := menu.Data("📖 راهنما", "wp_guide", "timer")

	btnRefresh := menu.Data("🔄 بروزرسانی وضعیت", "wp_refresh")

	menu.Inline(
		menu.Row(btnToggleAntiDel, btnGuideAntiDel),
		menu.Row(btnToggleEditLog, btnGuideEditLog),
		menu.Row(btnToggleTimer, btnGuideTimer),
		menu.Row(btnRefresh),
	)

	return menu
}

// ثبت هندلرهای دکمه‌های ولف + در ربات تلگرام
func RegisterWolfPlusHandlers(bot *tele.Bot) {
	// تغییر وضعیت (Toggle)
	bot.Handle(&tele.Btn{Unique: "wp_toggle"}, func(c tele.Context) error {
		userID := c.Sender().ID
		if IsUserBlocked(userID) {
			return c.Respond(&tele.CallbackResponse{Text: "❌ حساب شما مسدود است."})
		}

		target := c.Data()
		var column string
		var alertMsg string

		switch target {
		case "antidel":
			column = "is_anti_delete_enabled"
			alertMsg = "وضعیت ضد حذف تغییر کرد."
		case "editlog":
			column = "is_edit_logger_enabled"
			alertMsg = "وضعیت ثبت ادیت تغییر کرد."
		case "timer":
			column = "is_timer_media_enabled"
			alertMsg = "وضعیت رسانه تایمردار تغییر کرد."
		default:
			return c.Respond()
		}

		query := fmt.Sprintf("UPDATE users SET %s = NOT %s WHERE id = ?", column, column)
		_, _ = db.Exec(query, userID)

		_ = c.Edit(buildWolfPlusDashboardText(userID), buildWolfPlusKeyboard(userID), tele.ModeHTML)
		return c.Respond(&tele.CallbackResponse{Text: "✅ " + alertMsg})
	})

	// بخش راهنماها
	bot.Handle(&tele.Btn{Unique: "wp_guide"}, func(c tele.Context) error {
		userID := c.Sender().ID
		target := c.Data()

		backMenu := &tele.ReplyMarkup{}
		btnBack := backMenu.Data("🔙 بازگشت به منوی ولف +", "wp_back")
		backMenu.Inline(backMenu.Row(btnBack))

		var guideText string
		switch target {
		case "antidel":
			guideText = `🗑 <b>راهنمای جامع ضد حذف (Anti-Delete)</b>

🔹 <b>این قابلیت چه کاری انجام می‌دهد؟</b>
به محض دریافت هر پیام متنی در پی‌وی یا گروه‌ها، ربات آن را درون حافظه موقت دیتابیس کش می‌کند. اگر فرستنده پیام خود را به هر دلیلی «حذف دوطرفه» کند، سلف‌بات بلافاصله متوجه حذف شده و یک نسخه کامل از متن پاک‌شده را همراه با نام فرستنده، آیدی عددی و ساعت دقیق به <b>پیام‌های ذخیره‌شده (Saved Messages)</b> شما ارسال می‌کند.`

		case "editlog":
			guideText = `📝 <b>راهنمای جامع لاگر ادیت (Edit Logger)</b>

🔹 <b>این قابلیت چه کاری انجام می‌دهد؟</b>
اگر کاربری پیامی برای شما بفرستد و سپس آن را ویرایش (Edit) کند، ربات نسخه قبل از ادیت و نسخه جدید بعد از ادیت را با یکدیگر مقایسه می‌کند. سپس گزارشی شامل متن اولیه، متن تغییر‌یافته، مشخصات فرستنده و زمان دقیق ادیت را در <b>پیام‌های ذخیره‌شده (Saved Messages)</b> شما ثبت می‌کند.`

		case "timer":
			guideText = `📸 <b>راهنمای جامع رسانه تایمردار (View-Once)</b>

🔹 <b>این قابلیت چه کاری انجام می‌دهد؟</b>
تلگرام به افراد اجازه می‌دهد عکس یا ویدیوی زمان‌دار و یک‌بار مصرف بفرستند که پس از دیدن ناپدید می‌شود. با فعال بودن این گزینه، ربات فایل را به محض رسیدن و قبل از باز شدن توسط شما به طور خودکار دانلود کرده و نسخه بدون محدودیت زمانی آن را مستقیماً در همین ربات برای شما ارسال می‌کند.`
		}

		return c.Edit(guideText, backMenu, tele.ModeHTML)
	})

	// دکمه بازگشت به پنل اصلی ولف +
	bot.Handle(&tele.Btn{Unique: "wp_back"}, func(c tele.Context) error {
		userID := c.Sender().ID
		return c.Edit(buildWolfPlusDashboardText(userID), buildWolfPlusKeyboard(userID), tele.ModeHTML)
	})

	// رفرش کردن وضعیت
	bot.Handle(&tele.Btn{Unique: "wp_refresh"}, func(c tele.Context) error {
		userID := c.Sender().ID
		_ = c.Edit(buildWolfPlusDashboardText(userID), buildWolfPlusKeyboard(userID), tele.ModeHTML)
		return c.Respond(&tele.CallbackResponse{Text: "🔄 وضعیت به‌روز شد."})
	})
}

// شنود و کش پیام‌های ورودی و رسانه‌های تایمردار
func WolfPlusHandleIncoming(ctx context.Context, client *telegram.Client, bot *tele.Bot, userID int64, msg *tg.Message, e tg.Entities) {
	if msg.Out {
		return
	}

	// ۱. بررسی رسانه تایمردار
	if _, isUser := msg.PeerID.(*tg.PeerUser); isUser && msg.Media != nil {
		var timerEnabled bool
		_ = db.QueryRow("SELECT is_timer_media_enabled FROM users WHERE id = ?", userID).Scan(&timerEnabled)
		if timerEnabled {
			go func() {
				dCtx, dCancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer dCancel()
				downloadAndRelayTTL(dCtx, client, bot, userID, msg, e)
			}()
		}
	}

	// ۲. کش کردن متن پیام‌ها جهت مقایسه در ضد حذف و ادیت
	text := strings.TrimSpace(msg.Message)
	if text != "" {
		chatID := int64(0)
		switch p := msg.PeerID.(type) {
		case *tg.PeerUser:
			chatID = p.UserID
		case *tg.PeerChat:
			chatID = p.ChatID
		case *tg.PeerChannel:
			chatID = p.ChannelID
		}

		senderID := int64(0)
		senderName := "ناشناس"
		if fromUser, ok := msg.FromID.(*tg.PeerUser); ok {
			senderID = fromUser.UserID
		} else if peerUser, ok := msg.PeerID.(*tg.PeerUser); ok {
			senderID = peerUser.UserID
		}

		if senderID != 0 {
			if u, exists := e.Users[senderID]; exists {
				if u.FirstName != "" || u.LastName != "" {
					senderName = strings.TrimSpace(u.FirstName + " " + u.LastName)
				} else if u.Username != "" {
					senderName = "@" + u.Username
				}
			}
		}

		_, _ = db.Exec(`
			INSERT INTO wolf_message_cache (owner_id, chat_id, message_id, sender_id, sender_name, message_text)
			VALUES (?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE message_text = VALUES(message_text), sender_name = VALUES(sender_name)
		`, userID, chatID, msg.ID, senderID, senderName, text)
	}
}

// ثبت رویدادهای حذف و ادیت در دیسپچر تلگرام
func RegisterWolfPlusDispatcher(dispatcher *tg.UpdateDispatcher, client *telegram.Client, userID int64) {
	// رویداد حذف پیام در پیوی و گروه‌های عادی
	dispatcher.OnDeleteMessages(func(ctx context.Context, e tg.Entities, u *tg.UpdateDeleteMessages) error {
		go handleDeletedMessages(client, userID, u.Messages)
		return nil
	})

	// رویداد حذف پیام در سوپرگروه‌ها و کانال‌ها
	dispatcher.OnDeleteChannelMessages(func(ctx context.Context, e tg.Entities, u *tg.UpdateDeleteChannelMessages) error {
		go handleDeletedMessages(client, userID, u.Messages)
		return nil
	})

	// رویداد ادیت پیام در پیوی و گروه‌ها
	dispatcher.OnEditMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateEditMessage) error {
		go handleEditedMessage(client, userID, u.Message)
		return nil
	})

	// رویداد ادیت پیام در کانال‌ها و سوپرگروه‌ها
	dispatcher.OnEditChannelMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateEditChannelMessage) error {
		go handleEditedMessage(client, userID, u.Message)
		return nil
	})
}

// مدیریت نوتیفیکیشن حذف پیام به Saved Messages
func handleDeletedMessages(client *telegram.Client, ownerID int64, msgIDs []int) {
	var antiDelete bool
	_ = db.QueryRow("SELECT is_anti_delete_enabled FROM users WHERE id = ?", ownerID).Scan(&antiDelete)
	if !antiDelete || len(msgIDs) == 0 {
		return
	}

	for _, msgID := range msgIDs {
		var chatID, senderID int64
		var senderName, msgText string
		err := db.QueryRow("SELECT chat_id, sender_id, sender_name, message_text FROM wolf_message_cache WHERE owner_id = ? AND message_id = ?", ownerID, msgID).Scan(&chatID, &senderID, &senderName, &msgText)

		if err == nil && msgText != "" {
			report := fmt.Sprintf(
				"🗑 #حذف_پیام (Anti-Delete)\n\n"+
					"👤 <b>فرستنده:</b> %s\n"+
					"🆔 <b>آیدی عددی:</b> <code>%d</code>\n"+
					"💬 <b>شناسه چت:</b> <code>%d</code>\n"+
					"⏰ <b>زمان حذف:</b> %s\n\n"+
					"📄 <b>متن پاک‌شده:</b>\n%s",
				html.EscapeString(senderName), senderID, chatID, time.Now().Format("15:04:05"), html.EscapeString(msgText),
			)

			cTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, _ = client.API().MessagesSendMessage(cTimeout, &tg.MessagesSendMessageRequest{
				Peer:     &tg.InputPeerSelf{},
				Message:  report,
				RandomID: time.Now().UnixNano(),
			})
			cancel()

			_, _ = db.Exec("DELETE FROM wolf_message_cache WHERE owner_id = ? AND message_id = ?", ownerID, msgID)
		}
	}
}

// مدیریت نوتیفیکیشن ویرایش پیام به Saved Messages
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
	var oldText, senderName string
	var senderID, chatID int64
	err := db.QueryRow("SELECT chat_id, sender_id, sender_name, message_text FROM wolf_message_cache WHERE owner_id = ? AND message_id = ?", ownerID, msg.ID).Scan(&chatID, &senderID, &senderName, &oldText)

	if err == nil && oldText != "" && oldText != newText {
		report := fmt.Sprintf(
			"📝 #ویرایش_پیام (Edit Logger)\n\n"+
				"👤 <b>فرستنده:</b> %s\n"+
				"🆔 <b>آیدی عددی:</b> <code>%d</code>\n"+
				"💬 <b>شناسه چت:</b> <code>%d</code>\n"+
				"⏰ <b>زمان ویرایش:</b> %s\n\n"+
				"📌 <b>متن اولیه:</b>\n%s\n\n"+
				"✏️ <b>متن جدید:</b>\n%s",
			html.EscapeString(senderName), senderID, chatID, time.Now().Format("15:04:05"), html.EscapeString(oldText), html.EscapeString(newText),
		)

		cTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, _ = client.API().MessagesSendMessage(cTimeout, &tg.MessagesSendMessageRequest{
			Peer:     &tg.InputPeerSelf{},
			Message:  report,
			RandomID: time.Now().UnixNano(),
		})
		cancel()

		_, _ = db.Exec("UPDATE wolf_message_cache SET message_text = ? WHERE owner_id = ? AND message_id = ?", newText, ownerID, msg.ID)
	}
}

// دانلود و ارسال رسانه‌های تایمردار به ربات
func downloadAndRelayTTL(ctx context.Context, client *telegram.Client, bot *tele.Bot, targetUserID int64, msg *tg.Message, e tg.Entities) {
	var (
		isTTL      bool
		ttlSeconds int
		mediaType  string
	)

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
	senderName := "ناشناس"
	usernameStr := "ثبت نشده"

	if fromUser, ok := msg.FromID.(*tg.PeerUser); ok {
		senderID = fromUser.UserID
	} else if peerUser, ok := msg.PeerID.(*tg.PeerUser); ok {
		senderID = peerUser.UserID
	}

	if senderID != 0 {
		if u, exists := e.Users[senderID]; exists {
			if u.FirstName != "" || u.LastName != "" {
				senderName = strings.TrimSpace(u.FirstName + " " + u.LastName)
			}
			if u.Username != "" {
				usernameStr = "@" + u.Username
			}
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
		var thumbSize string
		for _, s := range photo.Sizes {
			switch sz := s.(type) {
			case *tg.PhotoSize:
				thumbSize = sz.Type
			case *tg.PhotoSizeProgressive:
				thumbSize = sz.Type
			}
		}
		if thumbSize == "" {
			thumbSize = "x"
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
			log.Printf("❌ UploadGetFile error: %v", err)
			break
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
		if len(fileData) > 50*1024*1024 {
			break
		}
	}

	if len(fileData) == 0 {
		return
	}

	tmpFile := filepath.Join("/tmp", fmt.Sprintf("wolf_ttl_%d%s", time.Now().UnixNano(), fileExt))
	if err := os.WriteFile(tmpFile, fileData, 0600); err != nil {
		return
	}
	defer os.Remove(tmpFile)

	caption := fmt.Sprintf(
		"📸 <b>رسانه تایمردار ذخیره شد! 🐺</b>\n\n"+
			"👤 <b>فرستنده:</b> %s (%s)\n"+
			"🆔 <b>آیدی:</b> <code>%d</code>\n"+
			"⏱ <b>تایمر:</b> %d ثانیه\n"+
			"🗂 <b>نوع:</b> %s",
		html.EscapeString(senderName), html.EscapeString(usernameStr), senderID, ttlSeconds,
		func() string {
			if mediaType == "photo" {
				return "عکس"
			}
			return "ویدیو"
		}(),
	)

	if mediaType == "photo" {
		p := &tele.Photo{
			File:    tele.FromDisk(tmpFile),
			Caption: caption,
		}
		_, _ = bot.Send(&tele.User{ID: targetUserID}, p, tele.ModeHTML)
	} else {
		v := &tele.Video{
			File:    tele.FromDisk(tmpFile),
			Caption: caption,
		}
		_, _ = bot.Send(&tele.User{ID: targetUserID}, v, tele.ModeHTML)
	}
}
