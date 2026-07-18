package functions

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

func LobbyPoll(s *discordgo.Session, i *discordgo.InteractionCreate, customID string, db *sql.DB) {
	message := i.Message
	userID := i.Member.User.ID

	var currentChoice string
	var switchCount int
	var oldFeedbackMsgID sql.NullString

	// Запрашиваем информацию о голосовании юзера из Базы Данных
	err := db.QueryRow("SELECT current_choice, switch_count, feedback_message_id FROM lobby_votes WHERE message_id = $1 AND user_id = $2", message.ID, userID).
		Scan(&currentChoice, &switchCount, &oldFeedbackMsgID)

	// если пытается переголосовать уже во ВТОРОЙ раз (switch_count >= 1)
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
				button.Label = UpdateButtonLabel(button.CustomID, count)
			}
		}

		// Увеличиваем счетчик у НОВОГО нажатого варианта
		if button.CustomID == customID {
			matches := re.FindStringSubmatch(button.Label)
			if len(matches) > 1 {
				count, _ := strconv.Atoi(matches[1])
				count++
				button.Label = UpdateButtonLabel(button.CustomID, count)
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
	switch err {
	case sql.ErrNoRows:
		// Первое нажатие
		_, errInsert := db.Exec("INSERT INTO lobby_votes (message_id, user_id, current_choice, switch_count, feedback_message_id) VALUES ($1, $2, $3, $4, $5)",
			message.ID, userID, customID, 0, newFeedbackID)
		if errInsert != nil {
			log.Printf("Ошибка INSERT в БД: %v", errInsert)
		}

	case nil:
		// Переголосование
		_, errUpdate := db.Exec("UPDATE lobby_votes SET current_choice = $1, switch_count = switch_count + 1, feedback_message_id = $2 WHERE message_id = $3 AND user_id = $4",
			customID, newFeedbackID, message.ID, userID)
		if errUpdate != nil {
			log.Printf("Ошибка UPDATE в БД: %v", errUpdate)
		}

	default:
		log.Printf("Необработанная ошибка: %v", err)
	}
}
