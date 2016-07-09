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
	dlog.Printf("status items with arg=%s, data=%s", arg, data)

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
		subtitle := fmt.Sprintf("%.2f hr, started %s at %s", duration, date,
			time)

		if project, _, ok := getProjectByID(entry.Pid); ok {
			subtitle = "[" + project.Name + "] " + subtitle
		}

		item := &alfred.Item{
			Title:    entry.Description,
			Subtitle: subtitle,
			Icon:     "running.png",
			Arg: &alfred.ItemArg{
				Keyword: "timers",
				Data:    alfred.Stringify(timerCfg{Timer: &entry.ID}),
			},
		}

		item.AddMod(alfred.ModCmd, &alfred.ItemMod{
			Subtitle: "Stop this timer",
			Arg: &alfred.ItemArg{
				Keyword: "timers",
				Mode:    alfred.ModeDo,
				Data:    alfred.Stringify(timerCfg{ToToggle: &entry.ID}),
			},
		})

		items = append(items, item)
	} else {
		items = append(items, &alfred.Item{
			Title: "No timers currently running",
		})
	}

	span, _ := getSpan("today")
	var report *summaryReport
	report, err = generateReport(span.Start, span.End, -1)
	for _, date := range report.dates {
		items = append(items, &alfred.Item{
			Title: fmt.Sprintf("Total time for today: %.2f", float32(date.total)/100.0),
		})
		break
	}

	return
}
