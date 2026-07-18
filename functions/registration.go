package functions

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

// обрабатывает регистрацию кооп-игрока
func ProcessCoopRegistration(s *discordgo.Session, i *discordgo.InteractionCreate, realName string, db *sql.DB, dotaRoleID, coopRoleID, welcomeChannelID string) {
	if realName == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Имя не может быть пустым!",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	newNickname := fmt.Sprintf("%s | Coop", realName)

	if len([]rune(newNickname)) > 32 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Ник слишком длинный! Сократи имя.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

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

	hasDotaRole := false
	hasCoopRole := false
	for _, roleID := range member.Roles {
		if roleID == dotaRoleID {
			hasDotaRole = true
		}
		if roleID == coopRoleID {
			hasCoopRole = true
		}
	}

	isOldMember := hasDotaRole || hasCoopRole

	err = s.GuildMemberNickname(i.GuildID, i.Member.User.ID, newNickname)
	if err != nil {
		log.Printf("Не удалось изменить ник пользователю %s: %v", i.Member.User.ID, err)
	}

	if isOldMember {
		_ = s.GuildMemberRoleAdd(i.GuildID, i.Member.User.ID, coopRoleID)
		_ = s.GuildMemberRoleRemove(i.GuildID, i.Member.User.ID, dotaRoleID)

		var updateNotification string
		if hasDotaRole && !hasCoopRole {
			updateNotification = fmt.Sprintf("📝 Лудик <@%s> перешёл в кооп-команду! Теперь только мужские игры 🤝🍆. Новый ник: **%s**", i.Member.User.ID, newNickname)
		} else {
			updateNotification = fmt.Sprintf("📝 Лудик <@%s> обновил свой кооп-профиль! Новый ник: **%s**", i.Member.User.ID, newNickname)
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})

		if welcomeChannelID != "" {
			s.ChannelMessageSend(welcomeChannelID, updateNotification)
		}
		return
	}

	err = s.GuildMemberRoleAdd(i.GuildID, i.Member.User.ID, coopRoleID)
	if err != nil {
		log.Printf("Не удалось выдать кооп-роль пользователю %s: %v", i.Member.User.ID, err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Не удалось выдать роль.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("🎉 Встречайте нового кооп-фрика — **%s**!\nРоль успешно выдана, добро пожаловать в семью! 🤝🍆", newNickname),
		},
	})
}

// обрабатывает регистрацию дотера
func ProcessDotaRegistration(s *discordgo.Session, i *discordgo.InteractionCreate, dotaNick, realName, dotaMMR string, modalID string, db *sql.DB, dotaRoleID, coopRoleID, welcomeChannelID string) {
	mmrValue, err := strconv.Atoi(dotaMMR)
	if err != nil || mmrValue < 0 || mmrValue > 15000 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Слышь, фрик, в поле MMR нужно ввести нормальное число (например, 322). Не выебывайся.",
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
				Content: "❌ Я не умею читать столько букв подряд:(",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	member, err := s.GuildMember(i.GuildID, i.Member.User.ID)
	if err != nil {
		log.Printf("Не удалось получить информацию о пользователе: %v", err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Я с первого раза не понял, давай по-новой",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	hasDotaRole := false
	hasCoopRole := false
	for _, roleID := range member.Roles {
		if roleID == dotaRoleID {
			hasDotaRole = true
		}
		if roleID == coopRoleID {
			hasCoopRole = true
		}
	}

	isOldMember := hasDotaRole || hasCoopRole

	if isOldMember {
		var updateNotification string

		err = s.GuildMemberNickname(i.GuildID, i.Member.User.ID, newNickname)
		if err != nil {
			log.Printf("Не удалось изменить ник пользователю %s: %v", i.Member.User.ID, err)
		}

		if modalID == "registration_modal_both" {
			_ = s.GuildMemberRoleAdd(i.GuildID, i.Member.User.ID, dotaRoleID)
			_ = s.GuildMemberRoleAdd(i.GuildID, i.Member.User.ID, coopRoleID)

			if hasDotaRole && !hasCoopRole {
				updateNotification = fmt.Sprintf("📝 Лудик <@%s> обновил профиль! Теперь этот перец играет еще и в мужские игры (Coop) 🤝🍆. Новый ник: **%s**", i.Member.User.ID, newNickname)
			} else if !hasDotaRole && hasCoopRole {
				updateNotification = fmt.Sprintf("📝 Лудик <@%s> обновил профиль! Кооп-боярин решил замарать руки и подраться на поляне ⚔️! Новый ник: **%s**", i.Member.User.ID, newNickname)
			} else {
				updateNotification = fmt.Sprintf("📝 Лудик <@%s> обновил свой профиль! Новый ник: **%s**", i.Member.User.ID, newNickname)
			}
		} else {
			_ = s.GuildMemberRoleAdd(i.GuildID, i.Member.User.ID, dotaRoleID)
			_ = s.GuildMemberRoleRemove(i.GuildID, i.Member.User.ID, coopRoleID)

			if hasCoopRole {
				updateNotification = fmt.Sprintf("📝 Лудик <@%s> обновил профиль! Теперь он будет драться только на поляне ⚔️. Новый ник: **%s**", i.Member.User.ID, newNickname)
			} else {
				updateNotification = fmt.Sprintf("📝 Лудик <@%s> обновил свой профиль! Новый ник нюхача: **%s**", i.Member.User.ID, newNickname)
			}
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})

		if welcomeChannelID != "" && updateNotification != "" {
			s.ChannelMessageSend(welcomeChannelID, updateNotification)
		}
		return
	}

	err = s.GuildMemberNickname(i.GuildID, i.Member.User.ID, newNickname)
	if err != nil {
		log.Printf("Не удалось изменить ник новому пользователю %s: %v", i.Member.User.ID, err)
	}

	err = s.GuildMemberRoleAdd(i.GuildID, i.Member.User.ID, dotaRoleID)
	if err != nil {
		log.Printf("Не удалось выдать роль Dota пользователю %s: %v", i.Member.User.ID, err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Не удалось выдать роль.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if modalID == "registration_modal_both" {
		_ = s.GuildMemberRoleAdd(i.GuildID, i.Member.User.ID, coopRoleID)
	}

	welcomeText := fmt.Sprintf("🎉 Встречайте нового фрика — **%s**!\nРоль успешно выдана, добро пожаловать в семью! 🎰", newNickname)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: welcomeText,
		},
	})
}
