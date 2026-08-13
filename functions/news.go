package functions

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// структура одного элемента новостной ленты
type NewsItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Guid        string `xml:"guid"`
}

// NewsFeed — структура всей RSS-ленты
type NewsFeed struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	Title       string     `xml:"title"`
	Link        string     `xml:"link"`
	Description string     `xml:"description"`
	Items       []NewsItem `xml:"item"`
}

// чистка хтмл
func stripHTML(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(s, "")
}

// получает и парсит RSS-ленту
func GetNews(rssURL string) ([]NewsItem, error) {
	resp, err := http.Get(rssURL)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса к RSS: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения RSS: %v", err)
	}

	var feed NewsFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("ошибка парсинга RSS: %v", err)
	}

	return feed.Channel.Items, nil
}

// отправляет новости в канал Discord
func SendNewsToChannel(s *discordgo.Session, channelID string, items []NewsItem) {

	stopWords := []string{
		"фрибет", "промокод", "БЕТБУМ", "Betboom",
		"реклама", "ставки", "Реклама",
	}

	for _, item := range items {

		skip := false
		for _, word := range stopWords {
			if strings.Contains(strings.ToLower(item.Title), strings.ToLower(word)) ||
				strings.Contains(strings.ToLower(item.Description), strings.ToLower(word)) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// Чистка HTML-тегов
		description := stripHTML(item.Description)
		description = strings.TrimSpace(description)

		// Отправляем только нормальные новости
		message := fmt.Sprintf("📰 **%s**\n%s\n🔗 %s", item.Title, description, item.Link)
		if len(message) > 2000 {
			message = message[:1997] + "..."
		}
		_, err := s.ChannelMessageSend(channelID, message)
		if err != nil {
			log.Printf("❌ Ошибка отправки новости: %v", err)
		}
	}
}

// периодическая проверку новостей
func StartNewsPoller(s *discordgo.Session, channelID, rssURL string, interval time.Duration) {
	// Хранилище уже отправленных новостей (по GUID)
	sentCache := make(map[string]bool)

	for {
		log.Printf("📡 Проверка новостей...")

		items, err := GetNews(rssURL)
		if err != nil {
			log.Printf("❌ Ошибка получения новостей: %v", err)
			time.Sleep(interval)
			continue
		}

		// Отправляем новые новости (в обратном порядке — свежие сверху)
		for i := len(items) - 1; i >= 0; i-- {
			item := items[i]
			guid := item.Guid
			if guid == "" {
				guid = item.Link // если нет GUID
			}

			if !sentCache[guid] {
				sentCache[guid] = true
				SendNewsToChannel(s, channelID, []NewsItem{item})

				// Небольшая задержка между новостями
				time.Sleep(1 * time.Second)
			}
		}

		log.Printf("✅ Проверка завершена, отправлено %d новых новостей", len(items)-len(sentCache))

		// Очищаем кеш, если он слишком большой
		if len(sentCache) > 1000 {
			sentCache = make(map[string]bool)
		}

		time.Sleep(interval)
	}
}
