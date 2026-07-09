package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"syscall"
	_ "github.com/lib/pq" 
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
	DB               *sql.DB
)

func main() {
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
	defer DB.Close()

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
	defer dg.Close()

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

	// Ожидаем завершения
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	log.Println("👋 Бот отключается...")
}

// ВСЕ ОБРАБОТЧИКИ (HandleUserJoin, HandleInteractions и т.д.)

// Авто-приветствие при входе нового игрока на сервер
func HandleUserJoin(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if WelcomeChannelID == "" {
		log.Println("Ошибка: Не задан WELCOME_CHANNEL_ID в .env")
		return
	}
	sendRegButton(s, m.User.ID, WelcomeChannelID)
}

// Вспомогательная функция, которая рисует сообщение с кнопкой регистрации
func sendRegButton(s *discordgo.Session, userID string, channelID string) {
	content := "Приветствую на сервере УДОВОЛЬСТВИЕ! Нажми кнопку ниже, чтобы пройти регистрацию."
	if userID != "" {
		content = fmt.Sprintf("👋 Приветствуем нового лудика, <@%s>! Нажми кнопку ниже, чтобы пройти регистрацию и получить по заслугам.", userID)
	}

	_, err := s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: content,
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "📝 Пройти регистрацию",
						Style:    discordgo.PrimaryButton,
						CustomID: "start_registration",
					},
				},
			},
		},
	})
	if err != nil {
		log.Printf("Не удалось отправить кнопку регистрации: %v", err)
	}
}

// Основной распределитель всех интеракций на сервере
func HandleInteractions(s *discordgo.Session, i *discordgo.InteractionCreate) {

	
	// 1. СЛЕШ-КОМАНДЫ (/opros и /setup_reg)
	
	if i.Type == discordgo.InteractionApplicationCommand {
		switch i.ApplicationCommandData().Name {
		case "opros":
			// Проверяем, в том ли чате вызвана команда
			if i.ChannelID != LobbyChannelID {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("❌ Эту команду можно использовать только в канале сбора лобби: <#%s>!", LobbyChannelID),
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			var inputTime string
			options := i.ApplicationCommandData().Options
			if len(options) > 0 {
				inputTime = options[0].StringValue()
			}

			msgText := fmt.Sprintf("🔔 **СБОР НА КАТКУ!** <@&%s>\n📊 Собираем лобби 5х5 в **%s**", DotaRoleID, inputTime)

			// Отвечаем админу скрытым сообщением об успехе
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "🚀 Опрос успешно создан!",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})

			_, err := s.ChannelMessageSendComplex(LobbyChannelID, &discordgo.MessageSend{
				Content: msgText,
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{Label: "Я буду (0)", Style: discordgo.SuccessButton, CustomID: "lobby_go", Emoji: &discordgo.ComponentEmoji{Name: "✅"}},
							discordgo.Button{Label: "Я долбоеб (0)", Style: discordgo.DangerButton, CustomID: "lobby_clown", Emoji: &discordgo.ComponentEmoji{Name: "🤡"}},
							discordgo.Button{Label: "Позже (0)", Style: discordgo.SecondaryButton, CustomID: "lobby_later", Emoji: &discordgo.ComponentEmoji{Name: "⏳"}},
						},
					},
				},
			})
			if err != nil {
				log.Printf("Не удалось отправить лобби: %v", err)
			}

		case "setup_reg":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Приветствую на сервере УДОВОЛЬСТВИЕ! Нажми кнопку ниже, чтобы пройти регистрацию.",
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.Button{Label: "📝 Пройти регистрацию", Style: discordgo.PrimaryButton, CustomID: "start_registration"},
							},
						},
					},
				},
			})
		}
		return
	}

	// 2. ОБРАБОТКА НАЖАТИЙ НА КНОПКИ
	
	if i.Type == discordgo.InteractionMessageComponent {
		customID := i.MessageComponentData().CustomID

		// Кнопка открытия анкеты верификации
		if customID == "start_registration" {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseModal,
				Data: &discordgo.InteractionResponseData{
					CustomID: "registration_modal",
					Title:    "Анкета игрока Dota 2",
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{CustomID: "dota_nick", Label: "Твой ник в Steam:", Style: discordgo.TextInputShort, Placeholder: "Грязная", Required: true}}},
						discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{CustomID: "real_name", Label: "Твоё имя:", Style: discordgo.TextInputShort, Placeholder: "Влад", Required: true}}},
						discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{CustomID: "dota_mmr", Label: "Твой рейтинг (MMR):", Style: discordgo.TextInputShort, Placeholder: "4600", Required: true}}},
					},
				},
			})
			return
		}

		// Логика голосования
		if customID == "lobby_go" || customID == "lobby_clown" || customID == "lobby_later" {
			message := i.Message
			userID := i.Member.User.ID

			var currentChoice string
			var switchCount int
			var oldFeedbackMsgID sql.NullString

			// Запрашиваем информацию о голосовании юзера 
			err := DB.QueryRow("SELECT current_choice, switch_count, feedback_message_id FROM lobby_votes WHERE message_id = $1 AND user_id = $2", message.ID, userID).
				Scan(&currentChoice, &switchCount, &oldFeedbackMsgID)

			// Лимит: если чувак пытается переголосовать уже во ВТОРОЙ раз (switch_count >= 1)
			if err == nil && switchCount >= 1 && currentChoice != customID {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "❌ Ошибка: Не заебывай, больше переобуваться нельзя.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			// Если он нажимает ту же самую кнопку, на которую уже нажал ранее
			if err == nil && currentChoice == customID {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "ℹ️ Ты уже выбрал этот вариант",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			actionRow := message.Components[0].(*discordgo.ActionsRow)
			re := regexp.MustCompile(`\((\d+)\)`)

			// Обновляем счетчики
			for idx, component := range actionRow.Components {
				button := component.(*discordgo.Button)

				// Если человек переголосовывает, уменьшаем счетчик у его СТАРОГО выбора
				if err == nil && button.CustomID == currentChoice {
					matches := re.FindStringSubmatch(button.Label)
					if len(matches) > 1 {
						count, _ := strconv.Atoi(matches[1])
						if count > 0 {
							count--
						}
						button.Label = updateButtonLabel(button.CustomID, count)
					}
				}

				// Увеличиваем счетчик у НОВОГО нажатого варианта
				if button.CustomID == customID {
					matches := re.FindStringSubmatch(button.Label)
					if len(matches) > 1 {
						count, _ := strconv.Atoi(matches[1])
						count++
						button.Label = updateButtonLabel(button.CustomID, count)
					}
				}
				actionRow.Components[idx] = button
			}

			// фидбеки на кнопки
			var userFeedback string
			switch customID {
			case "lobby_go":
				userFeedback = fmt.Sprintf("✅ Игрок <@%s> готов чистить ебла!", i.Member.User.ID)
			case "lobby_clown":
				userFeedback = fmt.Sprintf("🤡 <@%s> нажал кнопку 'Я долбоеб' и слился с катки.", i.Member.User.ID)
			case "lobby_later":
				userFeedback = fmt.Sprintf("⏳ <@%s> просит подождать его, подлетит позже.", i.Member.User.ID)
			}

			// Удаляем старое фидбек-сообщение, если оно есть
			if err == nil && oldFeedbackMsgID.Valid && oldFeedbackMsgID.String != "" {
				errDelete := s.ChannelMessageDelete(message.ChannelID, oldFeedbackMsgID.String)
				if errDelete != nil {
					log.Printf("Не удалось удалить старое фидбек-сообщение: %v", errDelete)
				}
			}

			// Редактируем сообщение пульта 
			_, errEdit := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
				ID:         message.ID,
				Channel:    message.ChannelID,
				Components: &[]discordgo.MessageComponent{actionRow},
			})
			if errEdit != nil {
				log.Printf("Не удалось отредактировать сообщение: %v", errEdit)
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "❌ Ошибка при обновлении голосования",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			// Отправляем фидбек-сообщение
			errRespond := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: userFeedback,
				},
			})
			if errRespond != nil {
				log.Printf("Не удалось отправить фидбек: %v", errRespond)
				return
			}

			// Получаем ID отправленного фидбек-сообщения
			var newFeedbackID string
			responseMsg, errFetch := s.InteractionResponse(i.Interaction)
			if errFetch == nil {
				newFeedbackID = responseMsg.ID
			}

			// ОБНОВЛЯЕМ БАЗУ ДАННЫХ
			if err == sql.ErrNoRows {
				// Первое нажатие
				_, errInsert := DB.Exec("INSERT INTO lobby_votes (message_id, user_id, current_choice, switch_count, feedback_message_id) VALUES ($1, $2, $3, $4, $5)",
					message.ID, userID, customID, 0, newFeedbackID)
				if errInsert != nil {
					log.Printf("Ошибка INSERT в БД: %v", errInsert)
				}
			} else if err == nil {
				// Переголосование
				_, errUpdate := DB.Exec("UPDATE lobby_votes SET current_choice = $1, switch_count = switch_count + 1, feedback_message_id = $2 WHERE message_id = $3 AND user_id = $4",
					customID, newFeedbackID, message.ID, userID)
				if errUpdate != nil {
					log.Printf("Ошибка UPDATE в БД: %v", errUpdate)
				}
			}
		}
		return
	}

	// 3. ОБРАБОТКА МОДАЛЬНЫХ ОКОН
	if i.Type == discordgo.InteractionModalSubmit {
		if i.ModalSubmitData().CustomID == "registration_modal" {
			var dotaNick, realName, dotaMMR string

			for _, row := range i.ModalSubmitData().Components {
				actionRow := row.(*discordgo.ActionsRow)
				for _, component := range actionRow.Components {
					textInput := component.(*discordgo.TextInput)
					switch textInput.CustomID {
					case "dota_nick":
						dotaNick = textInput.Value
					case "real_name":
						realName = textInput.Value
					case "dota_mmr":
						dotaMMR = textInput.Value
					}
				}
			}

			// Проверка MMR
			mmrValue, err := strconv.Atoi(dotaMMR)
			if err != nil || mmrValue < 0 || mmrValue > 15000 {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "❌ Слышь, фрик, в поле MMR нужно ввести нормальное число (например, 4500). Не выебывайся.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			newNickname := fmt.Sprintf("%s | %s | %s", dotaNick, realName, dotaMMR)

			if len([]rune(newNickname)) > 32 {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "❌ Ошибка: получившийся ник слишком длинный (больше 32 символов). Сука, сократи ник или имя.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			// Меняем ник
			err = s.GuildMemberNickname(i.GuildID, i.Member.User.ID, newNickname)
			if err != nil {
				log.Printf("Не удалось изменить ник пользователю %s: %v", i.Member.User.ID, err)
			}

			// Выдаем роль
			err = s.GuildMemberRoleAdd(i.GuildID, i.Member.User.ID, TargetRoleID)
			if err != nil {
				log.Printf("Не удалось выдать роль пользователю %s: %v", i.Member.User.ID, err)
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "❌ Не удалось выдать роль. Скорее всего, роль бота находится ниже целевой роли в настройках сервера.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			// Ответ
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("🎉 Встречайте нового фрика — **%s**!\nРоль успешно выдана, добро пожаловать в семью! 🎰", newNickname),
				},
			})
		}
	}
}

func updateButtonLabel(customID string, count int) string {
	switch customID {
	case "lobby_go":
		return fmt.Sprintf("Я буду (%d)", count)
	case "lobby_clown":
		return fmt.Sprintf("Я долбоеб (%d)", count)
	case "lobby_later":
		return fmt.Sprintf("Позже (%d)", count)
	}
	return ""
}
