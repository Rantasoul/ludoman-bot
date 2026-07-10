package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var (
	BotToken         string
	GuildID          string
	WelcomeChannelID string
	LobbyChannelID   string
	TargetRoleID     string
	DotaRoleID       string
	DB               *sql.DB
)

func main() {
	loc, _ := time.LoadLocation("Europe/Moscow") 
	time.Local = loc
	// ЗАГЛУШКА ДЛЯ RENDER
	go func() {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}

		// Render будет раз в несколько минут заходить сюда, чтобы проверить жив ли бот
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Бот успешно запущен и работает!")
		})

		log.Printf("[Render] Запуск веб-заглушки на порту %s", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Printf("[Render] Ошибка веб-сервера: %v", err)
		}
	}()

	go func() {
		for {
			time.Sleep(3 * time.Minute)
			url := "https://ludoman-bot-2f0s.onrender.com"
			resp, err := http.Get(url)
			if err != nil {
				log.Printf("Пинг не удался: %v", err)
			} else {
				resp.Body.Close()
				log.Printf("Пинг успешный!")
			}
		}
	}()

	// Загружаем .env
	err := godotenv.Load()
	if err != nil {
		log.Println("Предупреждение: Файл .env не найден.")
	}

	// Читаем переменные
	BotToken = os.Getenv("DISCORD_BOT_TOKEN")
	if BotToken == "" {
		log.Fatal("Ошибка: Переменная DISCORD_BOT_TOKEN не установлена в .env")
	}

	GuildID = os.Getenv("GUILD_ID")
	WelcomeChannelID = os.Getenv("WELCOME_CHANNEL_ID")
	LobbyChannelID = os.Getenv("LOBBY_CHANNEL_ID")
	TargetRoleID = os.Getenv("TARGET_ROLE_ID")
	DotaRoleID = os.Getenv("DOTA_ROLE_ID")

	// Подключаемся к базе данных Neon
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("Ошибка: Переменная DATABASE_URL не установлена.")
	}

	DB, err = sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Ошибка открытия базы данных: %v", err)
	}

	// Проверяем подключение
	err = DB.Ping()
	if err != nil {
		log.Fatalf("Не удалось подключиться к базе данных: %v", err)
	}
	log.Println("✅ Успешное подключение к базе данных!")

	// Создаём таблицу, если её нет
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS lobby_votes (
		message_id VARCHAR(32) NOT NULL,
		user_id VARCHAR(32) NOT NULL,
		current_choice VARCHAR(20) NOT NULL,
		switch_count INT DEFAULT 0,
		feedback_message_id VARCHAR(32),
		PRIMARY KEY (message_id, user_id)
	);`
	_, err = DB.Exec(createTableSQL)
	if err != nil {
		log.Fatalf("Ошибка создания таблицы: %v", err)
	}
	log.Println("✅ Таблица lobby_votes проверена/создана!")

	// Создаём сессию Discord
	dg, err := discordgo.New("Bot " + BotToken)
	if err != nil {
		log.Fatalf("Ошибка создания сессии Discord: %v", err)
	}

	dg.Identify.Intents = discordgo.IntentsGuildMembers | discordgo.IntentsGuilds

	// Регистрируем обработчики
	dg.AddHandler(HandleInteractions)
	dg.AddHandler(HandleUserJoin)

	err = dg.Open()
	if err != nil {
		log.Fatalf("Ошибка подключения к Discord: %v", err)
	}

	// Регистрируем слеш-команды
	log.Println("📝 Регистрация слеш-команд в Discord...")
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "opros",
			Description: "Сбор лобби 5х5",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "время",
					Description: "Укажите время сбора (например: 19:00 или через час минут)",
					Required:    true,
				},
			},
		},
		{
			Name:        "setup_reg",
			Description: "Отправить кнопку начала регистрации в текущий канал",
		},
	}

	for _, v := range commands {
		_, err := dg.ApplicationCommandCreate(dg.State.User.ID, GuildID, v)
		if err != nil {
			log.Printf("Не удалось зарегистрировать команду %v: %v", v.Name, err)
		}
	}

	log.Println("🎮 Бот Лудоман успешно запущен и готов к работе!")

	select {}
}
