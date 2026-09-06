package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"seemyfamily.jmetzg11/internal/models"
)

const (
	notifyTimeout = 30 * time.Second
	recentLimit   = 50
)

var notifyEvery = map[models.Kind]int{
	models.KindCreate: 5,
	models.KindEdit:   10,
	models.KindDelete: 1,
}

var kindNouns = map[models.Kind][2]string{
	models.KindCreate: {"addition", "additions"},
	models.KindEdit:   {"edit", "edits"},
	models.KindDelete: {"deletion", "deletions"},
}

func (app *application) notify(user models.User, kind models.Kind) {
	if !app.mailer.Configured() {
		return
	}

	go func() {
		defer func() {
			pv := recover()
			if pv != nil {
				app.logger.Error(fmt.Sprintf("%v", pv), "task", "notify", "kind", kind)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
		defer cancel()

		count, err := app.stats.RecentCount(ctx, user.Name, kind)
		if err != nil {
			app.logger.Error(err.Error(), "task", "notify", "kind", kind, "username", user.Name)
			return
		}

		if !shouldNotify(count, notifyEvery[kind]) {
			return
		}

		recent, err := app.stats.RecentByUser(ctx, user.Name, recentLimit)
		if err != nil {
			app.logger.Error(err.Error(), "task", "notify", "kind", kind, "username", user.Name)
		}

		subject, body := notifyMessage(user, kind, count, recent, time.Now())

		err = app.mailer.Send(nil, subject, body)
		if err != nil {
			app.logger.Error(err.Error(), "task", "notify", "kind", kind, "username", user.Name)
		}
	}()
}

func shouldNotify(count, every int) bool {
	if count < 1 || every < 1 {
		return false
	}

	return count == 1 || count%every == 0
}

func notifyMessage(user models.User, kind models.Kind, count int, recent []models.Edit, now time.Time) (string, string) {
	noun := kindNouns[kind][0]
	if count != 1 {
		noun = kindNouns[kind][1]
	}

	summary := fmt.Sprintf("User %s (pk=%d) made %d %s in the past 24 hours", user.Name, user.ID, count, noun)

	var b strings.Builder

	b.WriteString(summary)

	if len(recent) > 0 {
		fmt.Fprintf(&b, "\n\nEverything %s did:\n\n", user.Name)

		for _, e := range recent {
			fmt.Fprintf(&b, "%-9s %s (%s)\n", ago(now.Sub(e.CreatedAt)), e.Action, e.Recipient)
		}

		if len(recent) == recentLimit {
			fmt.Fprintf(&b, "\nOnly the %d most recent are listed; /info has the rest.\n", recentLimit)
		}
	}

	return "seemyfamily: " + summary, b.String()
}

func ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}
