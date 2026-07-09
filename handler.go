package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

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

			// Тегаем роль DotaRoleID из .env через формат <@&ID>
			msgText := fmt.Sprintf("🔔 **СБОР НА КАТКУ!** <@&%s>\n📊 Собираем лобби 5х5 в **%s**", DotaRoleID, inputTime)

			// Отвечаем админу скрытым сообщением об успехе
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "🚀 Опрос успешно создан!",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})

			// Выкатываем пульт лобби со счетчиками 
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

	
	// 2. ОБРАБОТКА НАЖАТИЙ НА КНОПКИ (СЧЁТЧИКИ)
	
	if i.Type == discordgo.InteractionMessageComponent {
		customID := i.MessageComponentData().CustomID

		// Кнопка открытия анкеты верификации фриков
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

		// Логика пересчета
		if customID == "lobby_go" || customID == "lobby_clown" || customID == "lobby_later" {
			message := i.Message
			actionRow := message.Components[0].(*discordgo.ActionsRow)
			re := regexp.MustCompile(`\((\d+)\)`)

			for idx, component := range actionRow.Components {
				button := component.(*discordgo.Button)

				if button.CustomID == customID {
					matches := re.FindStringSubmatch(button.Label)
					if len(matches) > 1 {
						currentCount, _ := strconv.Atoi(matches[1])
						newCount := currentCount + 1

						switch customID {
						case "lobby_go":
							button.Label = fmt.Sprintf("Я буду (%d)", newCount)
						case "lobby_clown":
							button.Label = fmt.Sprintf("Я долбоеб (%d)", newCount)
						case "lobby_later":
							button.Label = fmt.Sprintf("Позже (%d)", newCount)
						}
					}
				}
				actionRow.Components[idx] = button
			}

			
			var userFeedback string
			switch customID {
			case "lobby_go":
				userFeedback = fmt.Sprintf("✅ Игрок <@%s> подтвердил, что готов катать!", i.Member.User.ID)
			case "lobby_clown":
				userFeedback = fmt.Sprintf("🤡 <@%s> нажал кнопку 'Я долбоеб' и слился с катки.", i.Member.User.ID)
			case "lobby_later":
				userFeedback = fmt.Sprintf("⏳ <@%s> просит подождать его, подлетит позже.", i.Member.User.ID)
			}

			// 1. Редактируем сообщение пульта в Дискорде, обновляя цифры счетчика
			_, err := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
				ID:         message.ID,
				Channel:    message.ChannelID,
				Components: &[]discordgo.MessageComponent{actionRow},
			})
			if err != nil {
				log.Printf("Не удалось обновить счетчики на кнопках: %v", err)
			}

			// 2. Выводим обычное текстовое сообщение в чат лобби
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: userFeedback,
				},
			})
		}
		return
	}

	
	// 3. ОБРАБОТКА ОТПРАВКИ МОДАЛЬНЫХ ОКНО (АНКЕТ)
	
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

			// Склеиваем никнейм 
			newNickname := fmt.Sprintf("%s | %s | %s", dotaNick, realName, dotaMMR)

			// проверка на лимит никнейма в Discord
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

			// Пытаемся сменить ник 
			err = s.GuildMemberNickname(i.GuildID, i.Member.User.ID, newNickname)
			if err != nil {
				log.Printf("Не удалось изменить ник пользователю %s: %v", i.Member.User.ID, err)
				// Не блокируем процесс, если бот не может сменить ник админу/овнеру
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

			// ответ на ервер!
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("🎉 Встречайте нового фрика — **%s**!\nРоль успешно выдана, добро пожаловать в семью! 🎰", newNickname),
				},
			})
		}
	}
}
