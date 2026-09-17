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
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"github.com/joho/godotenv"
	tele "gopkg.in/telebot.v3"
)

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
		pvLockStatus = "🟢 روشن"
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
▫️ 🔐 <b>قفل پیوی:</b> %s
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
			return c.Send("❌ <b>دسترسی محدود!</b>\n\nبخش راهنما فقط برای کاربران فعال در دسترس است.", getKeyboard(userID), tele.ModeHTML)
		}
		return c.Send(buildGuideDashboardText(userID), guideMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGClock, func(c tele.Context) error {
		return c.Send("⏱ <b>ساعت زنده:</b> نمایش ساعت رسمی روی فامیلی اکانت.", guideClockMenu, tele.ModeHTML)
	})
	bot.Handle(&btnClockOn, func(c tele.Context) error {
		ub, ok := activeUserbots[c.Sender().ID]
		if !ok || ub.Client == nil { return c.Send("❌ سلف آنلاین نیست.", guideClockMenu, tele.ModeHTML) }
		handleClockOn(context.Background(), c.Sender().ID, ub.Client)
		return c.Send("🟢 ساعت زنده روشن شد.", guideClockMenu, tele.ModeHTML)
	})
	bot.Handle(&btnClockOff, func(c tele.Context) error {
		ub, ok := activeUserbots[c.Sender().ID]
		if !ok || ub.Client == nil { return c.Send("❌ سلف آنلاین نیست.", guideClockMenu, tele.ModeHTML) }
		handleClockOff(context.Background(), c.Sender().ID, ub.Client)
		return c.Send("🔴 ساعت زنده خاموش شد.", guideClockMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGEmoji, func(c tele.Context) error { return c.Send("🎭 <b>اموجی رندوم کنار اسم</b>", guideEmojiMenu, tele.ModeHTML) })
	bot.Handle(&btnEmojiOn, func(c tele.Context) error {
		ub, ok := activeUserbots[c.Sender().ID]
		if !ok || ub.Client == nil { return c.Send("❌ سلف آنلاین نیست.", guideEmojiMenu, tele.ModeHTML) }
		handleEmojiOn(context.Background(), c.Sender().ID, ub.Client)
		return c.Send("🟢 اموجی روشن شد.", guideEmojiMenu, tele.ModeHTML)
	})
	bot.Handle(&btnEmojiOff, func(c tele.Context) error {
		ub, ok := activeUserbots[c.Sender().ID]
		if !ok || ub.Client == nil { return c.Send("❌ سلف آنلاین نیست.", guideEmojiMenu, tele.ModeHTML) }
		handleEmojiOff(context.Background(), c.Sender().ID, ub.Client)
		return c.Send("🔴 اموجی خاموش شد.", guideEmojiMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGBio, func(c tele.Context) error { return c.Send("📝 <b>بیوگرافی هوشمند</b>", guideBioMenu, tele.ModeHTML) })
	bot.Handle(&btnBioOn, func(c tele.Context) error {
		ub, ok := activeUserbots[c.Sender().ID]
		if !ok || ub.Client == nil { return c.Send("❌ سلف آنلاین نیست.", guideBioMenu, tele.ModeHTML) }
		handleBioOn(context.Background(), c.Sender().ID, ub.Client)
		return c.Send("🟢 بیوگرافی فعال شد.", guideBioMenu, tele.ModeHTML)
	})
	bot.Handle(&btnBioOff, func(c tele.Context) error {
		ub, ok := activeUserbots[c.Sender().ID]
		if !ok || ub.Client == nil { return c.Send("❌ سلف آنلاین نیست.", guideBioMenu, tele.ModeHTML) }
		handleBioOff(context.Background(), c.Sender().ID, ub.Client)
		return c.Send("🔴 بیوگرافی خاموش شد.", guideBioMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGFriend, func(c tele.Context) error { return c.Send("🌸 <b>سیستم دوست:</b> پاسخ خودکار دوستانه به پیام‌ها.", guideFriendMenu, tele.ModeHTML) })
	bot.Handle(&btnGEnemy, func(c tele.Context) error { return c.Send("⚔️ <b>سیستم دشمن:</b> تیکه‌انداختن خودکار به پیام‌ها.", guideEnemyMenu, tele.ModeHTML) })

	bot.Handle(&btnGFont, func(c tele.Context) error { return c.Send("✒️ <b>تنظیمات استایل و فونت پیام‌ها</b>", guideFontMenu, tele.ModeHTML) })
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
			return c.Send(fmt.Sprintf("✅ فونت به «%s» تنظیم شد.", label), guideFontMenu, tele.ModeHTML)
		}
	}
	bot.Handle(&btnFontBold, setFont("bold", "بولد"))
	bot.Handle(&btnFontBoldItalic, setFont("bold_italic", "بولد ایتالیک"))
	bot.Handle(&btnFontItalic, setFont("italic", "ایتالیک"))
	bot.Handle(&btnFontUnderline, setFont("underline", "زیر خط"))
	bot.Handle(&btnFontStrike, setFont("strike", "خط خورده"))
	bot.Handle(&btnFontMono, setFont("mono", "مونو"))
	bot.Handle(&btnFontSpoiler, setFont("spoiler", "اسپویل"))

	bot.Handle(&btnGAction, func(c tele.Context) error { return c.Send("🎬 دستورات: <code>تایپینگ 20</code>، <code>ضبط صدا 20</code> و...", guideActionMenu, tele.ModeHTML) })
	bot.Handle(&btnGPurge, func(c tele.Context) error { return c.Send("🗑 دستور: <code>پاکشو 20</code> یا ریپلای با <code>پاکشو</code>", guidePurgeMenu, tele.ModeHTML) })
	bot.Handle(&btnGTimer, func(c tele.Context) error { return c.Send("⏳ دستور: <code>تایمر 10</code>", guideTimerMenu, tele.ModeHTML) })
	bot.Handle(&btnGAutoReact, func(c tele.Context) error { return c.Send("🔥 ریپلای کنید: <code>ری اکشن 🔥</code>", guideAutoReactMenu, tele.ModeHTML) })
	bot.Handle(&btnGTranslator, func(c tele.Context) error { return c.Send("🌍 ریپلای کنید: <code>ترجمه</code> یا <code>انگلیسی شو</code>", guideTranslatorMenu, tele.ModeHTML) })
	
	bot.Handle(&btnGPVLock, func(c tele.Context) error {
		return c.Send(`🔐 <b>راهنمای قفل پیوی ضد مزاحم:</b>
این قابلیت در پنل 🐺 <b>ولف +</b> قرار دارد و دارای دکمه مستقیم روشن/خاموش است.
همچنین با دستورات زیر در چت قابل کنترل است:
▫️ <code>قفل پیوی روشن</code>
▫️ <code>قفل پیوی خاموش</code>
▫️ <code>بازکردن پیوی</code> (داخل چت کاربر)
▫️ <code>بستن پیوی</code>`, guidePVLockMenu, tele.ModeHTML)
	})

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

// موتور مترجم Groq
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
		case "انگلیسی": return "English", "انگلیسی", true
		case "روسی": return "Russian", "روسی", true
		case "کره ای", "کره‌ای": return "Korean", "کره‌ای", true
		case "ترکی": return "Turkish", "ترکی", true
		case "عربی": return "Arabic", "عربی", true
		case "فرانسوی", "فرانسه": return "French", "فرانسوی", true
		case "آلمانی", "المان", "آلمان": return "German", "آلمانی", true
		case "اسپانیایی": return "Spanish", "اسپانیایی", true
		case "ژاپنی": return "Japanese", "ژاپنی", true
		case "چینی": return "Chinese", "چینی", true
		case "ایتالیایی": return "Italian", "ایتالیایی", true
		case "فارسی": return "Persian (Farsi)", "فارسی", true
		}
	}
	return "", "", false
}

func TranslateText(text, targetLang string) (string, error) {
	_ = godotenv.Load("/opt/wolf/.env")
	apiKey := strings.TrimSpace(os.Getenv("GROQ_API_KEY"))
	if apiKey == "" { return "", fmt.Errorf("کلید API یافت نشد") }

	apiURL := "https://api.groq.com/openai/v1/chat/completions"
	reqBody := GroqRequest{
		Model: "qwen/qwen3.8-27b",
		Messages: []Message{
			{Role: "system", Content: fmt.Sprintf("You are a professional translator. Translate the following text to %s. Output ONLY the final translation.", targetLang)},
			{Role: "user", Content: text},
		},
		Temperature: 0.1,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil { return "", err }

	var lastErr error
	for i := 0; i < 3; i++ {
		req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
		if err != nil { lastErr = err; continue }
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil { lastErr = err; time.Sleep(1 * time.Second); continue }
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("کد %d - %s", resp.StatusCode, string(body))
			time.Sleep(1 * time.Second)
			continue
		}
		var result GroqResponse
		if err := json.Unmarshal(body, &result); err != nil { lastErr = err; time.Sleep(1 * time.Second); continue }
		if len(result.Choices) > 0 { return strings.TrimSpace(result.Choices[0].Message.Content), nil }
	}
	return "", fmt.Errorf("خطا در ترجمه: %v", lastErr)
}

func ProcessLiveTranslator(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, msg *tg.Message, text string) bool {
	targetLangEn, targetLangFa, isCmd := getLanguageSettings(text)
	if !isCmd { return false }
	if msg.ReplyTo == nil {
		go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ روی یک پیام ریپلای کنید.")
		return true
	}
	header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
	if !ok || header.ReplyToMsgID == 0 { return true }

	go func(repID int, p tg.InputPeerClass, mID int) {
		dCtx, dCancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer dCancel()
		repMsg, _, err := getRepliedMessageAndUsers(dCtx, client, p, repID)
		if err != nil || repMsg == nil { return }
		origText := strings.TrimSpace(repMsg.Message)
		if origText == "" { return }

		_, _ = client.API().MessagesEditMessage(dCtx, &tg.MessagesEditMessageRequest{
			Peer: p, ID: mID, Message: fmt.Sprintf("⚡️ در حال ترجمه به %s...", targetLangFa),
		})
		translated, err := TranslateText(origText, targetLangEn)
		if err != nil || translated == "" {
			_, _ = client.API().MessagesEditMessage(dCtx, &tg.MessagesEditMessageRequest{
				Peer: p, ID: mID, Message: fmt.Sprintf("❌ خطا در ترجمه: %v", err),
			})
			return
		}
		finalText := fmt.Sprintf("🌍 ترجمه به %s:\n\n%s", targetLangFa, translated)
		importLen := len([]rune(fmt.Sprintf("🌍 ترجمه به %s:", targetLangFa)))
		_, _ = client.API().MessagesEditMessage(dCtx, &tg.MessagesEditMessageRequest{
			Peer: p, ID: mID, Message: finalText, Entities: []tg.MessageEntityClass{&tg.MessageEntityBold{Offset: 0, Length: importLen}},
		})
	}(header.ReplyToMsgID, inputPeer, msg.ID)
	return true
}
