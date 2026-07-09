package main

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

// Авто-приветствие при входе
func HandleUserJoin(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if WelcomeChannelID == "" {
		log.Println("Ошибка: Не задан WELCOME_CHANNEL_ID в .env")
		return
	}
	sendRegButton(s, m.User.ID, WelcomeChannelID)
}

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

func HandleInteractions(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// 1. СЛЕШ-КОМАНДЫ
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
				// Берем первый элемент среза [0] и вытаскиваем из него строку
				inputTime = options[0].StringValue()
			}

			msgText := fmt.Sprintf("🔔 **СБОР НА КАТКУ!** <@&%s>\n📊 Собираем лобби 5х5 в **%s**", DotaRoleID, inputTime)

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
							discordgo.Button{Label: "Газ", Style: discordgo.SuccessButton, CustomID: "lobby_go", Emoji: &discordgo.ComponentEmoji{Name: "✅"}},
							discordgo.Button{Label: "Я долбоеб", Style: discordgo.DangerButton, CustomID: "lobby_clown", Emoji: &discordgo.ComponentEmoji{Name: "🤡"}},
							discordgo.Button{Label: "Позже", Style: discordgo.SecondaryButton, CustomID: "lobby_later", Emoji: &discordgo.ComponentEmoji{Name: "⏳"}},
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

	// 2. КНОПКИ
	if i.Type == discordgo.InteractionMessageComponent {
		customID := i.MessageComponentData().CustomID

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

		switch customID {
		case "lobby_go":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: fmt.Sprintf("✅ Игрок <@%s> подтвердил, что готов катать!", i.Member.User.ID)},
			})
		case "lobby_clown":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: fmt.Sprintf("🤡 <@%s> нажал кнопку 'Я долбоеб' и слился с катки.", i.Member.User.ID)},
			})
		case "lobby_later":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: fmt.Sprintf("⏳ <@%s> просит подождать его, подлетит позже.", i.Member.User.ID)},
			})
		}
		return
	}

	// 3. МОДАЛЬНЫЕ ОКНА
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

			_ = s.GuildMemberNickname(i.GuildID, i.Member.User.ID, newNickname)
			_ = s.GuildMemberRoleAdd(i.GuildID, i.Member.User.ID, TargetRoleID)

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("🎉 Встречайте нового фрика — **%s**!\nРоль успешно выдана, добро пожаловать в семью! 🎰", newNickname),
				},
			})
		}
	}
}
