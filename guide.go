package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"github.com/joho/godotenv"
	tele "gopkg.in/telebot.v3"
)

// ==========================================
// 1. کیبوردها و دکمه‌های بخش راهنما
// ==========================================

var (
	guideMenu           = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideClockMenu      = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideEmojiMenu      = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideBioMenu        = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideFriendMenu     = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideEnemyMenu      = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideFontMenu       = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideActionMenu     = &tele.ReplyMarkup{ResizeKeyboard: true}
	guidePurgeMenu      = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideTimerMenu      = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideAutoReactMenu  = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideTranslatorMenu = &tele.ReplyMarkup{ResizeKeyboard: true}
	guidePVLockMenu     = &tele.ReplyMarkup{ResizeKeyboard: true}
	guidePVMenu         = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideGroupMenu      = &tele.ReplyMarkup{ResizeKeyboard: true}

	btnGClock      = guideMenu.Text("⏱ ساعت زنده")
	btnGEmoji      = guideMenu.Text("🎭 اموجی رندوم")
	btnGBio        = guideMenu.Text("📝 بیوگرافی هوشمند")
	btnGFont       = guideMenu.Text("✒️ خوشنویسی")
	btnGFriend     = guideMenu.Text("🌸 دوست")
	btnGEnemy      = guideMenu.Text("⚔️ دشمن")
	btnGAction     = guideMenu.Text("🎬 اکشن‌ها")
	btnGPurge      = guideMenu.Text("🗑 پاکسازی")
	btnGTimer      = guideMenu.Text("⏳ تایمر")
	btnGAutoReact  = guideMenu.Text("🔥 ری اکشن خودکار")
	btnGTranslator = guideMenu.Text("🌍 مترجم در لحظه")
	btnGPVLock     = guideMenu.Text("🔐 قفل پیوی")
	btnGPV         = guideMenu.Text("📩 پیوی همه")
	btnGGroup      = guideMenu.Text("👥 گروه همه")
	btnGBackMain   = guideMenu.Text("🔙 بازگشت به منوی اصلی")

	btnClockOn   = guideClockMenu.Text("🟢 روشن کردن ساعت")
	btnClockOff  = guideClockMenu.Text("🔴 خاموش کردن ساعت")
	btnClockBack = guideClockMenu.Text("🔙 بازگشت به راهنما")

	btnEmojiOn   = guideEmojiMenu.Text("🟢 روشن کردن اموجی")
	btnEmojiOff  = guideEmojiMenu.Text("🔴 خاموش کردن اموجی")
	btnEmojiBack = guideEmojiMenu.Text("🔙 بازگشت به راهنما")

	btnBioOn   = guideBioMenu.Text("🟢 روشن کردن بیو (رندوم)")
	btnBioOff  = guideBioMenu.Text("🔴 خاموش کردن بیو")
	btnBioBack = guideBioMenu.Text("🔙 بازگشت به راهنما")

	btnFriendList  = guideFriendMenu.Text("📋 لیست دوستان")
	btnFriendClear = guideFriendMenu.Text("🗑 پاکسازی دوستان")
	btnFriendBack  = guideFriendMenu.Text("🔙 بازگشت به راهنما")

	btnEnemyList  = guideEnemyMenu.Text("📋 لیست دشمنان")
	btnEnemyClear = guideEnemyMenu.Text("🗑 پاکسازی دشمنان")
	btnEnemyBack  = guideEnemyMenu.Text("🔙 بازگشت به راهنما")

	btnFontOn         = guideFontMenu.Text("🟢 روشن کردن خوشنویسی")
	btnFontOff        = guideFontMenu.Text("🔴 خاموش کردن خوشنویسی")
	btnFontBoldItalic = guideFontMenu.Text("✨ بولد ایتالیک")
	btnFontBold       = guideFontMenu.Text("🖋 بولد (پیش‌فرض)")
	btnFontItalic     = guideFontMenu.Text("🖊 ایتالیک")
	btnFontUnderline  = guideFontMenu.Text("📜 زیر خط")
	btnFontStrike     = guideFontMenu.Text("❌ خط خورده")
	btnFontMono       = guideFontMenu.Text("💻 مونو")
	btnFontSpoiler    = guideFontMenu.Text("🕵️ اسپویل")
	btnFontBack       = guideFontMenu.Text("🔙 بازگشت به راهنما")

	btnActionBack     = guideActionMenu.Text("🔙 بازگشت به راهنما")
	btnPurgeBack      = guidePurgeMenu.Text("🔙 بازگشت به راهنما")
	btnTimerBack      = guideTimerMenu.Text("🔙 بازگشت به راهنما")
	btnAutoReactBack  = guideAutoReactMenu.Text("🔙 بازگشت به راهنما")
	btnTranslatorBack = guideTranslatorMenu.Text("🔙 بازگشت به راهنما")
	btnPVLockBack     = guidePVLockMenu.Text("🔙 بازگشت به راهنما")
	btnPVBack         = guidePVMenu.Text("🔙 بازگشت به راهنما")
	btnGroupBack      = guideGroupMenu.Text("🔙 بازگشت به راهنما")
)

func initGuideMarkups() {
	guideMenu.Reply(
		guideMenu.Row(btnGClock, btnGEmoji),
		guideMenu.Row(btnGBio, btnGFont),
		guideMenu.Row(btnGFriend, btnGEnemy),
		guideMenu.Row(btnGAction, btnGPurge),
		guideMenu.Row(btnGTimer, btnGAutoReact),
		guideMenu.Row(btnGTranslator, btnGPVLock),
		guideMenu.Row(btnGPV, btnGGroup),
		guideMenu.Row(btnGBackMain),
	)

	guideClockMenu.Reply(
		guideClockMenu.Row(btnClockOn, btnClockOff),
		guideClockMenu.Row(btnClockBack),
	)

	guideEmojiMenu.Reply(
		guideEmojiMenu.Row(btnEmojiOn, btnEmojiOff),
		guideEmojiMenu.Row(btnEmojiBack),
	)

	guideBioMenu.Reply(
		guideBioMenu.Row(btnBioOn, btnBioOff),
		guideBioMenu.Row(btnBioBack),
	)

	guideFriendMenu.Reply(
		guideFriendMenu.Row(btnFriendList, btnFriendClear),
		guideFriendMenu.Row(btnFriendBack),
	)

	guideEnemyMenu.Reply(
		guideEnemyMenu.Row(btnEnemyList, btnEnemyClear),
		guideEnemyMenu.Row(btnEnemyBack),
	)

	guideFontMenu.Reply(
		guideFontMenu.Row(btnFontOn, btnFontOff),
		guideFontMenu.Row(btnFontBold, btnFontBoldItalic),
		guideFontMenu.Row(btnFontItalic, btnFontUnderline),
		guideFontMenu.Row(btnFontStrike, btnFontMono),
		guideFontMenu.Row(btnFontSpoiler),
		guideFontMenu.Row(btnFontBack),
	)

	guideActionMenu.Reply(guideActionMenu.Row(btnActionBack))
	guidePurgeMenu.Reply(guidePurgeMenu.Row(btnPurgeBack))
	guideTimerMenu.Reply(guideTimerMenu.Row(btnTimerBack))
	guideAutoReactMenu.Reply(guideAutoReactMenu.Row(btnAutoReactBack))
	guideTranslatorMenu.Reply(guideTranslatorMenu.Row(btnTranslatorBack))
	guidePVLockMenu.Reply(guidePVLockMenu.Row(btnPVLockBack))
	guidePVMenu.Reply(guidePVMenu.Row(btnPVBack))
	guideGroupMenu.Reply(guideGroupMenu.Row(btnGroupBack))
}

func buildGuideDashboardText(userID int64) string {
	var isClock, isEmoji, isBio, isFont bool
	var bioMode string
	_ = db.QueryRow("SELECT is_clock_enabled, is_emoji_enabled, is_bio_enabled, bio_mode, is_font_enabled FROM users WHERE id = ?", userID).Scan(&isClock, &isEmoji, &isBio, &bioMode, &isFont)
	var friendCount, enemyCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM wolf_friends WHERE owner_id = ?", userID).Scan(&friendCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM wolf_enemies WHERE owner_id = ?", userID).Scan(&enemyCount)

	clockStatus := "🔴 خاموش"
	if isClock {
		clockStatus = "🟢 روشن"
	}
	emojiStatus := "🔴 خاموش"
	if isEmoji {
		emojiStatus = "🟢 روشن"
	}
	bioStatus := "🔴 خاموش"
	if isBio {
		if bioMode == "custom" {
			bioStatus = "🟢 روشن (دستی)"
		} else {
			bioStatus = "🟢 روشن (رندوم)"
		}
	}
	fontStatus := "🔴 خاموش"
	if isFont {
		fontStatus = "🟢 روشن"
	}
	pvLockStatus := "🔴 خاموش"
	if IsPVLockEnabled(userID) {
		pvLockStatus = "🟢 روشن (امنیتی)"
	}

	return fmt.Sprintf(`📚 <b>بخش راهنما و امکانات سلف ولف 🐺</b>
➖➖➖➖➖➖➖➖➖➖
📊 <b>وضعیت لحظه‌ای قابلیت‌ها:</b>
▫️ ⏱ <b>ساعت زنده:</b> %s
▫️ 🎭 <b>اموجی رندوم:</b> %s
▫️ 📝 <b>بیوگرافی هوشمند:</b> %s
▫️ ✒️ <b>خوشنویسی پیام‌ها:</b> %s
▫️ 🌸 <b>سیستم دوست:</b> <code>%d نفر</code>
▫️ ⚔️ <b>سیستم دشمن:</b> <code>%d نفر</code>
▫️ 🎬 <b>اکشن‌های جعلی:</b> فعال
▫️ 🗑 <b>پاکسازی پیام‌ها:</b> فعال
▫️ ⏳ <b>تایمر زنده:</b> فعال
▫️ 🔥 <b>ری اکشن خودکار:</b> فعال
▫️ 🌍 <b>مترجم زنده:</b> فعال
▫️ 🔐 <b>قفل پیوی ضد مزاحم:</b> %s
➖➖➖➖➖➖➖➖➖➖
💡 <i>جهت مطالعه راهنما و تنظیم هر قابلیت، از کیبورد زیر انتخاب کنید:</i>`,
		clockStatus, emojiStatus, bioStatus, fontStatus, friendCount, enemyCount, pvLockStatus,
	)
}

func RegisterGuideHandlers(bot *tele.Bot) {
	initGuideMarkups()

	bot.Handle(&btnGuide, func(c tele.Context) error {
		userID := c.Sender().ID
		if IsUserBlocked(userID) {
			return c.Send("❌ حساب شما مسدود است.")
		}
		selfStatus := GetUserSelfStatus(userID)
		if selfStatus == "خرید نداشته" || selfStatus == "خروج" {
			return c.Send("❌ <b>دسترسی محدود!</b>\n\nبخش راهنما فقط برای کاربرانی که اشتراک سلف را خریداری کرده‌اند فعال می‌باشد.", getKeyboard(userID), tele.ModeHTML)
		}
		return c.Send(buildGuideDashboardText(userID), guideMenu, tele.ModeHTML)
	})

	// ساعت زنده
	bot.Handle(&btnGClock, func(c tele.Context) error {
		userID := c.Sender().ID
		var isClock bool
		_ = db.QueryRow("SELECT is_clock_enabled FROM users WHERE id = ?", userID).Scan(&isClock)
		statusStr := "🔴 خاموش"
		if isClock {
			statusStr = "🟢 روشن"
		}
		text := fmt.Sprintf(`⏱ <b>راهنمای ساعت زنده روی پروفایل</b>
➖➖➖➖➖➖➖➖➖➖
📌 <b>وضعیت فعلی:</b> %s
➖➖➖➖➖➖➖➖➖➖
📖 ساعت رسمی تهران به صورت زنده و با فونت بولد روی فامیلی اکانت قرار می‌گیرد.`, statusStr)
		return c.Send(text, guideClockMenu, tele.ModeHTML)
	})

	bot.Handle(&btnClockOn, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil {
			return c.Send("❌ <b>سلف شما آنلاین نیست!</b>", guideClockMenu, tele.ModeHTML)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleClockOn(ctx, userID, ub.Client)
		return c.Send("🟢 <b>ساعت زنده فعال شد.</b>", guideClockMenu, tele.ModeHTML)
	})

	bot.Handle(&btnClockOff, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil {
			return c.Send("❌ <b>سلف شما آنلاین نیست!</b>", guideClockMenu, tele.ModeHTML)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleClockOff(ctx, userID, ub.Client)
		return c.Send("🔴 <b>ساعت زنده خاموش شد.</b>", guideClockMenu, tele.ModeHTML)
	})

	// اموجی رندوم
	bot.Handle(&btnGEmoji, func(c tele.Context) error {
		userID := c.Sender().ID
		var isEmoji bool
		_ = db.QueryRow("SELECT is_emoji_enabled FROM users WHERE id = ?", userID).Scan(&isEmoji)
		statusStr := "🔴 خاموش"
		if isEmoji {
			statusStr = "🟢 روشن"
		}
		return c.Send(fmt.Sprintf("🎭 <b>اموجی رندوم</b>\nوضعیت: %s", statusStr), guideEmojiMenu, tele.ModeHTML)
	})

	bot.Handle(&btnEmojiOn, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil {
			return c.Send("❌ سلف آنلاین نیست.", guideEmojiMenu, tele.ModeHTML)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleEmojiOn(ctx, userID, ub.Client)
		return c.Send("🟢 اموجی روشن شد.", guideEmojiMenu, tele.ModeHTML)
	})

	bot.Handle(&btnEmojiOff, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil {
			return c.Send("❌ سلف آنلاین نیست.", guideEmojiMenu, tele.ModeHTML)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleEmojiOff(ctx, userID, ub.Client)
		return c.Send("🔴 اموجی خاموش شد.", guideEmojiMenu, tele.ModeHTML)
	})

	// بیوگرافی هوشمند
	bot.Handle(&btnGBio, func(c tele.Context) error {
		userID := c.Sender().ID
		var isBio bool
		_ = db.QueryRow("SELECT is_bio_enabled FROM users WHERE id = ?", userID).Scan(&isBio)
		statusStr := "🔴 خاموش"
		if isBio {
			statusStr = "🟢 روشن"
		}
		return c.Send(fmt.Sprintf("📝 <b>بیوگرافی هوشمند</b>\nوضعیت: %s", statusStr), guideBioMenu, tele.ModeHTML)
	})

	bot.Handle(&btnBioOn, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil {
			return c.Send("❌ سلف آنلاین نیست.", guideBioMenu, tele.ModeHTML)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleBioOn(ctx, userID, ub.Client)
		return c.Send("🟢 بیوگرافی فعال شد.", guideBioMenu, tele.ModeHTML)
	})

	bot.Handle(&btnBioOff, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil {
			return c.Send("❌ سلف آنلاین نیست.", guideBioMenu, tele.ModeHTML)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleBioOff(ctx, userID, ub.Client)
		return c.Send("🔴 بیوگرافی خاموش شد.", guideBioMenu, tele.ModeHTML)
	})

	// دوست و دشمن
	bot.Handle(&btnGFriend, func(c tele.Context) error {
		var cnt int
		_ = db.QueryRow("SELECT COUNT(*) FROM wolf_friends WHERE owner_id = ?", c.Sender().ID).Scan(&cnt)
		return c.Send(fmt.Sprintf("🌸 <b>سیستم دوست</b>\nتعداد: %d نفر", cnt), guideFriendMenu, tele.ModeHTML)
	})

	bot.Handle(&btnFriendList, func(c tele.Context) error {
		rows, err := db.Query("SELECT friend_id, friend_name FROM wolf_friends WHERE owner_id = ?", c.Sender().ID)
		if err != nil {
			return c.Send("❌ خطا در دریافت لیست.", guideFriendMenu, tele.ModeHTML)
		}
		defer rows.Close()
		var list []string
		idx := 1
		for rows.Next() {
			var fid int64
			var fname string
			if err := rows.Scan(&fid, &fname); err == nil {
				list = append(list, fmt.Sprintf("%d. %s (<code>%d</code>)", idx, fname, fid))
				idx++
			}
		}
		txt := "📋 <b>لیست دوستان:</b>\n\n" + strings.Join(list, "\n")
		if len(list) == 0 {
			txt = "⚠️ لیست دوستان خالی است."
		}
		return c.Send(txt, guideFriendMenu, tele.ModeHTML)
	})

	bot.Handle(&btnFriendClear, func(c tele.Context) error {
		_, _ = db.Exec("DELETE FROM wolf_friends WHERE owner_id = ?", c.Sender().ID)
		clearFriendsCache(c.Sender().ID)
		return c.Send("🗑 لیست دوستان پاکسازی شد.", guideFriendMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGEnemy, func(c tele.Context) error {
		var cnt int
		_ = db.QueryRow("SELECT COUNT(*) FROM wolf_enemies WHERE owner_id = ?", c.Sender().ID).Scan(&cnt)
		return c.Send(fmt.Sprintf("⚔️ <b>سیستم دشمن</b>\nتعداد: %d نفر", cnt), guideEnemyMenu, tele.ModeHTML)
	})

	bot.Handle(&btnEnemyList, func(c tele.Context) error {
		rows, err := db.Query("SELECT enemy_id, enemy_name FROM wolf_enemies WHERE owner_id = ?", c.Sender().ID)
		if err != nil {
			return c.Send("❌ خطا در دریافت لیست.", guideEnemyMenu, tele.ModeHTML)
		}
		defer rows.Close()
		var list []string
		idx := 1
		for rows.Next() {
			var eid int64
			var ename string
			if err := rows.Scan(&eid, &ename); err == nil {
				list = append(list, fmt.Sprintf("%d. %s (<code>%d</code>)", idx, ename, eid))
				idx++
			}
		}
		txt := "⚔️ <b>لیست دشمنان:</b>\n\n" + strings.Join(list, "\n")
		if len(list) == 0 {
			txt = "⚠️ لیست دشمنان خالی است."
		}
		return c.Send(txt, guideEnemyMenu, tele.ModeHTML)
	})

	bot.Handle(&btnEnemyClear, func(c tele.Context) error {
		_, _ = db.Exec("DELETE FROM wolf_enemies WHERE owner_id = ?", c.Sender().ID)
		clearEnemiesCache(c.Sender().ID)
		return c.Send("🗑 لیست دشمنان پاکسازی شد.", guideEnemyMenu, tele.ModeHTML)
	})

	// خوشنویسی
	bot.Handle(&btnGFont, func(c tele.Context) error {
		en, mode := getFontSetting(c.Sender().ID)
		st := "🔴 خاموش"
		if en {
			st = "🟢 روشن"
		}
		return c.Send(fmt.Sprintf("✒️ <b>تنظیمات خوشنویسی</b>\nوضعیت: %s\nفونت: %s", st, mode), guideFontMenu, tele.ModeHTML)
	})

	bot.Handle(&btnFontOn, func(c tele.Context) error {
		_, _ = db.Exec("UPDATE users SET is_font_enabled = TRUE WHERE id = ?", c.Sender().ID)
		_, m := getFontSetting(c.Sender().ID)
		updateFontCache(c.Sender().ID, true, m)
		return c.Send("🟢 خوشنویسی روشن شد.", guideFontMenu, tele.ModeHTML)
	})

	bot.Handle(&btnFontOff, func(c tele.Context) error {
		_, _ = db.Exec("UPDATE users SET is_font_enabled = FALSE WHERE id = ?", c.Sender().ID)
		_, m := getFontSetting(c.Sender().ID)
		updateFontCache(c.Sender().ID, false, m)
		return c.Send("🔴 خوشنویسی خاموش شد.", guideFontMenu, tele.ModeHTML)
	})

	setFont := func(mode, label string) tele.HandlerFunc {
		return func(c tele.Context) error {
			_, _ = db.Exec("UPDATE users SET font_mode = ?, is_font_enabled = TRUE WHERE id = ?", mode, c.Sender().ID)
			updateFontCache(c.Sender().ID, true, mode)
			return c.Send(fmt.Sprintf("✅ فونت به «%s» تنظیم و روشن شد.", label), guideFontMenu, tele.ModeHTML)
		}
	}
	bot.Handle(&btnFontBold, setFont("bold", "بولد"))
	bot.Handle(&btnFontBoldItalic, setFont("bold_italic", "بولد ایتالیک"))
	bot.Handle(&btnFontItalic, setFont("italic", "ایتالیک"))
	bot.Handle(&btnFontUnderline, setFont("underline", "زیر خط"))
	bot.Handle(&btnFontStrike, setFont("strike", "خط خورده"))
	bot.Handle(&btnFontMono, setFont("mono", "مونو"))
	bot.Handle(&btnFontSpoiler, setFont("spoiler", "اسپویل"))

	// اکشن‌ها، پاکسازی، تایمر، ری اکشن
	bot.Handle(&btnGAction, func(c tele.Context) error {
		return c.Send("🎬 <b>اکشن‌ها:</b>\nدستورات: تایپینگ 20، ضبط صدا 20 و...", guideActionMenu, tele.ModeHTML)
	})
	bot.Handle(&btnGPurge, func(c tele.Context) error {
		return c.Send("🗑 <b>پاکسازی:</b>\nدستور: <code>پاکشو 20</code> یا ریپلای با <code>پاکشو</code>", guidePurgeMenu, tele.ModeHTML)
	})
	bot.Handle(&btnGTimer, func(c tele.Context) error {
		return c.Send("⏳ <b>تایمر:</b>\nدستور: <code>تایمر 10</code>", guideTimerMenu, tele.ModeHTML)
	})
	bot.Handle(&btnGAutoReact, func(c tele.Context) error {
		return c.Send("🔥 <b>ری‌اکشن:</b>\nبا ریپلای: <code>ری اکشن 🔥</code>", guideAutoReactMenu, tele.ModeHTML)
	})
	bot.Handle(&btnGTranslator, func(c tele.Context) error {
		return c.Send("🌍 <b>مترجم:</b>\nبا ریپلای: <code>ترجمه</code> یا <code>انگلیسی شو</code>", guideTranslatorMenu, tele.ModeHTML)
	})
	bot.Handle(&btnGPVLock, func(c tele.Context) error {
		return c.Send("🔐 <b>قفل پیوی:</b>\nپیش‌فرض خاموش است. روشن کردن: <code>قفل پیوی روشن</code>", guidePVLockMenu, tele.ModeHTML)
	})

	// دکمه‌های بازگشت
	backHandler := func(c tele.Context) error {
		return c.Send(buildGuideDashboardText(c.Sender().ID), guideMenu, tele.ModeHTML)
	}
	bot.Handle(&btnClockBack, backHandler)
	bot.Handle(&btnEmojiBack, backHandler)
	bot.Handle(&btnBioBack, backHandler)
	bot.Handle(&btnFriendBack, backHandler)
	bot.Handle(&btnEnemyBack, backHandler)
	bot.Handle(&btnFontBack, backHandler)
	bot.Handle(&btnActionBack, backHandler)
	bot.Handle(&btnPurgeBack, backHandler)
	bot.Handle(&btnTimerBack, backHandler)
	bot.Handle(&btnAutoReactBack, backHandler)
	bot.Handle(&btnTranslatorBack, backHandler)
	bot.Handle(&btnPVLockBack, backHandler)
	bot.Handle(&btnPVBack, backHandler)
	bot.Handle(&btnGroupBack, backHandler)
	bot.Handle(&btnGBackMain, func(c tele.Context) error {
		return c.Send("🔙 <b>منوی اصلی:</b>", getMainKeyboard(c.Sender().ID), tele.ModeHTML)
	})
}

// ==========================================
// 2. سیستم مترجم زنده هوشمند (Groq Qwen)
// ==========================================

type GroqRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float32   `json:"temperature"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GroqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func getLanguageSettings(cmd string) (string, string, bool) {
	if cmd == "ترجمه" || cmd == "ترجمه کن" {
		return "Persian (Farsi)", "فارسی", true
	}
	if strings.HasSuffix(cmd, " شو") {
		langPart := strings.TrimSpace(strings.TrimSuffix(cmd, " شو"))
		switch langPart {
		case "انگلیسی":
			return "English", "انگلیسی", true
		case "روسی":
			return "Russian", "روسی", true
		case "کره ای", "کره‌ای":
			return "Korean", "کره‌ای", true
		case "ترکی":
			return "Turkish", "ترکی", true
		case "عربی":
			return "Arabic", "عربی", true
		case "فرانسوی", "فرانسه":
			return "French", "فرانسوی", true
		case "آلمانی", "المان", "آلمان":
			return "German", "آلمانی", true
		case "اسپانیایی":
			return "Spanish", "اسپانیایی", true
		case "ژاپنی":
			return "Japanese", "ژاپنی", true
		case "چینی":
			return "Chinese", "چینی", true
		case "ایتالیایی":
			return "Italian", "ایتالیایی", true
		case "فارسی":
			return "Persian (Farsi)", "فارسی", true
		}
	}
	return "", "", false
}

func TranslateText(text, targetLang string) (string, error) {
	_ = godotenv.Load("/opt/wolf/.env")
	apiKey := strings.TrimSpace(os.Getenv("GROQ_API_KEY"))
	if apiKey == "" {
		return "", fmt.Errorf("کلید API یافت نشد")
	}

	apiURL := "https://api.groq.com/openai/v1/chat/completions"
	sysPrompt := fmt.Sprintf("You are a professional translator. Translate the following text to %s. Output ONLY the final translation. Do not include any extra text, comments, quotes, or conversational phrases.", targetLang)

	reqBody := GroqRequest{
		Model: "qwen/qwen3.8-27b",
		Messages: []Message{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: text},
		},
		Temperature: 0.1,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	var lastErr error
	for i := 0; i < 3; i++ {
		req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
		if err != nil {
			lastErr = err
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(1 * time.Second)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("کد %d - %s", resp.StatusCode, string(body))
			time.Sleep(1 * time.Second)
			continue
		}

		var result GroqResponse
		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = err
			time.Sleep(1 * time.Second)
			continue
		}

		if len(result.Choices) > 0 {
			return strings.TrimSpace(result.Choices[0].Message.Content), nil
		}
	}

	return "", fmt.Errorf("خطا پس از 3 بار تلاش: %v", lastErr)
}

func ProcessLiveTranslator(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, msg *tg.Message, text string) bool {
	targetLangEn, targetLangFa, isCmd := getLanguageSettings(text)
	if !isCmd {
		return false
	}

	if msg.ReplyTo == nil {
		go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ لطفاً روی یک پیام ریپلای کنید.")
		return true
	}

	header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
	if !ok || header.ReplyToMsgID == 0 {
		go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ پیام نامعتبر است!")
		return true
	}

	replyMsgID := header.ReplyToMsgID

	go func(repID int, p tg.InputPeerClass, mID int) {
		dCtx, dCancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer dCancel()

		repMsg, _, err := getRepliedMessageAndUsers(dCtx, client, p, repID)
		if err != nil || repMsg == nil {
			notifyAndSelfDestruct(dCtx, client, p, mID, "❌ خطا در یافتن پیام اصلی.")
			return
		}

		origText := strings.TrimSpace(repMsg.Message)
		if origText == "" {
			notifyAndSelfDestruct(dCtx, client, p, mID, "⚠️ پیام فاقد متن است!")
			return
		}

		loadingText := fmt.Sprintf("⚡️ در حال ترجمه به %s...", targetLangFa)
		_, _ = client.API().MessagesEditMessage(dCtx, &tg.MessagesEditMessageRequest{
			Peer:    p,
			ID:      mID,
			Message: loadingText,
		})

		translated, err := TranslateText(origText, targetLangEn)
		if err != nil || translated == "" {
			_, _ = client.API().MessagesEditMessage(dCtx, &tg.MessagesEditMessageRequest{
				Peer:    p,
				ID:      mID,
				Message: fmt.Sprintf("❌ خطا در ترجمه:\n<code>%v</code>", err),
				Entities: []tg.MessageEntityClass{
					&tg.MessageEntityCode{Offset: 16, Length: len([]rune(fmt.Sprintf("%v", err)))},
				},
			})
			return
		}

		finalText := fmt.Sprintf("🌍 ترجمه به %s:\n\n%s", targetLangFa, translated)
		importUtf16Len := len([]rune(fmt.Sprintf("🌍 ترجمه به %s:", targetLangFa)))

		_, _ = client.API().MessagesEditMessage(dCtx, &tg.MessagesEditMessageRequest{
			Peer:    p,
			ID:      mID,
			Message: finalText,
			Entities: []tg.MessageEntityClass{
				&tg.MessageEntityBold{Offset: 0, Length: importUtf16Len},
			},
		})
	}(replyMsgID, inputPeer, msg.ID)

	return true
}

// ==========================================
// 3. سیستم قفل پیوی (PV Lock - پیش‌فرض خاموش)
// ==========================================

var (
	pvLockMu         sync.RWMutex
	pvLockSettings   = make(map[int64]bool)
	pvWhitelistCache = make(map[int64]map[int64]bool)
)

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

func IsPVLockEnabled(uid int64) bool {
	pvLockMu.RLock()
	defer pvLockMu.RUnlock()
	if enabled, ok := pvLockSettings[uid]; ok {
		return enabled
	}
	return false // پیش‌فرض: خاموش
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

// ProcessPVLockIncoming رهگیری چت‌های دوطرفه در پیوی
func ProcessPVLockIncoming(ctx context.Context, client *telegram.Client, userID int64, msg *tg.Message, e tg.Entities) bool {
	if msg.Out {
		return false
	}

	// فقط و فقط پیوی‌های دونفره
	peerUser, isPV := msg.PeerID.(*tg.PeerUser)
	if !isPV {
		return false
	}

	senderID := peerUser.UserID
	if senderID == 0 || senderID == userID || senderID == 777000 || senderID == 42777 {
		return false
	}

	// رد کردن ربات‌ها، پشتیبانی رسمی و افراد تاییدشده
	if u, ok := e.Users[senderID]; ok {
		if u.Bot || u.Verified || u.Support {
			return false
		}
	}

	// اگر قفل پیوی خاموش باشد رد می‌شود
	if !IsPVLockEnabled(userID) {
		return false
	}

	// اگر در لیست دوستان یا لیست سفید پیوی باشد رد می‌شود
	if isUserFriend(userID, senderID) || IsPVWhitelisted(userID, senderID) {
		return false
	}

	// حذف دوطرفه چت مزاحم
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

// ProcessPVLockCommand پردازش دستورات پیوی
func ProcessPVLockCommand(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, msg *tg.Message, text string, userID int64) bool {
	if text == "قفل پیوی روشن" {
		SetPVLockEnabled(userID, true)
		if inputPeer != nil {
			go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "🔒 قفل پیوی فعال شد")
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
