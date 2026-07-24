package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMonthYear(t *testing.T) {
	cases := map[string]struct {
		raw     string
		wantErr bool
		want    MonthYear
	}{
		"valid":            {raw: "07-2025", want: NewMonthYear(2025, time.July)},
		"december":         {raw: "12-2025", want: NewMonthYear(2025, time.December)},
		"spaces_trimmed":   {raw: "  01-2020  ", want: NewMonthYear(2020, time.January)},
		"empty":            {raw: "", wantErr: true},
		"month_out_of_range": {raw: "13-2025", wantErr: true},
		"wrong_order":      {raw: "2025-07", wantErr: true},
		"garbage":          {raw: "july", wantErr: true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseMonthYear(tc.raw)
			if tc.wantErr {
				assert.ErrorIs(t, err, ErrInvalidMonthYear)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestMonthYear_String(t *testing.T) {
	assert.Equal(t, "07-2025", NewMonthYear(2025, time.July).String())
	assert.Equal(t, "12-2025", NewMonthYear(2025, time.December).String())
}

func TestMonthYear_NormalizesToFirstDay(t *testing.T) {
	m := MonthYearOf(time.Date(2025, time.July, 23, 15, 4, 5, 0, time.UTC))
	assert.Equal(t, time.Date(2025, time.July, 1, 0, 0, 0, 0, time.UTC), m.Time)
	assert.Equal(t, NewMonthYear(2025, time.July), m)
}

func TestMonthYear_MonthsInclusive(t *testing.T) {
	july := NewMonthYear(2025, time.July)

	assert.Equal(t, 1, july.MonthsInclusive(july))
	assert.Equal(t, 6, july.MonthsInclusive(NewMonthYear(2025, time.December)))
	assert.Equal(t, 13, july.MonthsInclusive(NewMonthYear(2026, time.July)))
	assert.Equal(t, 0, july.MonthsInclusive(NewMonthYear(2025, time.June)))
}

func TestMonthYear_JSON(t *testing.T) {
	raw, err := json.Marshal(NewMonthYear(2025, time.July))
	require.NoError(t, err)
	assert.JSONEq(t, `"07-2025"`, string(raw))

	var m MonthYear
	require.NoError(t, json.Unmarshal([]byte(`"07-2025"`), &m))
	assert.Equal(t, NewMonthYear(2025, time.July), m)

	assert.Error(t, json.Unmarshal([]byte(`"2025"`), &m))
	assert.Error(t, json.Unmarshal([]byte(`123`), &m))
}

func TestSubscription_ActiveMonths(t *testing.T) {
	from := NewMonthYear(2025, time.July)
	to := NewMonthYear(2025, time.December)

	endOct := NewMonthYear(2025, time.October)
	endMay := NewMonthYear(2025, time.May)
	end2026 := NewMonthYear(2026, time.March)

	cases := map[string]struct {
		sub  Subscription
		want int
	}{
		"open_ended_covers_whole_period": {
			sub:  Subscription{StartDate: NewMonthYear(2024, time.January)},
			want: 6,
		},
		"starts_inside_period": {
			sub:  Subscription{StartDate: NewMonthYear(2025, time.September)},
			want: 4,
		},
		"ends_inside_period": {
			sub:  Subscription{StartDate: NewMonthYear(2025, time.July), EndDate: &endOct},
			want: 4,
		},
		"single_month": {
			sub:  Subscription{StartDate: to, EndDate: &to},
			want: 1,
		},
		"ends_before_period": {
			sub:  Subscription{StartDate: NewMonthYear(2025, time.January), EndDate: &endMay},
			want: 0,
		},
		"starts_after_period": {
			sub:  Subscription{StartDate: NewMonthYear(2026, time.January)},
			want: 0,
		},
		"ends_after_period": {
			sub:  Subscription{StartDate: NewMonthYear(2025, time.August), EndDate: &end2026},
			want: 5,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.sub.ActiveMonths(from, to))
		})
	}
}
