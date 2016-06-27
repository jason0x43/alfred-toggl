package main

import (
	"encoding/json"
	"fmt"
	"log"
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

// About returns information about a command
func (c TimeEntryCommand) About() *alfred.CommandDef {
	return &alfred.CommandDef{
		Keyword:     "timers",
		Description: "List and modify recent time entries, add new ones",
		WithSpace:   true,
	}
}

// IsEnabled returns true if the command is enabled
func (c TimeEntryCommand) IsEnabled() bool {
	return config.APIKey != ""
}

// Items returns a list of filter items
func (c TimeEntryCommand) Items(arg, data string) (items []*alfred.Item, err error) {
	if err = checkRefresh(); err != nil {
		return items, err
	}

	pid := -1
	tid := -1

	var cfg timerCfg
	if data != "" {
		if err := json.Unmarshal([]byte(data), &cfg); err != nil {
			dlog.Printf("Invalid timer config")
		}
	}

	if cfg.Project != nil {
		pid = *cfg.Project
	}

	if cfg.Timer != nil {
		tid = *cfg.Timer
	}

	var entries []toggl.TimeEntry

	if pid == -1 {
		// If the user didn't specify a PID, use the default one
		if config.DefaultProjectID != 0 {
			pid = config.DefaultProjectID
		}
	}

	if tid != -1 {
		// Do someting with a specific time entry
		if entry, _, ok := getTimerByID(tid); ok {
			items, err = timeEntryItems(entry, arg)
			return
		}
	} else if pid != -1 {
		// Filter time entries by project ID
		entries = findTimersByProjectID(pid)
		dlog.Printf("found %d timers for project %d", len(entries), pid)
	} else {
		// Use all time entries
		entries = cache.Account.Data.TimeEntries
		dlog.Printf("showing all %d timers", len(entries))
	}

	var filtered []toggl.TimeEntry
	for _, entry := range entries {
		if alfred.FuzzyMatches(entry.Description, arg) {
			filtered = append(filtered, entry)
		}
	}

	if len(filtered) == 0 {
		// No entries matched, so we must be creating a new one
		if arg != "" {
			// Arg is the new project's description

			newTimer := startDesc{Description: arg}
			if pid != -1 {
				newTimer.Pid = pid
			}

			items = append(items, &alfred.Item{
				Title:    arg,
				Subtitle: "New entry",
				Arg: &alfred.ItemArg{
					Keyword: "timers",
					Mode:    alfred.ModeDo,
					Data:    alfred.Stringify(timerCfg{ToStart: &newTimer}),
				},
			})
		}
	} else {
		sort.Sort(sort.Reverse(byTime(filtered)))

		for _, entry := range filtered {
			item := &alfred.Item{
				Title:        entry.Description,
				Autocomplete: entry.Description,
				Arg: &alfred.ItemArg{
					Keyword: "timers",
					Data:    alfred.Stringify(timerCfg{Timer: &entry.ID}),
				},
			}

			item.AddMod("cmd", "Toggle this timer's running status", &alfred.ItemArg{
				Keyword: "timers",
				Mode:    alfred.ModeDo,
				Data:    alfred.Stringify(timerCfg{ToToggle: &entry.ID}),
			})

			var seconds int64

			startTime := entry.StartTime()
			if entry.Duration < 0 {
				seconds = int64(time.Now().Sub(startTime).Seconds())
			} else {
				seconds = entry.Duration
			}

			duration := float32(roundDuration(seconds)) / 100.0

			item.Subtitle = fmt.Sprintf("%.2f, %s from %s to ", duration,
				toHumanDateString(startTime), startTime.Local().Format("3:04pm"))

			if entry.Duration < 0 {
				item.Subtitle += "now"
			} else if !entry.StopTime().IsZero() {
				item.Subtitle += entry.StopTime().Local().Format("3:04pm")
			} else {
				dlog.Printf("No duration or stop time")
			}

			if project, _, ok := getProjectByID(entry.Pid); ok {
				item.Subtitle = "[" + project.Name + "] " + item.Subtitle
			}

			if entry.IsRunning() {
				item.Icon = "running.png"
			}

			items = append(items, item)
		}
	}

	return
}

// Do runs the command
func (c TimeEntryCommand) Do(arg, data string) (out string, err error) {
	var cfg timerCfg

	if data != "" {
		if err := json.Unmarshal([]byte(data), &cfg); err != nil {
			dlog.Printf("Error unmarshalling data: %v", err)
		}
	}

	if cfg.ToUpdate != nil {
		dlog.Printf("updating time entry %v", cfg.ToUpdate)
		var timer toggl.TimeEntry
		if timer, err = updateTimeEntry(*cfg.ToUpdate); err != nil {
			return
		}
		return fmt.Sprintf(`Updated time entry "%s"`, timer.Description), nil
	}

	if cfg.ToStart != nil {
		dlog.Printf("starting new entry %v", cfg.ToStart)
		var timer toggl.TimeEntry
		if timer, err = startTimeEntry(*cfg.ToStart); err != nil {
			return
		}
		return fmt.Sprintf(`Started time entry "%s"`, timer.Description), nil
	}

	if cfg.ToToggle != nil {
		dlog.Printf("toggling entry %v", cfg.ToToggle)
		var timer toggl.TimeEntry
		if timer, err = toggleTimeEntry(*cfg.ToToggle); err != nil {
			return
		}
		if timer.IsRunning() {
			return fmt.Sprintf(`Started time entry "%s"`, timer.Description), nil
		}
		return fmt.Sprintf(`Stopped time entry "%s"`, timer.Description), nil
	}

	if cfg.ToDelete != nil {
		dlog.Printf("deleting entry %v", cfg.ToDelete)
		var timer toggl.TimeEntry
		if timer, err = deleteTimeEntry(*cfg.ToDelete); err != nil {
			return
		}
		return fmt.Sprintf(`Deleted time entry "%s"`, timer.Description), nil
	}

	return "Unrecognized input", nil
}

// support -------------------------------------------------------------------

type timerCfg struct {
	Timer    *int       `json:"timer,omitempty"`
	Property *string    `json:"property,omitempty"`
	Project  *int       `json:"project,omitempty"`
	ToStart  *startDesc `json:"tostart,omitempty"`
	ToUpdate *string    `json:"toupdate,omitempty"`
	ToDelete *int       `json:"todelete,omitempty"`
	ToToggle *int       `json:"totoggle,omitempty"`
}

type startDesc struct {
	Description string `json:"description"`
	Pid         int    `json:"pid"`
}

func deleteTimeEntry(id int) (entry toggl.TimeEntry, err error) {
	var ok bool
	var index int
	if entry, index, ok = getTimerByID(id); !ok {
		err = fmt.Errorf(`Time entry %d does not exist`, id)
		return
	}

	session := toggl.OpenSession(config.APIKey)
	if _, err = session.DeleteTimeEntry(entry); err == nil {
		adata := &cache.Account.Data
		if index < len(adata.TimeEntries)-1 {
			adata.TimeEntries = append(adata.TimeEntries[:index], adata.TimeEntries[index+1:]...)
		} else {
			adata.TimeEntries = adata.TimeEntries[:index]
		}
		if err := alfred.SaveJSON(cacheFile, &cache); err != nil {
			dlog.Printf("Error saving cache: %s\n", err)
		}
	}

	return
}

func startTimeEntry(desc startDesc) (entry toggl.TimeEntry, err error) {
	session := toggl.OpenSession(config.APIKey)

	if desc.Pid != 0 {
		entry, err = session.StartTimeEntryForProject(desc.Description, desc.Pid)
	} else {
		entry, err = session.StartTimeEntry(desc.Description)
	}

	if err == nil {
		dlog.Printf("Got entry: %#v\n", entry)
		cache.Account.Data.TimeEntries = append(cache.Account.Data.TimeEntries, entry)
		if err := alfred.SaveJSON(cacheFile, &cache); err != nil {
			dlog.Printf("Error saving cache: %s\n", err)
		}
	}

	return entry, nil
}

func toggleTimeEntry(toToggle int) (updatedEntry toggl.TimeEntry, err error) {
	var entry toggl.TimeEntry
	var ok bool
	var index int
	if entry, index, ok = getTimerByID(toToggle); !ok {
		err = fmt.Errorf("Invalid timer ID %d", toToggle)
		return
	}

	running, isRunning := getRunningTimer()
	session := toggl.OpenSession(config.APIKey)

	if entry.IsRunning() {
		if updatedEntry, err = session.StopTimeEntry(entry); err != nil {
			return
		}
	} else {
		if updatedEntry, err = session.ContinueTimeEntry(entry, config.DurationOnly); err != nil {
			return
		}
	}

	adata := &cache.Account.Data

	if updatedEntry.ID == entry.ID {
		adata.TimeEntries[index] = updatedEntry
	} else {
		adata.TimeEntries = append(adata.TimeEntries, updatedEntry)
	}

	if isRunning && running.ID != updatedEntry.ID {
		// If a different timer was previously running, refresh everything
		if err = refresh(); err != nil {
			log.Printf("Error refreshing: %v\n", err)
			return
		}
	} else {
		if err = alfred.SaveJSON(cacheFile, &cache); err != nil {
			log.Printf("Error saving cache: %v\n", err)
			return
		}
	}

	return
}

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
		if e.ID == entry.ID {
			adata.TimeEntries[i] = entry
			if err := alfred.SaveJSON(cacheFile, &cache); err != nil {
				dlog.Printf("Error saving cache: %v\n", err)
			}
			break
		}
	}

	return
}

func getNewTime(original, new time.Time) time.Time {
	originalMinutes := original.Hour()*60 + original.Minute()
	newMinutes := new.Hour()*60 + new.Minute()
	delta, _ := time.ParseDuration(fmt.Sprintf("%dm", newMinutes-originalMinutes))
	return original.Add(delta)
}

func timeEntryItems(entry toggl.TimeEntry, query string) (items []*alfred.Item, err error) {
	if alfred.FuzzyMatches("description:", query) {
		var item alfred.Item
		parts := alfred.CleanSplitN(query, " ", 2)

		if len(parts) > 1 {
			newDesc := parts[1]
			updateEntry := entry
			updateEntry.Description = newDesc
			item.Title = "Description: " + newDesc
			item.Subtitle = "Description: " + entry.Description
			// item.Arg = "-update " + alfred.Stringify(updateEntry)
		} else {
			item.Title = "Description: " + entry.Description
			item.Subtitle = "Update this entry's description"
			item.Autocomplete = "Description: "
		}

		items = append(items, &item)
	}

	if alfred.FuzzyMatches("project:", query) {
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
					if entry.Pid == proj.ID {
						updateEntry.Pid = 0
					} else {
						updateEntry.Pid = proj.ID
					}
					item := &alfred.Item{
						Title:        proj.Name,
						Autocomplete: command + ": " + proj.Name,
						// Arg:          "-update " + alfred.Stringify(updateEntry),
					}
					item.MakeChoice(entry.Pid == proj.ID)
					items = append(items, item)
				}
			}
		} else {
			item := &alfred.Item{
				Title:        command + ": ",
				Subtitle:     "Change the project this entry is assigned to",
				Autocomplete: command + ": ",
			}
			if project, _, ok := getProjectByID(entry.Pid); ok {
				item.Title += project.Name
			} else {
				item.Title += "<None>"
			}
			items = append(items, item)
		}
	}

	if alfred.FuzzyMatches("tags:", query) {
		command := "Tags"
		parts := alfred.CleanSplitN(query, " ", 2)

		if strings.ToLower(parts[0]) == "tags:" {
			var tagName string

			if len(parts) > 1 {
				tagName = parts[1]
			}

			for _, tag := range cache.Account.Data.Tags {
				if alfred.FuzzyMatches(tag.Name, tagName) {
					item := &alfred.Item{
						Title:        tag.Name,
						Autocomplete: tag.Name,
					}
					item.MakeChoice(entry.HasTag(tag.Name))

					updateEntry := entry.Copy()
					if entry.HasTag(tag.Name) {
						updateEntry.RemoveTag(tag.Name)
					} else {
						updateEntry.AddTag(tag.Name)
					}
					// item.Arg = "-update " + alfred.Stringify(updateEntry)

					items = append(items, item)
				}
			}
		} else {
			item := &alfred.Item{
				Title:        command + ": ",
				Subtitle:     "Update tags",
				Autocomplete: command + ": ",
			}
			if len(entry.Tags) > 0 {
				item.Title += strings.Join(entry.Tags, ", ")
			} else {
				item.Title += "<None>"
			}
			items = append(items, item)
		}
	}

	if alfred.FuzzyMatches("start:", query) {
		command := "Start"
		parts := alfred.CleanSplitN(query, " ", 2)

		var startTime string
		if !entry.StartTime().IsZero() {
			startTime = entry.StartTime().Local().Format("15:04")
		}

		item := &alfred.Item{
			Title:        command + ": " + startTime,
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
					ID:    entry.ID,
					Start: &newStart,
				}

				if !entry.IsRunning() {
					updateTimer.Duration = entry.Duration
				}

				item.Title = command + ": " + timeStr
				item.Subtitle = "Press enter to change start time (end time will also be adjusted)"
				// item.Arg = "-update " + alfred.Stringify(updateTimer)
			} else {
				dlog.Printf("Invalid time: %s\n", timeStr)
			}
		}

		items = append(items, item)
	}

	if !entry.IsRunning() {
		if alfred.FuzzyMatches("stop:", query) {
			command := "Stop"
			parts := alfred.CleanSplitN(query, " ", 2)

			var stopTime string
			if !entry.StopTime().IsZero() {
				stopTime = entry.StopTime().Local().Format("15:04")
			}

			item := &alfred.Item{
				Title:        command + ": " + stopTime,
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
						ID:       entry.ID,
						Stop:     &newStop,
						Duration: entry.Duration,
					}

					item.Title = command + ": " + timeStr
					item.Subtitle = "Press enter to change start time (end time will also be adjusted)"
					item.Arg = &alfred.ItemArg{
						Keyword: "timers",
						Data:    alfred.Stringify(updateTimer),
					}
				} else {
					dlog.Printf("Invalid time: %s\n", timeStr)
				}
			}

			items = append(items, item)
		}

		if alfred.FuzzyMatches("duration", query) {
			command := "Duration"
			parts := alfred.CleanSplitN(query, " ", 2)
			duration := float32(entry.Duration) / 60.0 / 60.0

			item := &alfred.Item{
				Title:        fmt.Sprintf("%s: %.2f", command, duration),
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
						ID:       entry.ID,
						Duration: int64(val * 60 * 60),
					}
					item.Title = fmt.Sprintf("%s: %.2f", command, val)
					item.Subtitle = "Press enter to change duration (end time will be adjusted)"
					item.Arg = &alfred.ItemArg{
						Keyword: "timers",
						Mode:    alfred.ModeDo,
						// TODO: should be config
						Data: alfred.Stringify(updateTimer),
					}
				}
			}

			items = append(items, item)
		}
	}

	if alfred.FuzzyMatches("delete", query) {
		items = append(items, &alfred.Item{
			Title:    "Delete",
			Subtitle: "Delete this time entry",
			// Arg:          fmt.Sprintf("-delete=%d", entry.ID),
			Autocomplete: "Delete",
		})
	}

	return
}
