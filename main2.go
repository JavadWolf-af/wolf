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
)

// ساختارهای مورد نیاز برای API گروک (استاندارد OpenAI)
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

// TranslateText با استفاده از سرعت بی‌نظیر هوش مصنوعی Groq (مدل Llama 3)
func TranslateText(text string) (string, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("کلید API گروک در فایل .env تنظیم نشده است")
	}

	apiURL := "https://api.groq.com/openai/v1/chat/completions"

	// استفاده از مدل LLaMA 3.3 که برای زبان فارسی عالی و پرسرعت است
	reqBody := GroqRequest{
		Model: "llama-3.3-70b-versatile",
		Messages: []Message{
			{Role: "system", Content: "You are a professional translator. Translate the following text to Persian (Farsi). Output ONLY the final translation. Do not include any extra text, comments, quotes, or conversational phrases."},
			{Role: "user", Content: text},
		},
		Temperature: 0.1, // دمای بسیار پایین برای ترجمه کاملاً دقیق و پرهیز از داستان‌سرایی
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 15 * time.Second} // گروک آنقدر سریع است که نیاز به تایم‌اوت طولانی ندارد
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("خطای گروک (کد %d): %s", resp.StatusCode, string(body))
	}

	var result GroqResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) > 0 {
		translated := strings.TrimSpace(result.Choices[0].Message.Content)
		return translated, nil
	}

	return "", fmt.Errorf("پاسخی از شبکه گروک دریافت نشد")
}

// ProcessLiveTranslator هندل کننده دستورات مترجم در چت تلگرام
func ProcessLiveTranslator(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, msg *tg.Message, text string) bool {
	if text != "ترجمه" && text != "ترجمه کن" {
		return false
	}

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

	go func(repID int, p tg.InputPeerClass, mID int) {
		dCtx, dCancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer dCancel()

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

		loadingText := "⚡️ در حال ترجمه با موتور Groq..."
		_, _ = client.API().MessagesEditMessage(dCtx, &tg.MessagesEditMessageRequest{
			Peer:    p,
			ID:      mID,
			Message: loadingText,
		})

		translated, err := TranslateText(origText)
		if err != nil || translated == "" {
			notifyAndSelfDestruct(dCtx, client, p, mID, "❌ خطا در ارتباط با هوش مصنوعی Groq.")
			return
		}

		finalText := fmt.Sprintf("🌍 ترجمه هوشمند:\n\n%s", translated)
		titleLen := len([]rune("🌍 ترجمه هوشمند:"))

		_, _ = client.API().MessagesEditMessage(dCtx, &tg.MessagesEditMessageRequest{
			Peer:    p,
			ID:      mID,
			Message: finalText,
			Entities: []tg.MessageEntityClass{
				&tg.MessageEntityBold{Offset: 0, Length: titleLen},
			},
		})
	}(replyMsgID, inputPeer, msg.ID)

	return true
}
