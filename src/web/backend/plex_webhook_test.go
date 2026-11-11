package backend

import "testing"

func TestRatedTrack(t *testing.T) {
	payload := func(event, typ, lib string, rating int) string {
		return `{"event":"` + event + `","Metadata":{"type":"` + typ + `","librarySectionTitle":"` + lib +
			`","title":"Numb","parentTitle":"Meteora","grandparentTitle":"Linkin Park","userRating":` + string(rune('0'+rating)) + `}}`
	}
	tests := []struct {
		name, payload string
		want          bool
	}{
		{"rated in Explo", payload("media.rate", "track", "Explo", 8), true},
		{"main library", payload("media.rate", "track", "Music", 8), false},
		{"rating removed", payload("media.rate", "track", "Explo", 0), false},
		{"other event", payload("media.play", "track", "Explo", 8), false},
		{"album rated", payload("media.rate", "album", "Explo", 8), false},
		{"garbage", "not json", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		got := ratedTrack(tt.payload, "Explo")
		if (got != nil) != tt.want {
			t.Errorf("%s: got %v, want match=%v", tt.name, got, tt.want)
		}
		if got != nil && (got.Album != "Meteora" || got.MainArtist != "Linkin Park") {
			t.Errorf("%s: wrong track %+v", tt.name, got)
		}
	}
}
