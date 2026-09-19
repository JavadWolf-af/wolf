package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var randomEmojiPool = []string{
	"🐺", "👑", "⚡", "🔥", "💎", "✨", "🚀", "🪐", "🌪", "🦁",
	"🦅", "🎯", "🎲", "🖤", "🤍", "❄️", "🌙", "⭐", "💫", "🗡",
	"🛡", "🧿", "🔮", "🎭", "🌊", "🩸", "🕊", "☘️", "🥀", "🌹",
	"🍒", "☕", "🛸", "⚓", "⏳", "🗝", "⚔️", "🏎", "🐉", "🐾",
}

var randomBioPool = []string{
	"🐺 در سکوت شب، زوزه‌ی گرگ شنیدنی‌تر است.",
	"⚡️ قوی بمان، قصه‌ی تو پایان درخشانی دارد.",
	"🖤 گاهی سکوت، رساترین فریاد درونی است.",
	"👑 پادشاه قلمرو خویشتن باش، نه برده دیگران.",
	"🌙 شب‌های تاریک، نویدبخش سپیده‌دمی روشن‌اند.",
	"🗡️ با زخم‌هایت رشد کن، نه فقط با آرزوهایت.",
	"✨ در عمق تاریکی‌ها نیز می‌توان درخشید.",
	"🕊️ آزادی حقیقی، رهایی از قضاوت بی‌ارزش‌هاست.",
	"🦁 شیر در بند هم که باشد، همچنان سلطانی مغرور است.",
	"🌊 آرام مثل سطح آب، عمیق و سهمگین چون اقیانوس.",
	"🔥 از خاکسترِ شکست‌ها، ققنوسی مقتدر بساز!",
	"💎 اصالت را هیچ بهایی نمی‌تواند بسنجد.",
	"⏳ زمان می‌گذرد و حقیقت‌ها عریان‌تر می‌شوند.",
	"🎯 متمرکز بر هدف؛ صداهای مزاحم را نشنیده بگیر.",
	"🥀 از ریشه‌های خویش جوانه می‌نم؛ استوارتر از دیروز.",
	"🪐 در مدار سرنوشت خود، ستاره‌ای بی‌همتایم.",
	"☕️ تلخ اما سرشار از آرامش، چون خلوت شبانه.",
	"🌪️ طوفان‌ها برپا می‌شوند تا مسیر را هموار سازند.",
	"🧿 از چشم بد دور و در پناه روشنایی امید.",
	"🗝️ کلید پیروزی در صبری سرسختانه نهفته است.",
	"🏎️ شتابان به پیش؛ ایستادن مرگ جریان‌هاست.",
	"❄️ خونسرد چون بلور یخ، استوار چون صخره البرز.",
	"🎭 زندگی صحنه ماست و ما معمار تقدیر خویشیم.",
	"🐉 شعله‌های باور را در سینه زنده نگاه دار.",
	"🌟 رویاهایت را خلق کن، پیش از آنکه دیر شود!",
}

func getRandomEmoji() string {
	return randomEmojiPool[rand.Intn(len(randomEmojiPool))]
}

func getRandomBio() string {
	return randomBioPool[rand.Intn(len(randomBioPool))]
}

func cleanName(name string) string {
	name = strings.TrimSpace(name)
	changed := true
	for changed {
		changed = false
		for _, em := range randomEmojiPool {
			if strings.HasSuffix(name, em) {
				name = strings.TrimSpace(strings.TrimSuffix(name, em))
				changed = true
			}
		}
	}
	return strings.TrimSpace(name)
}

func toBoldDigits(t string) string {
	boldDigits := map[rune]string{
		'0': "𝟎", '1': "𝟏", '2': "𝟐", '3': "𝟑", '4': "𝟒",
		'5': "𝟓", '6': "𝟔", '7': "𝟕", '8': "𝟖", '9': "𝟗",
		':': ":",
	}
	var sb strings.Builder
	for _, r := range t {
		if b, ok := boldDigits[r]; ok {
			sb.WriteString(b)
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func getTehranLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Tehran")
	if err != nil {
		return time.FixedZone("Asia/Tehran", 12600)
	}
	return loc
}

func getTehranBoldTime() string {
	loc := getTehranLocation()
	now := time.Now().In(loc)
	return toBoldDigits(now.Format("15:04"))
}

func toPersianDigits(s string) string {
	persianDigits := []string{"۰", "۱", "۲", "۳", "۴", "۵", "۶", "۷", "۸", "۹"}
	for i, d := range persianDigits {
		s = strings.ReplaceAll(s, strconv.Itoa(i), d)
	}
	return s
}

func extractDigits(s string) string {
	digitMap := map[rune]rune{
		'۰': '0', '۱': '1', '۲': '2', '۳': '3', '۴': '4',
		'۵': '5', '۶': '6', '۷': '7', '۸': '8', '۹': '9',
		'٠': '0', '١': '1', '٢': '2', '٣': '3', '٤': '4',
		'٥': '5', '٦': '6', '٧': '7', '٨': '8', '٩': '9',
	}
	var sb strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			if en, ok := digitMap[r]; ok {
				sb.WriteRune(en)
			} else {
				sb.WriteRune(r)
			}
		}
	}
	return sb.String()
}

func formatMoney(n int) string {
	s := fmt.Sprintf("%d", n)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return strings.Join(parts, ",")
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func getKeyPrice() int {
	priceStr := GetSetting("key_price")
	price, err := strconv.Atoi(priceStr)
	if err != nil || price <= 0 {
		return 3333
	}
	return price
}
// ------------------------------------
// سیستم مترجم هوشمند و بررسی زبان
// ------------------------------------

func IsPersianText(text string) bool {
	persianCount := 0
	totalCount := 0
	for _, r := range text {
		if unicode.IsLetter(r) {
			totalCount++
			if unicode.In(r, unicode.Arabic) { // فارسی در بلاک عربی یونیکد است
				persianCount++
			}
		}
	}
	if totalCount == 0 {
		return false
	}
	return float64(persianCount)/float64(totalCount) > 0.4 // اگر بیش از ۴۰ درصد کاراکترها فارسی بود
}

func TranslateWithGroq(text string, targetLang string) (string, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return "", errors.New("کلید API در فایل .env یافت نشد")
	}

	// شرط خنده‌داری که خواسته بودی
	if targetLang == "فارسی" && IsPersianText(text) {
		return "😐 کسخلی؟ این خودش فارسیه", nil
	}

	langMap := map[string]string{
		"فارسی":   "Persian",
		"ترکی":    "Turkish",
		"انگلیسی": "English",
		"ژاپنی":   "Japanese",
		"چینی":    "Chinese",
		"آلمانی":  "German",
		"روسی":    "Russian",
		"دری":     "Dari (Afghan Persian)",
		"کره ای":  "Korean",
		"کره‌ای":  "Korean",
	}

	engLang, ok := langMap[targetLang]
	if !ok {
		engLang = "Persian" // پیش‌فرض
	}

	url := "https://api.groq.com/openai/v1/chat/completions"
	modelName := "qwen/qwen3.8-27b" // بهترین مدل برای این زبان‌ها بر اساس لیست سرور شما

	systemPrompt := fmt.Sprintf("You are an expert native translator. Translate the user's text into %s accurately. Output ONLY the translated text without quotes, notes, conversational filler, or markdown formatting.", engLang)

	payload := map[string]interface{}{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": text},
		},
		"temperature": 0.2,
	}

	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.New("خطا در ارتباط با سرور هوش مصنوعی")
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("خطای API: %d", resp.StatusCode)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", errors.New("خطا در پاسخ هوش مصنوعی")
	}

	if len(result.Choices) > 0 {
		return strings.TrimSpace(result.Choices[0].Message.Content), nil
	}

	return "", errors.New("متن ترجمه دریافت نشد")
}
