package render

import (
	"fmt"
	"time"
)

var ruMonthsShort = [13]string{
	"", "янв", "фев", "мар", "апр", "май", "июн",
	"июл", "авг", "сен", "окт", "ноя", "дек",
}

var ruMonthsGenitive = [13]string{
	"", "января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря",
}

func formatDateRU(t time.Time, loc *time.Location) string {
	if loc == nil {
		loc = defaultLocation
	}
	lt := t.In(loc)
	return fmt.Sprintf("%d %s %d, %02d:%02d",
		lt.Day(), ruMonthsShort[lt.Month()], lt.Year(),
		lt.Hour(), lt.Minute())
}

func formatDayRU(t time.Time, loc *time.Location) string {
	if loc == nil {
		loc = defaultLocation
	}
	lt := t.In(loc)
	return fmt.Sprintf("%d %s %d",
		lt.Day(), ruMonthsShort[lt.Month()], lt.Year())
}

// formatBirthdateRU — дата рождения без учёта таймзоны: календарная дата.
func formatBirthdateRU(t time.Time) string {
	return fmt.Sprintf("%d %s %d",
		t.Day(), ruMonthsGenitive[t.Month()], t.Year())
}
