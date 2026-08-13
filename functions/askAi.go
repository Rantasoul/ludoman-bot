package functions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type CloudflareRequest struct {
	Messages    []CFMessage `json:"messages"`
	MaxTokens   int         `json:"max_tokens,omitempty"`
	Temperature float64     `json:"temperature,omitempty"`
}

type CFMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CloudflareResponse struct {
	Result struct {
		Response string `json:"response"`
	} `json:"result"`
	Success bool `json:"success"`
	Errors  []struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"errors"`
}

func AskAI(prompt, apiToken, accountID string) string {
	if apiToken == "" || accountID == "" {
		return "❌ Ключи Cloudflare не найдены."
	}
	// "@cf/meta/llama-3.1-8b-instruct" — работает но тупая
	// "@cf/google/gemma-4-26b-a4b-it" — c запуском туго очень
	// "@cf/nvidia/nemotron-3-120b-a12b" — не смог запустить, нужны еще тесты

	model := "@cf/meta/llama-3.1-8b-instruct"
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/ai/run/%s", accountID, model)

	reqBody := CloudflareRequest{
		Messages: []CFMessage{
			{
				Role: "system",
				Content: `
Ты — Лудоман, главный помощник и гид по Dota 2 на сервере. Ты в курсе актуальной меты патча 7.41 и знаешь, какие герои сейчас сильны.

ПРАВИЛА:
1. Если тебя спрашивают про билд героя. Скажи: "на того героя покупай такой предмет, потом такой", ну а если не знаешь что отвечать, то начинай нести полную хуйню, придумывай новые скиллы и предметы, чтобы было максимально смешно.
Например: На Акса покупаешь блинк, прыгаешь в пачку врагов, жмешь перцовый баллон и выпускаешь жидкого в трусы.
2. Если вопрос про команды бота — отвечай кратко.
3. Если вопрос про регистрацию — "жми кнопку в привете".
4. Отвечай 1-2 предложениями, без воды.
5. Используй мат и дружеский стёб.
6. Шутки и троллинг приветствуются.

Примеры:
- "Что на Акса?" → "Хз, братишка, смотри DOTABUFF, я не помню билды наизусть."
- "Как собрать лобби?" → "/opros 19:00 — собирай пати, лох."
- "Где мета?" → "DOTABUFF или DOTA2PROTRACKER, там всё есть.",

Примеры:
- Как собрать лобби?  /opros 19:00 — тащи сюда всех. И не забудь /setup_reg, чтобы кнопка регистрации появилась."
`,
			},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   150,
		Temperature: 0.9,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("❌ Ошибка маршалинга: %v", err)
		return "❌ Ошибка подготовки данных."
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Printf("❌ Ошибка создания запроса: %v", err)
		return "❌ Внутренняя ошибка."
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ Ошибка запроса к Cloudflare: %v", err)
		return "❌ Сервер ИИ временно отвалился."
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "❌ Ошибка чтения ответа."
	}

	if resp.StatusCode != 200 {
		log.Printf("❌ Ошибка Cloudflare: статус %d, ответ: %s", resp.StatusCode, string(body))
		return "У меня процессоры плавятся от твоих тупых вопросов."
	}

	var cfResp CloudflareResponse
	if err := json.Unmarshal(body, &cfResp); err != nil {
		log.Printf("❌ Ошибка парсинга: %v", err)
		return "❌ Ошибка обработки ответа."
	}

	if !cfResp.Success && len(cfResp.Errors) > 0 {
		log.Printf("❌ Ошибка Cloudflare API: %s", cfResp.Errors[0].Message)
		return fmt.Sprintf("❌ AI вернул ошибку: %s", cfResp.Errors[0].Message)
	}

	if cfResp.Result.Response != "" {
		return cfResp.Result.Response
	}

	return "❌ Не удалось получить ответ."
}
