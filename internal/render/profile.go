package render

import (
	"fmt"
	"time"
)

const defaultAvatarURL = "/static/img/avatar_default.svg"

// Presence форматирует last_seen_at в человекочитаемую строку для шапки профиля.
// nil → «давно», <5 мин → «онлайн», иначе «был N мин/ч/д назад».
func Presence(t *time.Time) string {
	if t == nil {
		return "давно не заходил"
	}
	d := time.Since(*t)
	switch {
	case d < 5*time.Minute:
		return "онлайн"
	case d < time.Hour:
		return fmt.Sprintf("был %d мин назад", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("был %d ч назад", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("был %d дн назад", int(d.Hours()/24))
	default:
		return "был " + t.Local().Format("2 Jan 2006")
	}
}

func FormatBirthDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2 January 2006")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func GenderLabel(g string) string {
	switch g {
	case "male":
		return "мужской"
	case "female":
		return "женский"
	default:
		return "не указан"
	}
}
