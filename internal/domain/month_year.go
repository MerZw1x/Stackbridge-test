package domain

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// MonthYearLayout — формат даты подписки: месяц и год, например "07-2025".
const MonthYearLayout = "01-2006"

var ErrInvalidMonthYear = errors.New(`invalid date: expected format "MM-YYYY"`)

// MonthYear — дата с точностью до месяца. Внутри всегда нормализована
// к первому дню месяца в UTC, поэтому две даты одного месяца всегда равны.
type MonthYear struct {
	time.Time
}

func NewMonthYear(year int, month time.Month) MonthYear {
	return MonthYear{time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)}
}

// MonthYearOf отбрасывает у времени всё, что мельче месяца.
func MonthYearOf(t time.Time) MonthYear {
	return NewMonthYear(t.Year(), t.Month())
}

func ParseMonthYear(s string) (MonthYear, error) {
	t, err := time.Parse(MonthYearLayout, strings.TrimSpace(s))
	if err != nil {
		return MonthYear{}, ErrInvalidMonthYear
	}
	return MonthYearOf(t), nil
}

func (m MonthYear) String() string {
	return m.Format(MonthYearLayout)
}

// Index — порядковый номер месяца в абсолютной шкале, удобен для арифметики.
func (m MonthYear) Index() int {
	return m.Year()*12 + int(m.Month()) - 1
}

// MonthsInclusive — количество месяцев в отрезке [m, other] с учётом обеих границ.
// Для одного и того же месяца возвращает 1, для отрезка "назад" — неположительное число.
func (m MonthYear) MonthsInclusive(other MonthYear) int {
	return other.Index() - m.Index() + 1
}

func (m MonthYear) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(m.String())), nil
}

func (m *MonthYear) UnmarshalJSON(data []byte) error {
	raw, err := strconv.Unquote(string(data))
	if err != nil {
		return ErrInvalidMonthYear
	}

	parsed, err := ParseMonthYear(raw)
	if err != nil {
		return err
	}

	*m = parsed
	return nil
}
