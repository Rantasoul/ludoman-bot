package functions

import (
	"fmt"
	"time"
)

// парсит время из строки вида "19:00"
func ParseTimeFromInput(input string) (time.Time, error) {
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
