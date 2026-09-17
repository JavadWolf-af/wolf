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

// TranslateText دارای سیستم Retry برای جلوگیری از باگ و قطعی سرور هوش مصنوعی
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
	// سیستم تلاش مجدد هوشمند (حداکثر ۳ بار)
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
			time.Sleep(1 * time.Second) // صبر و تلاش مجدد
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
		
		// محاسبه دقیق طول برای بولد کردن با رعایت استانداردهای تلگرام (جلوگیری از باگ)
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
