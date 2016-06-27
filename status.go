package main

import (
	"fmt"
	"time"

	"github.com/jason0x43/go-alfred"
)

// StatusFilter is a command
type StatusFilter struct{}

// About returns information about this command
func (c StatusFilter) About() *alfred.CommandDef {
	return &alfred.CommandDef{
		Keyword:     "status",
		Description: "Show current status",
		WithSpace:   false,
	}
}

// IsEnabled returns true if the command is enabled
func (c StatusFilter) IsEnabled() bool {
	return true
}

// Items returns a list of filter items
func (c StatusFilter) Items(arg, data string) (items []*alfred.Item, err error) {
	if err = refresh(); err != nil {
		items = append(items, &alfred.Item{
			Title:    "Error syncing with toggle.com",
			Subtitle: fmt.Sprintf("%v", err),
		})
		return
	}

	if entry, found := getRunningTimer(); found {
		startTime := entry.StartTime().Local()
		seconds := int64(time.Now().Sub(startTime).Seconds())
		duration := float32(seconds) / float32(60*60)
		date := toHumanDateString(startTime)
		time := startTime.Format("15:04:05")

		items = append(items, &alfred.Item{
			Title: entry.Description,
			Subtitle: fmt.Sprintf("%.2f hr, started %s at %s", duration, date,
				time),
			Icon: "running.png",
			// Arg:  fmt.Sprintf("timers -toggle=%d", entry.ID),
		})
	} else {
		items = append(items, &alfred.Item{
			Title: "No timers currently running",
		})
	}

	return
}
