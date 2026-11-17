package importer

import (
	"testing"
	"time"
)

func TestParseReleaseDate(t *testing.T) {
	tests := []struct {
		name       string
		date       string
		comingSoon bool
		wantValid  bool
		want       time.Time
	}{
		{name: "day short month", date: "21 Nov, 2024", wantValid: true, want: time.Date(2024, time.November, 21, 0, 0, 0, 0, time.UTC)},
		{name: "day long month", date: "21 November, 2024", wantValid: true, want: time.Date(2024, time.November, 21, 0, 0, 0, 0, time.UTC)},
		{name: "short month day", date: "Nov 21, 2024", wantValid: true, want: time.Date(2024, time.November, 21, 0, 0, 0, 0, time.UTC)},
		{name: "long month day", date: "November 21, 2024", wantValid: true, want: time.Date(2024, time.November, 21, 0, 0, 0, 0, time.UTC)},
		{name: "month year", date: "Nov 2024", wantValid: true, want: time.Date(2024, time.November, 1, 0, 0, 0, 0, time.UTC)},
		{name: "year only", date: "2024", wantValid: true, want: time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)},
		{name: "first quarter", date: "Q1 2024", wantValid: true, want: time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)},
		{name: "second quarter", date: "Q2 2024", wantValid: true, want: time.Date(2024, time.April, 1, 0, 0, 0, 0, time.UTC)},
		{name: "third quarter", date: "Q3 2024", wantValid: true, want: time.Date(2024, time.July, 1, 0, 0, 0, 0, time.UTC)},
		{name: "fourth quarter", date: "Q4 2024", wantValid: true, want: time.Date(2024, time.October, 1, 0, 0, 0, 0, time.UTC)},
		{name: "lowercase quarter", date: "q4 2024", wantValid: true, want: time.Date(2024, time.October, 1, 0, 0, 0, 0, time.UTC)},
		{name: "extra whitespace", date: "  21   Nov,   2024 ", wantValid: true, want: time.Date(2024, time.November, 21, 0, 0, 0, 0, time.UTC)},
		{name: "coming soon flag overrides date", date: "21 Nov, 2024", comingSoon: true},
		{name: "coming soon text", date: "Coming Soon"},
		{name: "to be announced", date: "To Be Announced"},
		{name: "tba", date: "TBA"},
		{name: "empty", date: ""},
		{name: "whitespace only", date: "   "},
		{name: "garbage", date: "sometime next year"},
		{name: "invalid day", date: "31 Feb 2024"},
		{name: "invalid quarter", date: "Q5 2024"},
		{name: "invalid quarter year", date: "Q1 nope"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseReleaseDate(tt.date, tt.comingSoon)
			if got.Valid != tt.wantValid {
				t.Fatalf("parseReleaseDate(%q, %v).Valid = %v, want %v", tt.date, tt.comingSoon, got.Valid, tt.wantValid)
			}
			if tt.wantValid && !got.Time.Equal(tt.want) {
				t.Fatalf("parseReleaseDate(%q, %v) = %v, want %v", tt.date, tt.comingSoon, got.Time, tt.want)
			}
		})
	}
}
