package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-toggl"
)

// entry -------------------------------------------------

// TimeEntryCommand is a command
type TimeEntryCommand struct{}

// Keyword returns the command's keyword
func (c TimeEntryCommand) Keyword() string {
	return "timers"
}

// IsEnabled returns true if the command is enabled
func (c TimeEntryCommand) IsEnabled() bool {
	return config.APIKey != ""
}

// MenuItem returns the command's menu item
func (c TimeEntryCommand) MenuItem() alfred.Item {
	return alfred.NewKeywordItem(c.Keyword(), "List and modify recent time entries, add new ones")
}

// Items returns a list of filter items
func (c TimeEntryCommand) Items(args []string) (items []alfred.Item, err error) {
	dlog.Printf("timers args: %#v", args)

	if err = checkRefresh(); err != nil {
		return items, err
	}

	var projectID int
	var timerID int
	var property string

	flags := flag.NewFlagSet("timerFlags", flag.ContinueOnError)
	flags.IntVar(&projectID, "project", -1, "Project ID")
	flags.IntVar(&timerID, "timer", -1, "Timer ID")
	flags.StringVar(&property, "property", "", "Timer property")
	flags.Parse(args)

	query := flags.Arg(0)
	dlog.Printf("query: %s", query)

	var entries []toggl.TimeEntry

	if timerID != -1 {
		// do someting with a specific time entry
		if entry, ok := findTimerByID(timerID); ok {
			items, err = timerEntryItems(entry, query)
			return
		}
	} else if projectID != -1 {
		// filter by project ID
		entries = findTimersByProjectID(projectID)
		dlog.Printf("found %d timers for project %d", len(entries), projectID)
	} else {
		entries = cache.Account.Data.TimeEntries
		dlog.Printf("showing all %d timers", len(entries))
	}

	if len(entries) == 0 {
		if query != "" {
			items = append(items, alfred.Item{
				Title:    query,
				Subtitle: "New entry",
				Arg:      "start " + alfred.MustMakeDataArg(startMessage{Description: query}),
			})
		}
	} else {
		var filtered []toggl.TimeEntry
		for _, entry := range entries {
			if alfred.FuzzyMatches(entry.Description, query) {
				filtered = append(filtered, entry)
			}
		}

		sort.Sort(sort.Reverse(byTime(filtered)))

		for _, entry := range filtered {
			item := alfred.Item{
				Title:        entry.Description,
				Arg:          fmt.Sprintf("%d", entry.Id),
				Autocomplete: entry.Description,
			}

			item.Mods = append(item.Mods, alfred.ItemMod{
				Key:      "cmd",
				Arg:      fmt.Sprintf("-toggle=%d", entry.Id),
				Subtitle: "Toggle this timer's running status",
			})

			var seconds int64

			startTime := entry.StartTime()
			if entry.Duration < 0 {
				seconds = int64(time.Now().Sub(startTime).Seconds())
			} else {
				seconds = entry.Duration
			}

			duration := float32(roundDuration(seconds)) / 100.0

			item.Subtitle = fmt.Sprintf("%.2f, %s from %s to", duration,
				toHumanDateString(startTime), startTime.Local().Format("3:04pm"))

			if entry.Duration < 0 {
				item.Subtitle += "now"
			} else if !entry.StopTime().IsZero() {
				item.Subtitle += entry.StopTime().Local().Format("3:04pm")
			} else {
				dlog.Printf("No duration or stop time")
			}

			if project, ok := findProjectByID(entry.Pid); ok {
				item.Subtitle = "[" + project.Name + "] " + item.Subtitle
			}

			if entry.IsRunning() {
				item.Icon = "running.png"
			}

			items = append(items, item)
		}
	}

	items = alfred.SortItemsForKeyword(items, query)
	return
}

// Do runs the command
func (c TimeEntryCommand) Do(args []string) (out string, err error) {
	var toUpdate string

	flags := flag.NewFlagSet("timerFlags", flag.ContinueOnError)
	flags.StringVar(&toUpdate, "update", "", "Updated time entry data")
	flags.Parse(args)

	if toUpdate != "" {
		dlog.Printf("updating time entry %v", toUpdate)
		var timer toggl.TimeEntry
		if timer, err = updateTimeEntry(toUpdate); err != nil {
			return
		}
		return fmt.Sprintf(`Updated time entry "%s"`, timer.Description), nil
	}

	return "Unrecognized input", nil
}

// support -------------------------------------------------------------------

// Do runs the command
func updateTimeEntry(dataString string) (entry toggl.TimeEntry, err error) {
	if err = json.Unmarshal([]byte(dataString), &entry); err != nil {
		return
	}

	session := toggl.OpenSession(config.APIKey)

	if entry, err = session.UpdateTimeEntry(entry); err != nil {
		return
	}

	adata := &cache.Account.Data

	for i, e := range adata.TimeEntries {
		if e.Id == entry.Id {
			adata.TimeEntries[i] = entry
			if err := alfred.SaveJSON(cacheFile, &cache); err != nil {
				dlog.Printf("Error saving cache: %v\n", err)
			}
			break
		}
	}

	return
}

// support -----------------------------------------------

func getNewTime(original, new time.Time) time.Time {
	originalMinutes := original.Hour()*60 + original.Minute()
	newMinutes := new.Hour()*60 + new.Minute()
	delta, _ := time.ParseDuration(fmt.Sprintf("%dm", newMinutes-originalMinutes))
	return original.Add(delta)
}

func timerEntryItems(entry toggl.TimeEntry, query string) (items []alfred.Item, err error) {
	if alfred.PartiallyPrefixes("description:", query) {
		item := alfred.Item{}
		parts := alfred.CleanSplitN(query, " ", 2)

		if len(parts) > 1 {
			newDesc := parts[1]
			updateEntry := entry
			updateEntry.Description = newDesc
			item.Title = "Description: " + newDesc
			item.Subtitle = "Description: " + entry.Description
			item.Arg = "-update " + alfred.MustMakeDataArg(updateEntry)
		} else {
			item.Title = "Description: " + entry.Description
			item.Subtitle = "Update this entry's description"
			item.Autocomplete = "Description: "
			item.Invalid = true
		}

		items = append(items, item)
	}

	if alfred.PartiallyPrefixes("project:", query) {
		command := "Project"
		parts := alfred.CleanSplitN(query, " ", 2)

		if strings.ToLower(parts[0]) == "project:" {
			var name string

			if len(parts) > 1 {
				name = parts[1]
			}

			for _, proj := range cache.Account.Data.Projects {
				if alfred.FuzzyMatches(proj.Name, name) {
					updateEntry := entry
					if entry.Pid == proj.Id {
						updateEntry.Pid = 0
					} else {
						updateEntry.Pid = proj.Id
					}
					item := alfred.Item{
						Title:        proj.Name,
						Autocomplete: command + ": " + proj.Name,
						Arg:          "-update " + alfred.MustMakeDataArg(updateEntry),
					}
					items = append(items, alfred.MakeChoice(item, entry.Pid == proj.Id))
				}
			}
		} else {
			item := alfred.Item{
				Title:        command + ": ",
				Subtitle:     "Change the project this entry is assigned to",
				Autocomplete: command + ": ",
				Invalid:      true,
			}
			if project, ok := findProjectByID(entry.Pid); ok {
				item.Title += project.Name
			} else {
				item.Title += "<None>"
			}
			items = append(items, item)
		}
	}

	if alfred.PartiallyPrefixes("tags:", query) {
		command := "Tags"
		parts := alfred.CleanSplitN(query, " ", 2)

		if strings.ToLower(parts[0]) == "tags:" {
			var tagName string

			if len(parts) > 1 {
				tagName = parts[1]
			}

			for _, tag := range cache.Account.Data.Tags {
				if alfred.FuzzyMatches(tag.Name, tagName) {
					item := alfred.MakeChoice(alfred.Item{
						Title:        tag.Name,
						Autocomplete: tag.Name,
					}, entry.HasTag(tag.Name))

					updateEntry := entry.Copy()
					if entry.HasTag(tag.Name) {
						updateEntry.RemoveTag(tag.Name)
					} else {
						updateEntry.AddTag(tag.Name)
					}
					item.Arg = "-update " + alfred.MustMakeDataArg(updateEntry)

					items = append(items, item)
				}
			}
		} else {
			item := alfred.Item{
				Title:        command + ": ",
				Subtitle:     "Update tags",
				Autocomplete: command + ": ",
				Invalid:      true,
			}
			if len(entry.Tags) > 0 {
				item.Title += strings.Join(entry.Tags, ", ")
			} else {
				item.Title += "<None>"
			}
			items = append(items, item)
		}
	}

	if alfred.PartiallyPrefixes("start:", query) {
		command := "Start"
		parts := alfred.CleanSplitN(query, " ", 2)

		var startTime string
		if !entry.StartTime().IsZero() {
			startTime = entry.StartTime().Local().Format("15:04")
		}

		item := alfred.Item{
			Title:        command + ": " + startTime,
			Invalid:      true,
			Autocomplete: command + ": ",
			Subtitle:     "Change the start time",
		}

		if strings.ToLower(parts[0]) == "start:" {
			item.Subtitle = "Change start time"
		}

		if len(parts) > 1 {
			timeStr := parts[1]

			if newTime, err := time.Parse("15:04", timeStr); err == nil {
				newStart := getNewTime(entry.StartTime().Local(), newTime)

				updateTimer := toggl.TimeEntry{
					Id:    entry.Id,
					Start: &newStart,
				}

				if !entry.IsRunning() {
					updateTimer.Duration = entry.Duration
				}

				item.Title = command + ": " + timeStr
				item.Subtitle = "Press enter to change start time (end time will also be adjusted)"
				item.Invalid = false
				item.Arg = "-update " + alfred.MustMakeDataArg(updateTimer)
			} else {
				dlog.Printf("Invalid time: %s\n", timeStr)
			}
		}

		items = append(items, item)
	}

	if !entry.IsRunning() {
		if alfred.PartiallyPrefixes("stop:", query) {
			command := "Stop"
			parts := alfred.CleanSplitN(query, " ", 2)

			var stopTime string
			if !entry.StopTime().IsZero() {
				stopTime = entry.StopTime().Local().Format("15:04")
			}

			item := alfred.Item{
				Title:        command + ": " + stopTime,
				Invalid:      true,
				Autocomplete: command + ": ",
				Subtitle:     "Change the stop time",
			}

			if strings.ToLower(parts[0]) == "stop:" {
				item.Subtitle = "Change stop time"
			}

			if len(parts) > 1 {
				timeStr := parts[1]

				if newTime, err := time.Parse("15:04", timeStr); err == nil {
					newStop := getNewTime(entry.StopTime().Local(), newTime)

					updateTimer := toggl.TimeEntry{
						Id:       entry.Id,
						Stop:     &newStop,
						Duration: entry.Duration,
					}

					item.Title = command + ": " + timeStr
					item.Subtitle = "Press enter to change start time (end time will also be adjusted)"
					item.Invalid = false
					item.Arg = "-update " + alfred.MustMakeDataArg(updateTimer)
				} else {
					dlog.Printf("Invalid time: %s\n", timeStr)
				}
			}

			items = append(items, item)
		}

		if alfred.PartiallyPrefixes("duration", query) {
			command := "Duration"
			parts := alfred.CleanSplitN(query, " ", 2)
			duration := float32(entry.Duration) / 60.0 / 60.0

			item := alfred.Item{
				Title:        fmt.Sprintf("%s: %.2f", command, duration),
				Invalid:      true,
				Autocomplete: command + ": ",
				Subtitle:     "Change the duration",
			}

			if strings.ToLower(parts[0]) == "duration:" {
				item.Subtitle = "Change duration (end time will be adjusted)"
			}

			if len(parts) > 1 {
				newDuration := parts[1]
				if val, err := strconv.ParseFloat(newDuration, 64); err == nil {
					updateTimer := toggl.TimeEntry{
						Id:       entry.Id,
						Duration: int64(val * 60 * 60),
					}
					item.Title = fmt.Sprintf("%s: %.2f", command, val)
					item.Subtitle = "Press enter to change duration (end time will be adjusted)"
					item.Arg = "-update " + alfred.MustMakeDataArg(updateTimer)
					item.Invalid = false
				}
			}

			items = append(items, item)
		}
	}

	if alfred.PartiallyPrefixes("delete", query) {
		items = append(items, alfred.Item{
			Title:        "Delete",
			Subtitle:     "Delete this time entry",
			Arg:          fmt.Sprintf("-delete=%d", entry.Id),
			Autocomplete: "Delete",
		})
	}

	items = alfred.SortItemsForKeyword(items, query)

	return
}
