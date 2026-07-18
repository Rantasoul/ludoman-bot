package functions

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

// отправляет личное сообщение всем, кто нажал "Я буду"
func SendReminderToAll(s *discordgo.Session, pollMessageID string, DB *sql.DB) {
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
