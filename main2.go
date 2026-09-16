package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
)

// TranslateText متن را با API رایگان و بدون لیمیت گوگل به فارسی ترجمه می‌کند
func TranslateText(text string) (string, error) {
	// استفاده از کلاینت gtx برای دور زدن محدودیت کلید API
	apiURL := "https://translate.googleapis.com/translate_a/single?client=gtx&sl=auto&tl=fa&dt=t&q=" + url.QueryEscape(text)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result []interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	// استخراج ترجمه از آرایه‌های تودرتوی گوگل
	if len(result) > 0 {
		var translatedText strings.Builder
		innerArray, ok := result[0].([]interface{})
		if !ok {
			return "", fmt.Errorf("فرمت پاسخ نامعتبر است")
		}
		for _, item := range innerArray {
			part, ok := item.([]interface{})
			if ok && len(part) > 0 {
				if str, ok := part[0].(string); ok {
					translatedText.WriteString(str)
				}
			}
		}
		return translatedText.String(), nil
	}
	return "", fmt.Errorf("ترجمه‌ای یافت نشد")
}

// ProcessLiveTranslator بررسی می‌کند که آیا پیام مربوط به دستور ترجمه است یا خیر
func ProcessLiveTranslator(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, msg *tg.Message, text string) bool {
	if text != "ترجمه" && text != "ترجمه کن" {
		return false
	}

	// اگر روی پیامی ریپلای نکرده باشد
	if msg.ReplyTo == nil {
		go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ لطفاً روی یک پیام خارجی ریپلای کنید و بنویسید: ترجمه")
		return true
	}

	header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
	if !ok || header.ReplyToMsgID == 0 {
		go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ پیام نامعتبر است!")
		return true
	}

	replyMsgID := header.ReplyToMsgID

	// اجرای ترجمه در یک روتین موازی تا ربات قفل نکند
	go func(repID int, p tg.InputPeerClass, mID int) {
		dCtx, dCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer dCancel()

		// ۱. واکشی پیام اصلی که روی آن ریپلای شده (با استفاده از تابع مشترک در main.go)
		repMsg, _, err := getRepliedMessageAndUsers(dCtx, client, p, repID)
		if err != nil || repMsg == nil {
			notifyAndSelfDestruct(dCtx, client, p, mID, "❌ خطا در یافتن پیام اصلی.")
			return
		}

		origText := strings.TrimSpace(repMsg.Message)
		if origText == "" {
			notifyAndSelfDestruct(dCtx, client, p, mID, "⚠️ پیام اصلی فاقد متن برای ترجمه است!")
			return
		}

		// ۲. ویرایش زنده پیام کاربر به وضعیت لودینگ
		loadingText := "🌍 در حال ترجمه..."
		_, _ = client.API().MessagesEditMessage(dCtx, &tg.MessagesEditMessageRequest{
			Peer:    p,
			ID:      mID,
			Message: loadingText,
		})

		// ۳. ارسال درخواست به گوگل
		translated, err := TranslateText(origText)
		if err != nil || translated == "" {
			notifyAndSelfDestruct(dCtx, client, p, mID, "❌ خطا در ارتباط با سرور گوگل.")
			return
		}

		// ۴. جایگزینی ترجمه روان فارسی در پیام
		finalText := fmt.Sprintf("🌍 ترجمه پیام:\n\n%s", translated)

		// اعمال استایل بولد فقط روی تیتر
		titleLen := len([]rune("🌍 ترجمه پیام:"))

		_, _ = client.API().MessagesEditMessage(dCtx, &tg.MessagesEditMessageRequest{
			Peer:    p,
			ID:      mID,
			Message: finalText,
			Entities: []tg.MessageEntityClass{
				&tg.MessageEntityBold{Offset: 0, Length: titleLen},
			},
		})
	}(replyMsgID, inputPeer, msg.ID)

	return true // به این معنی است که دستور ترجمه پردازش شد و کدهای main.go دیگر اجرا نشوند
}
