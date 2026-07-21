package functions

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

// Вспомогательная функция, которая рисует сообщение с кнопкой регистрации
func SendRegButton(s *discordgo.Session, userID string, channelID string) {
	content := "Приветствую на сервере УДОВОЛЬСТВИЕ! Нажми кнопку ниже, чтобы пройти регистрацию."
	if userID != "" {
		content = fmt.Sprintf("👋 Приветствуем нового лудика, <@%s>! Нажми кнопку ниже, чтобы пройти регистрацию и получить по заслугам.", userID)
	}

	_, err := s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: content,
		Flags:   discordgo.MessageFlagsEphemeral,
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
