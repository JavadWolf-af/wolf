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
)

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

// getLanguageSettings زبان مورد نظر را از پیام کاربر تشخیص می‌دهد
func getLanguageSettings(cmd string) (string, string, bool) {
	// حالت پیش‌فرض (ترجمه به فارسی)
	if cmd == "ترجمه" || cmd == "ترجمه کن" {
		return "Persian (Farsi)", "فارسی", true
	}

	// تشخیص زبان‌های دیگر با پسوند "شو"
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
		return "", fmt.Errorf("کلید API در سرور یافت نشد. مطمئن شوید در فایل .env قرار دارد")
	}

	apiURL := "https://api.groq.com/openai/v1/chat/completions"

	// پرامپت پویا برای ترجمه به زبانی که کاربر خواسته است
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
		return "", fmt.Errorf("خطای ساخت جیسون: %v", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("خطای ساخت ریکوئست: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("خطای شبکه یا اینترنت سرور: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("کد %d - %s", resp.StatusCode, string(body))
	}

	var result GroqResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("خطا در خواندن پاسخ هوش مصنوعی: %v", err)
	}

	if len(result.Choices) > 0 {
		translated := strings.TrimSpace(result.Choices[0].Message.Content)
		return translated, nil
	}

	return "", fmt.Errorf("پاسخی از شبکه گروک دریافت نشد")
}

func ProcessLiveTranslator(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, msg *tg.Message, text string) bool {
	
	// بررسی اینکه آیا پیام کاربر یک دستور ترجمه است یا خیر
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
		titleLen := len([]rune(fmt.Sprintf("🌍 ترجمه به %s:", targetLangFa)))

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
