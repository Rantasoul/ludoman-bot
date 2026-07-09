package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var (
	BotToken         string
	GuildID          string
	WelcomeChannelID string
	LobbyChannelID   string
	TargetRoleID     string
	DotaRoleID       string
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Предупреждение: Файл .env не найден.")
	}

	BotToken = os.Getenv("DISCORD_BOT_TOKEN")
	if BotToken == "" {
		log.Fatal("Ошибка: Переменная DISCORD_BOT_TOKEN не установлена в .env")
	}

	GuildID = os.Getenv("GUILD_ID")
	WelcomeChannelID = os.Getenv("WELCOME_CHANNEL_ID")
	LobbyChannelID = os.Getenv("LOBBY_CHANNEL_ID")
	TargetRoleID = os.Getenv("TARGET_ROLE_ID")
	DotaRoleID = os.Getenv("DOTA_ROLE_ID")

	dg, err := discordgo.New("Bot " + BotToken)
	if err != nil {
		log.Fatalf("Ошибка создания сессии Discord: %v", err)
	}

	dg.Identify.Intents = discordgo.IntentsGuildMembers | discordgo.IntentsGuilds

	dg.AddHandler(HandleInteractions)

	dg.AddHandler(HandleUserJoin)

	err = dg.Open()
	if err != nil {
		log.Fatalf("Ошибка подключения к Discord: %v", err)
	}
	defer dg.Close()

	log.Println("Регистрация слеш-команд в Discord...")
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "opros",
			Description: "Сбор лобби 5х5",
			// Добавляем аргумент для ввода времени
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "время",
					Description: "Укажите время сбора (например: 19:00 или через час минут)",
					Required:    true, // Сделать поле обязательным
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

	log.Println("Бот Лудоман успешно запущен и готов к работе!")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
