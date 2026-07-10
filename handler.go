package main

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strconv"
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
			targetTime, err := parseTimeFromInput(inputTime)
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
					cleanupPoll(s, pollMsg.ID, pollMsg.ChannelID)
				}()
			} else {
				log.Printf("⚠️ Время очистки уже прошло!")
				s.ChannelMessageSend(LobbyChannelID, "⚠️ Время для очистки уже прошло, опрос будет удалён сейчас.")
				cleanupPoll(s, pollMsg.ID, pollMsg.ChannelID)
			}

			//напоминание за 5 минут

			reminderTime := targetTime.Add(-5 * time.Minute) // за 5 минут до сбора
			timeUntilReminder := time.Until(reminderTime)

			if timeUntilReminder > 0 {
				log.Printf("📨 Напоминание запланировано на %s (через %v)", reminderTime.Format("15:04"), timeUntilReminder)
				go func() {
					time.Sleep(timeUntilReminder)
					sendReminderToAll(s, pollMsg.ID)
				}()
			} else {
				log.Printf("⚠️ Время напоминания уже прошло!")
			}

		case "setup_reg":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "👋 Нажми кнопку ниже, чтобы пройти регистрацию!",
					Flags:   discordgo.MessageFlagsEphemeral, // <-- ДОБАВИТЬ ЭТУ СТРОКУ!
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
				},
			})
			return
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

			// Запрашиваем информацию о голосовании юзера из Базы Данных
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

			// Удаляем старое фидбек-сообщение
			if err == nil && oldFeedbackMsgID.Valid && oldFeedbackMsgID.String != "" {
				errDelete := s.ChannelMessageDelete(message.ChannelID, oldFeedbackMsgID.String)
				if errDelete != nil {
					log.Printf("Не удалось удалить старое фидбек-сообщение: %v", errDelete)
				}
			}

			// Редактируем сообщение пульта (обновляем счетчики)
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

			// Получаем ID отправленного сообщения
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

			// есть ли у пользователя уже роль

			member, err := s.GuildMember(i.GuildID, i.Member.User.ID)
			if err != nil {
				log.Printf("Не удалось получить информацию о пользователе: %v", err)
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "❌ Ошибка при проверке профиля. Попробуйте позже.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			// Проверяем, есть ли у пользователя целевая роль
			hasRole := false
			for _, roleID := range member.Roles {
				if roleID == TargetRoleID {
					hasRole = true
					break
				}
			}

			// Меняем ник
			err = s.GuildMemberNickname(i.GuildID, i.Member.User.ID, newNickname)
			if err != nil {
				log.Printf("Не удалось изменить ник пользователю %s: %v", i.Member.User.ID, err)
			}

			// Если роль ЕСТЬ - просто обновляем данные
			if hasRole {
				// Ответ для уже зарегистрированного пользователя
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("✅ **%s** обновил свои данные!\nНик изменён на: **%s**", i.Member.User.Mention(), newNickname),
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})

				// можно отправить сообщение в канал, что кто-то обновился
				if WelcomeChannelID != "" {
					s.ChannelMessageSend(WelcomeChannelID, fmt.Sprintf("📝 Лудик <@%s> обновил свой профиль! Новый ник нюхача: **%s**", i.Member.User.ID, newNickname))
				}
				return
			}

			// Если роли НЕТ - выдаём роль как нового игрока
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

// cleanupPoll удаляет сообщение опроса и все связанные с ним фидбек-сообщения
func cleanupPoll(s *discordgo.Session, pollMessageID, channelID string) {
	log.Printf("🧹 Начинаю очистку опроса %s", pollMessageID)

	// 1. Удаляем все фидбек-сообщения из базы данных
	rows, err := DB.Query("SELECT feedback_message_id FROM lobby_votes WHERE message_id = $1", pollMessageID)
	if err != nil {
		log.Printf("Ошибка получения фидбек-сообщений для очистки: %v", err)
		return
	}
	defer rows.Close()

	var feedbackIDs []string
	for rows.Next() {
		var feedbackID sql.NullString
		if err := rows.Scan(&feedbackID); err == nil && feedbackID.Valid && feedbackID.String != "" {
			feedbackIDs = append(feedbackIDs, feedbackID.String)
		}
	}
	// Проверяем ошибки после завершения цикла
	if err = rows.Err(); err != nil {
		log.Printf("Ошибка при итерации по результатам: %v", err)
	}

	// 2. Удаляем фидбек-сообщения из Discord
	for _, fbID := range feedbackIDs {
		if err := s.ChannelMessageDelete(channelID, fbID); err != nil {
			log.Printf("Не удалось удалить фидбек %s: %v", fbID, err)
		}
	}

	// 3. Удаляем само сообщение с опросом
	if err := s.ChannelMessageDelete(channelID, pollMessageID); err != nil {
		log.Printf("Не удалось удалить сообщение опроса %s: %v", pollMessageID, err)
	}

	// 4. Удаляем все записи из БД для этого опроса
	_, err = DB.Exec("DELETE FROM lobby_votes WHERE message_id = $1", pollMessageID)
	if err != nil {
		log.Printf("Ошибка удаления записей из БД: %v", err)
	}

	// 5. Отправляем уведомление в канал
	s.ChannelMessageSend(channelID, "🧹 **Канал очищен!** Все голоса и сообщения о сборе удалены.")

	log.Printf("✅ Очистка опроса %s завершена", pollMessageID)
}

// parseTimeFromInput парсит время из строки вида "19:00"
func parseTimeFromInput(input string) (time.Time, error) {
	layouts := []string{"15:04", "15:04:05"}
	var parsedTime time.Time
	var err error
	for _, layout := range layouts {
		parsedTime, err = time.Parse(layout, input)
		if err == nil {
			break
		}
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("не удалось распарсить время: %s", input)
	}

	now := time.Now()
	parsedTime = time.Date(now.Year(), now.Month(), now.Day(), parsedTime.Hour(), parsedTime.Minute(), 0, 0, now.Location())

	if parsedTime.Before(now) {
		parsedTime = parsedTime.Add(24 * time.Hour)
	}

	return parsedTime, nil
}

// sendReminderToAll отправляет личное сообщение всем, кто нажал "Я буду"
func sendReminderToAll(s *discordgo.Session, pollMessageID string) {
	// Получаем всех пользователей, которые нажали "lobby_go"
	rows, err := DB.Query("SELECT user_id FROM lobby_votes WHERE message_id = $1 AND current_choice = 'lobby_go'", pollMessageID)
	if err != nil {
		log.Printf("Ошибка получения списка игроков для напоминания: %v", err)
		return
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err == nil {
			userIDs = append(userIDs, userID)
		}
	}
	// Проверяем ошибки после завершения цикла
	if err = rows.Err(); err != nil {
		log.Printf("Ошибка при итерации по результатам: %v", err)
	}

	if len(userIDs) == 0 {
		log.Printf("Нет игроков для напоминания (никто не нажал 'Я буду')")
		return
	}

	log.Printf("📨 Отправляю напоминания %d игрокам...", len(userIDs))

	for _, userID := range userIDs {
		// Создаём личный канал с пользователем
		channel, err := s.UserChannelCreate(userID)
		if err != nil {
			log.Printf("Не удалось создать канал с пользователем %s: %v", userID, err)
			continue
		}

		// Отправляем сообщение
		_, err = s.ChannelMessageSend(channel.ID, fmt.Sprintf(
			"🔔 **Напоминание!**\n"+
				"🕐 Сбор начинается через **5 минут**!\n"+
				"🎮 Заходи, тебя все ждут!\n"+
				"🏃‍♂️ Не опаздывай!",
		))
		if err != nil {
			log.Printf("Не удалось отправить сообщение пользователю %s: %v", userID, err)
		} else {
			log.Printf("✅ Напоминание отправлено пользователю %s", userID)
		}
	}
}
