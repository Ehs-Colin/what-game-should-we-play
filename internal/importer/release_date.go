package importer

import (
	"database/sql"
	"strconv"
	"strings"
	"time"
)

// releaseDateLayouts are the normalized Steam release-date formats we can
// parse. More specific formats come first.
var releaseDateLayouts = []string{
	"2 Jan 2006",
	"2 January 2006",
	"Jan 2 2006",
	"January 2 2006",
	"Jan 2006",
	"2006",
}

// parseReleaseDate converts Steam's free-form release date into a date.
// It returns an invalid NullTime when the game is not confirmed released or
// the text does not match a known format.
func parseReleaseDate(date string, comingSoon bool) sql.NullTime {
	if comingSoon {
		return sql.NullTime{}
	}

	normalized := strings.Join(strings.Fields(strings.ReplaceAll(date, ",", "")), " ")
	switch strings.ToLower(normalized) {
	case "", "coming soon", "to be announced", "tba":
		return sql.NullTime{}
	}

	if parsed, ok := parseQuarter(normalized); ok {
		return parsed
	}

	for _, layout := range releaseDateLayouts {
		parsed, err := time.Parse(layout, normalized)
		if err == nil {
			return sql.NullTime{Time: parsed.UTC(), Valid: true}
		}
	}
	return sql.NullTime{}
}

// parseQuarter handles "Q1 2006" through "Q4 2006", returning the first day of
// the quarter.
func parseQuarter(date string) (sql.NullTime, bool) {
	if len(date) != 7 || (date[0] != 'Q' && date[0] != 'q') || date[2] != ' ' {
		return sql.NullTime{}, false
	}
	quarter := date[1]
	if quarter < '1' || quarter > '4' {
		return sql.NullTime{}, false
	}
	year, err := strconv.Atoi(date[3:])
	if err != nil {
		return sql.NullTime{}, false
	}
	month := time.Month((int(quarter-'0')-1)*3 + 1)
	return sql.NullTime{
		Time:  time.Date(year, month, 1, 0, 0, 0, 0, time.UTC),
		Valid: true,
	}, true
}
