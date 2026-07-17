package functions

import "fmt"

func UpdateButtonLabel(customID string, count int) string {
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
