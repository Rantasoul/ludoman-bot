package main

import (
	"fmt"
	"log"
	"ludoman-bot/functions"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	_ "github.com/lib/pq"
)

// Авто-приветствие при входе нового игрока на сервер
func HandleUserJoin(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if WelcomeChannelID == "" {
		log.Println("Ошибка: Не задан WELCOME_CHANNEL_ID в .env")
		return
	}
	functions.SendRegButton(s, m.User.ID, WelcomeChannelID)
}

// Основной распределитель всех интеракций на сервере
func HandleInteractions(s *discordgo.Session, i *discordgo.InteractionCreate) {

	// СЛЕШ-КОМАНДЫ (/opros и /setup_reg)

	if i.Type == discordgo.InteractionApplicationCommand {
		switch i.ApplicationCommandData().Name {
		case "opros":
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

			// Парсим время, указанное пользователем
			targetTime, err := functions.ParseTimeFromInput(inputTime)
			if err != nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("❌ Не удалось распознать время: %s\nИспользуйте формат: 19:00", inputTime),
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			// Время очистки = указанное время + 20 минут
			cleanupTime := targetTime.Add(20 * time.Minute)

			msgText := fmt.Sprintf("🔔 **СБОР НА КАТКУ!** <@&%s>\n📊 Собираем лобби 5х5 в **%s**\n⏰ Очистка в: **%s**", DotaRoleID, inputTime, cleanupTime.Format("15:04"))

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("🚀 Опрос создан!\n🕐 Очистка через 20 минут после сбора (в %s)", cleanupTime.Format("15:04")),
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})

			// Отправляем сообщение с опросом
			pollMsg, err := s.ChannelMessageSendComplex(LobbyChannelID, &discordgo.MessageSend{
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
				log.Printf("Не удалось отправить опрос: %v", err)
				return
			}

			// Запускаем горутину для очистки в нужное время
			timeUntilCleanup := time.Until(cleanupTime)
			if timeUntilCleanup > 0 {
				log.Printf("⏰ Очистка запланирована на %s (через %v)", cleanupTime.Format("15:04"), timeUntilCleanup)
				go func() {
					time.Sleep(timeUntilCleanup)
					functions.CleanupPoll(s, pollMsg.ID, pollMsg.ChannelID, DB)
				}()
			} else {
				log.Printf("⚠️ Время очистки уже прошло!")
				s.ChannelMessageSend(LobbyChannelID, "⚠️ Время для очистки уже прошло, опрос будет удалён сейчас.")
				functions.CleanupPoll(s, pollMsg.ID, pollMsg.ChannelID, DB)
			}

			//напоминание за 5 минут

			reminderTime := targetTime.Add(-5 * time.Minute)
			timeUntilReminder := time.Until(reminderTime)

			if timeUntilReminder > 0 {
				log.Printf("📨 Напоминание запланировано на %s (через %v)", reminderTime.Format("15:04"), timeUntilReminder)
				go func() {
					time.Sleep(timeUntilReminder)
					functions.SendReminderToAll(s, pollMsg.ID, DB)
				}()
			} else {
				log.Printf("⚠️ Время напоминания уже прошло!")
			}

		case "setup_reg":
			if i.ChannelID == WelcomeChannelID {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "👋 Для обновления статуса жми на кнопку:",
						Flags:   discordgo.MessageFlagsEphemeral,
						Components: []discordgo.MessageComponent{
							discordgo.ActionsRow{
								Components: []discordgo.MessageComponent{
									discordgo.Button{
										Label:    "📝 Обновить статус",
										Style:    discordgo.PrimaryButton,
										CustomID: "start_registration",
									},
								},
							},
						},
					},
				})
			} else {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("❌ Эту команду можно использовать только в приветственном канале: <#%s>!", WelcomeChannelID),
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
			}
			return
		}
		return
	}
			
	// ОБРАБОТКА НАЖАТИЙ НА КНОПКИ
	if i.Type == discordgo.InteractionMessageComponent {
		customID := i.MessageComponentData().CustomID

		// Кнопка открытия анкеты верификации
		if customID == "start_registration" {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "🎮 **Выбери игры, в которые ты планируешь с нами играть:**",
					Flags:   discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.Button{
									Label:    "Только Dota 2",
									Style:    discordgo.PrimaryButton,
									CustomID: "reg_choose_dota",
								},
								discordgo.Button{
									Label:    "Только Кооп игры",
									Style:    discordgo.SuccessButton,
									CustomID: "reg_choose_coop",
								},
								discordgo.Button{
									Label:    "И Dota 2, и Кооп",
									Style:    discordgo.DangerButton,
									CustomID: "reg_choose_both",
								},
							},
						},
					},
				},
			})
			return
		}

		// Обработчик кнопки "Только Dota 2"
		if customID == "reg_choose_dota" {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseModal,
				Data: &discordgo.InteractionResponseData{
					CustomID: "registration_modal_dota",
					Title:    "Анкета игрока Dota 2",
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{CustomID: "dota_nick", Label: "Твой ник в Steam:", Style: discordgo.TextInputShort, Placeholder: "0 мужского", Required: true}}},
						discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{CustomID: "real_name", Label: "Твоё имя:", Style: discordgo.TextInputShort, Placeholder: "Поли", Required: true}}},
						discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{CustomID: "dota_mmr", Label: "Твой рейтинг (MMR):", Style: discordgo.TextInputShort, Placeholder: "67", Required: true}}},
					},
				},
			})
			return
		}

		// Обработчик кнопки "И Dota 2, и Кооп"
		if customID == "reg_choose_both" {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseModal,
				Data: &discordgo.InteractionResponseData{
					CustomID: "registration_modal_both",
					Title:    "Анкета: Dota 2 + Coop",
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{CustomID: "dota_nick", Label: "Твой ник в Steam:", Style: discordgo.TextInputShort, Placeholder: "0 мужского", Required: true}}},
						discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{CustomID: "real_name", Label: "Твоё имя:", Style: discordgo.TextInputShort, Placeholder: "Поли", Required: true}}},
						discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{CustomID: "dota_mmr", Label: "Твой рейтинг (MMR):", Style: discordgo.TextInputShort, Placeholder: "67", Required: true}}},
					},
				},
			})
			return
		}

		// Обработчик кнопки "Только Кооп"
		if customID == "reg_choose_coop" {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseModal,
				Data: &discordgo.InteractionResponseData{
					CustomID: "registration_modal_coop",
					Title:    "Анкета игрока Coop",
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{CustomID: "real_name", Label: "Твоё имя:", Style: discordgo.TextInputShort, Placeholder: "Влад", Required: true}}},
					},
				},
			})
			return
		}

		// Логика голосования
		if customID == "lobby_go" || customID == "lobby_clown" || customID == "lobby_later" {
			functions.LobbyPoll(s, i, customID, DB)
			return
		}
		return
	}

	// ОБРАБОТКА МОДАЛЬНЫХ ОКОН

	if i.Type == discordgo.InteractionModalSubmit {
		modalID := i.ModalSubmitData().CustomID

		// Обработка модалки для Coop
		if modalID == "registration_modal_coop" {
			var realName string

			for _, row := range i.ModalSubmitData().Components {
				actionRow := row.(*discordgo.ActionsRow)
				for _, component := range actionRow.Components {
					textInput := component.(*discordgo.TextInput)
					if textInput.CustomID == "real_name" {
						realName = textInput.Value
					}
				}
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
				},
			})

			functions.ProcessCoopRegistration(s, i, realName, DB, DotaRoleID, CoopRoleID, WelcomeChannelID)

			content := "✅ Профиль успешно обновлён!"
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &content,
			})

			return
		}

		// Обработка модалок для Dota и Both
		if modalID == "registration_modal_dota" || modalID == "registration_modal_both" {
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

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
				},
			})

			functions.ProcessDotaRegistration(s, i, dotaNick, realName, dotaMMR, modalID, DB, DotaRoleID, CoopRoleID, WelcomeChannelID)

			content := "✅ Профиль успешно обновлён!"
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &content,
			})

			return
		}
	}
}

// Обработчик всех сообщений в чате
func HandleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	
	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.ChannelID != ChatChannelID {
		return
	}

	botMention := "<@" + s.State.User.ID + ">"
	botMentionWithExclamation := "<@!" + s.State.User.ID + ">"
	if !strings.Contains(m.Content, botMention) && !strings.Contains(m.Content, botMentionWithExclamation) {
		return 
	}
	
	if strings.HasPrefix(m.Content, "/") {
		return
	}

	cleanContent := strings.ReplaceAll(m.Content, botMention, "")
	cleanContent = strings.ReplaceAll(cleanContent, botMentionWithExclamation, "")
	cleanContent = strings.TrimSpace(cleanContent)

	if len(cleanContent) < 2 {
		s.ChannelMessageSendReply(m.ChannelID, "Поставил твою маму на зелёное.", m.Reference())
		time.Sleep(3 * time.Second)
		s.ChannelTyping(m.ChannelID)
		s.ChannelMessageSendReply(m.ChannelID, "Мамы больше нет.", m.Reference())
		return
	}

	s.ChannelTyping(m.ChannelID)

	answer := functions.AskAI(cleanContent, CfAPIToken, CfAccountID)

	if answer == "" {
		answer = "У меня сервера расплавились от твоей тупости. Исчезни."
	}

	// ответ в виде Reply 
	_, err := s.ChannelMessageSendReply(m.ChannelID, answer, m.Reference())
	if err != nil {
		log.Printf("❌ Ошибка отправки ответа Лудомана: %v", err)
	}
}
