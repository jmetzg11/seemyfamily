package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"seemyfamily.jmetzg11/internal/models"
)

func TestShouldNotify(t *testing.T) {
	tests := []struct {
		name  string
		count int
		every int
		want  bool
	}{
		{"no rows", 0, 10, false},
		{"first of the day", 1, 10, true},
		{"second", 2, 10, false},
		{"ninth", 9, 10, false},
		{"tenth", 10, 10, true},
		{"eleventh", 11, 10, false},
		{"twentieth", 20, 10, true},
		{"first create", 1, 5, true},
		{"fifth create", 5, 5, true},
		{"sixth create", 6, 5, false},
		{"tenth create", 10, 5, true},
		{"first delete", 1, 1, true},
		{"second delete", 2, 1, true},
		{"every delete", 7, 1, true},
		{"no threshold", 4, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldNotify(tt.count, tt.every); got != tt.want {
				t.Errorf("shouldNotify(%d, %d) = %v; want %v", tt.count, tt.every, got, tt.want)
			}
		})
	}
}

// TestNotifyEveryCoversEveryKind guards against a kind reaching notify with no
// threshold, which shouldNotify reads as zero and silently never sends.
func TestNotifyEveryCoversEveryKind(t *testing.T) {
	for _, kind := range []models.Kind{models.KindCreate, models.KindEdit, models.KindDelete} {
		if notifyEvery[kind] < 1 {
			t.Errorf("kind %q has no threshold in notifyEvery", kind)
		}
		if kindNouns[kind][0] == "" || kindNouns[kind][1] == "" {
			t.Errorf("kind %q has no nouns in kindNouns", kind)
		}
	}
}

func TestNotifySummaryLine(t *testing.T) {
	user := models.User{ID: 7, Name: "jmetzg11"}

	tests := []struct {
		name  string
		kind  models.Kind
		count int
		want  string
	}{
		{"one edit", models.KindEdit, 1, "User jmetzg11 (pk=7) made 1 edit in the past 24 hours"},
		{"ten edits", models.KindEdit, 10, "User jmetzg11 (pk=7) made 10 edits in the past 24 hours"},
		{"one create", models.KindCreate, 1, "User jmetzg11 (pk=7) made 1 addition in the past 24 hours"},
		{"five creates", models.KindCreate, 5, "User jmetzg11 (pk=7) made 5 additions in the past 24 hours"},
		{"one delete", models.KindDelete, 1, "User jmetzg11 (pk=7) made 1 deletion in the past 24 hours"},
		{"three deletes", models.KindDelete, 3, "User jmetzg11 (pk=7) made 3 deletions in the past 24 hours"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subject, body := notifyMessage(user, tt.kind, tt.count, nil, time.Now())

			if want := "seemyfamily: " + tt.want; subject != want {
				t.Errorf("subject: got %q; want %q", subject, want)
			}
			if body != tt.want {
				t.Errorf("body: got %q; want %q; with no history there is nothing to list", body, tt.want)
			}
		})
	}
}

// TestNotifyMessageListsRecent pins the digest to offsets from send time. Nothing
// in the email may name a clock time or a date: the people reading it are in
// timezones the server knows nothing about.
func TestNotifyMessageListsRecent(t *testing.T) {
	user := models.User{ID: 7, Name: "jmetzg11"}
	now := time.Now()

	recent := []models.Edit{
		{CreatedAt: now.Add(-20 * time.Second), Action: models.ActionUpdated, Recipient: "Boris Vance"},
		{CreatedAt: now.Add(-25 * time.Minute), Action: models.ActionAdded("spouse"), Recipient: "Uncle Bob"},
		{CreatedAt: now.Add(-90 * time.Minute), Action: models.ActionPhoto, Recipient: "Aunt Jane"},
		{CreatedAt: now.Add(-23 * time.Hour), Action: models.ActionDeleted, Recipient: "Cousin Ann"},
	}

	_, body := notifyMessage(user, models.KindEdit, 10, recent, now)

	want := []string{
		"User jmetzg11 (pk=7) made 10 edits in the past 24 hours",
		"",
		"Everything jmetzg11 did:",
		"",
		"just now  updated details (Boris Vance)",
		"25m ago   added spouse (Uncle Bob)",
		"1h ago    added photo (Aunt Jane)",
		"23h ago   deleted profile (Cousin Ann)",
	}

	got := strings.Split(strings.TrimSuffix(body, "\n"), "\n")

	if !slices.Equal(got, want) {
		t.Errorf("body:\ngot  %q\nwant %q", got, want)
	}

	if strings.Contains(body, "most recent are listed") {
		t.Error("body claims it was truncated, but the list is shorter than recentLimit")
	}
}

func TestAgo(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"future", -5 * time.Minute, "just now"},
		{"zero", 0, "just now"},
		{"seconds", 42 * time.Second, "just now"},
		{"a minute", time.Minute, "1m ago"},
		{"minutes", 25*time.Minute + 30*time.Second, "25m ago"},
		{"just under an hour", 59*time.Minute + 59*time.Second, "59m ago"},
		{"an hour", time.Hour, "1h ago"},
		{"rounds down", 90 * time.Minute, "1h ago"},
		{"edge of the window", 23*time.Hour + 59*time.Minute, "23h ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ago(tt.d); got != tt.want {
				t.Errorf("ago(%v) = %q; want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestNotifyMessageSaysWhenTruncated(t *testing.T) {
	user := models.User{ID: 7, Name: "jmetzg11"}

	recent := make([]models.Edit, recentLimit)
	for i := range recent {
		recent[i] = models.Edit{CreatedAt: time.Now(), Action: models.ActionUpdated, Recipient: "Boris Vance"}
	}

	_, body := notifyMessage(user, models.KindEdit, 60, recent, time.Now())

	if !strings.Contains(body, "Only the 50 most recent are listed") {
		t.Errorf("body does not say it was truncated:\n%s", body)
	}
}
