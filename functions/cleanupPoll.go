package functions

import (
	"database/sql"
	"log"

	"github.com/bwmarrin/discordgo"
)

// удаляет сообщение опроса и все связанные с ним фидбек-сообщения
func CleanupPoll(s *discordgo.Session, pollMessageID, channelID string, DB *sql.DB) {
	log.Printf("🧹 Начинаю очистку опроса %s", pollMessageID)

	// Удаляем все фидбек-сообщения из базы данных
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

	// Удаляем фидбек-сообщения из Discord
	for _, fbID := range feedbackIDs {
		if err := s.ChannelMessageDelete(channelID, fbID); err != nil {
			log.Printf("Не удалось удалить фидбек %s: %v", fbID, err)
		}
	}

	// Удаляем само сообщение с опросом
	if err := s.ChannelMessageDelete(channelID, pollMessageID); err != nil {
		log.Printf("Не удалось удалить сообщение опроса %s: %v", pollMessageID, err)
	}

	// Удаляем все записи из БД для этого опроса
	_, err = DB.Exec("DELETE FROM lobby_votes WHERE message_id = $1", pollMessageID)
	if err != nil {
		log.Printf("Ошибка удаления записей из БД: %v", err)
	}

	// Отправляем уведомление в канал
	s.ChannelMessageSend(channelID, "🧹 **Канал очищен!** Все голоса и сообщения о сборе удалены.")

	log.Printf("✅ Очистка опроса %s завершена", pollMessageID)
}
