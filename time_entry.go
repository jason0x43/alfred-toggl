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
)

// entry -------------------------------------------------

// TimeEntryCommand is a command
type TimeEntryCommand struct{}

// About returns information about a command
func (c TimeEntryCommand) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     "timers",
		Description: "List and modify recent time entries, add new ones",
		IsEnabled:   config.APIKey != "",
	}
}

// Items returns a list of filter items
func (c TimeEntryCommand) Items(arg, data string) (items []alfred.Item, err error) {
	if err = checkRefresh(); err != nil {
		return
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

	// Starting a new timer, still needs something
	if cfg.ToStart != nil {
		toStart := cfg.ToStart
		if toStart.Pid == 0 {
			for _, proj := range cache.Account.Data.Projects {
				if proj.IsActive() && alfred.FuzzyMatches(proj.Name, arg) {
					toStart.Pid = proj.ID
					item := alfred.Item{
						Title:        proj.Name,
						Autocomplete: proj.Name,
						Arg: &alfred.ItemArg{
							Keyword: "timers",
							Mode:    alfred.ModeDo,
							Data:    alfred.Stringify(timerCfg{ToStart: toStart}),
						},
					}
					item.AddCheckBox(false)
					items = append(items, item)
				}
			}
			return
		}
	}

	var entries []TimeEntry

	if pid == -1 {
		// If the user didn't specify a PID, use the default one
		if config.DefaultProjectID != 0 {
			pid = config.DefaultProjectID
		}
	}

	if tid != -1 {
		// Do someting with a specific time entry
		if entry, _, ok := getTimerByID(tid); ok {
			items, err = timeEntryItems(&entry, arg)
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

	var filtered []TimeEntry
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

			subtitle := "New entry"
			if pid != -1 {
				project, _, _ := getProjectByID(pid)
				subtitle += " in " + project.Name
			}

			item := alfred.Item{
				Title:    arg,
				Subtitle: subtitle,
				Arg: &alfred.ItemArg{
					Keyword: "timers",
					Mode:    alfred.ModeDo,
					Data:    alfred.Stringify(timerCfg{ToStart: &newTimer}),
				},
			}

			item.AddMod(alfred.ModAlt, alfred.ItemMod{
				Subtitle: "Choose project...",
				Arg: &alfred.ItemArg{
					Keyword: "timers",
					Data:    alfred.Stringify(timerCfg{ToStart: &newTimer}),
				},
			})

			items = append(items, item)
		}
	} else {
		sort.Sort(sort.Reverse(byTime(filtered)))

		for _, entry := range filtered {
			item := alfred.Item{
				Title:        entry.Description,
				Autocomplete: entry.Description,
				Arg: &alfred.ItemArg{
					Keyword: "timers",
					Data:    alfred.Stringify(timerCfg{Timer: &entry.ID}),
				},
			}

			var modTitle string
			if entry.IsRunning() {
				modTitle = "Stop this timer"
			} else {
				modTitle = "Start this timer"
			}

			item.AddMod(alfred.ModCmd, alfred.ItemMod{
				Subtitle: modTitle,
				Arg: &alfred.ItemArg{
					Keyword: "timers",
					Mode:    alfred.ModeDo,
					Data:    alfred.Stringify(timerCfg{ToToggle: &entry.ID}),
				},
			})

			var seconds int64

			startTime := entry.StartTime()
			if entry.Duration < 0 {
				seconds = int64(time.Now().Sub(startTime).Seconds())
			} else {
				seconds = entry.Duration
			}

			duration := float64(seconds) / 3600.0

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

	if pid != -1 && arg == "" {
		project, _, _ := getProjectByID(pid)
		items = alfred.InsertItem(items, alfred.Item{
			Title:    fmt.Sprintf("%s time entries", project.Name),
			Subtitle: alfred.Line,
			Arg: &alfred.ItemArg{
				Keyword: "projects",
				Data:    alfred.Stringify(&projectCfg{Project: &project.ID}),
			},
		}, 0)
	}

	return
}

// Do runs the command
func (c TimeEntryCommand) Do(data string) (out string, err error) {
	var cfg timerCfg

	if data != "" {
		if err := json.Unmarshal([]byte(data), &cfg); err != nil {
			dlog.Printf("Error unmarshalling data: %v", err)
		}
	}

	if cfg.ToUpdate != nil {
		dlog.Printf("updating time entry %v", cfg.ToUpdate)
		var timer TimeEntry
		if timer, err = updateTimeEntry(*cfg.ToUpdate); err != nil {
			return
		}
		return fmt.Sprintf(`Updated time entry "%s"`, timer.Description), nil
	}

	if cfg.ToStart != nil {
		dlog.Printf("starting new entry %v", cfg.ToStart)
		var timer TimeEntry
		if timer, err = startTimeEntry(*cfg.ToStart); err != nil {
			return
		}
		return fmt.Sprintf(`Started time entry "%s"`, timer.Description), nil
	}

	if cfg.ToToggle != nil {
		dlog.Printf("toggling entry %v", cfg.ToToggle)
		var timer TimeEntry
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
		var timer TimeEntry
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
	ToUpdate *TimeEntry `json:"toupdate,omitempty"`
	ToDelete *int       `json:"todelete,omitempty"`
	ToToggle *int       `json:"totoggle,omitempty"`
}

type startDesc struct {
	Description string `json:"description"`
	Pid         int    `json:"pid"`
}

func deleteTimeEntry(id int) (entry TimeEntry, err error) {
	var ok bool
	var index int
	if entry, index, ok = getTimerByID(id); !ok {
		err = fmt.Errorf(`Time entry %d does not exist`, id)
		return
	}

	session := OpenSession(config.APIKey)
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

func startTimeEntry(desc startDesc) (entry TimeEntry, err error) {
	session := OpenSession(config.APIKey)

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

func toggleTimeEntry(toToggle int) (updatedEntry TimeEntry, err error) {
	var entry TimeEntry
	var ok bool
	var index int
	if entry, index, ok = getTimerByID(toToggle); !ok {
		err = fmt.Errorf("Invalid timer ID %d", toToggle)
		return
	}

	running, isRunning := getRunningTimer()
	session := OpenSession(config.APIKey)

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

func updateTimeEntry(entryIn TimeEntry) (entry TimeEntry, err error) {
	session := OpenSession(config.APIKey)

	if entry, err = session.UpdateTimeEntry(entryIn); err != nil {
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

func timeEntryItems(entry *TimeEntry, query string) (items []alfred.Item, err error) {
	parts := alfred.CleanSplitN(query, " ", 2)

	if alfred.FuzzyMatches("description:", parts[0]) {
		var item alfred.Item

		if len(parts) > 1 {
			newDesc := parts[1]

			updateEntry := entry.Copy()
			updateEntry.Description = newDesc

			item.Title = "Description: " + newDesc
			item.Subtitle = "Description: " + entry.Description
			item.Arg = &alfred.ItemArg{
				Keyword: "timers",
				Mode:    alfred.ModeDo,
				Data:    alfred.Stringify(timerCfg{ToUpdate: &updateEntry}),
			}
		} else {
			item.Title = "Description: " + entry.Description
			item.Subtitle = "Update this entry's description"
			item.Autocomplete = "Description: " + entry.Description
		}

		items = append(items, item)
	}

	if alfred.FuzzyMatches("project:", parts[0]) {
		command := "Project"

		if strings.ToLower(parts[0]) == "project:" {
			var name string

			if len(parts) > 1 {
				name = parts[1]
			}

			for _, proj := range cache.Account.Data.Projects {
				if proj.IsActive() && alfred.FuzzyMatches(proj.Name, name) {
					updateEntry := entry.Copy()
					if entry.Pid == proj.ID {
						updateEntry.Pid = 0
					} else {
						updateEntry.Pid = proj.ID
					}
					item := alfred.Item{
						Title:        proj.Name,
						Autocomplete: command + ": " + proj.Name,
						Arg: &alfred.ItemArg{
							Keyword: "timers",
							Mode:    alfred.ModeDo,
							Data:    alfred.Stringify(timerCfg{ToUpdate: &updateEntry}),
						},
					}
					item.AddCheckBox(entry.Pid == proj.ID)
					items = append(items, item)
				}
			}
		} else {
			item := alfred.Item{
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

	if alfred.FuzzyMatches("tags:", parts[0]) {
		command := "Tags"

		if strings.ToLower(parts[0]) == "tags:" {
			var tagName string

			if len(parts) > 1 {
				tagName = parts[1]
			}

			for _, tag := range cache.Account.Data.Tags {
				if alfred.FuzzyMatches(tag.Name, tagName) {
					item := alfred.Item{
						Title:        tag.Name,
						Autocomplete: tag.Name,
					}
					item.AddCheckBox(entry.HasTag(tag.Name))

					updateEntry := entry.Copy()
					if entry.HasTag(tag.Name) {
						updateEntry.RemoveTag(tag.Name)
					} else {
						updateEntry.AddTag(tag.Name)
					}

					item.Arg = &alfred.ItemArg{
						Keyword: "timers",
						Mode:    alfred.ModeDo,
						Data:    alfred.Stringify(timerCfg{ToUpdate: &updateEntry}),
					}

					items = append(items, item)
				}
			}
		} else {
			item := alfred.Item{
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

	if alfred.FuzzyMatches("start:", parts[0]) {
		command := "Start"

		var startTime string
		if !entry.StartTime().IsZero() {
			startTime = entry.StartTime().Local().Format("15:04")
		}

		item := alfred.Item{
			Title:        command + ": " + startTime,
			Autocomplete: command + ": ",
			Subtitle:     "Change the start time",
		}

		if len(parts) > 1 {
			timeStr := parts[1]

			if newTime, err := time.Parse("15:04", timeStr); err == nil {
				newStart := getNewTime(entry.StartTime().Local(), newTime)

				updateTimer := entry.Copy()
				updateTimer.Start = &newStart

				if !entry.IsRunning() {
					updateTimer.Duration = entry.Duration
				}

				item.Title = command + ": " + timeStr
				item.Subtitle = "Press enter to change start time (end time will also be adjusted)"
				item.Arg = &alfred.ItemArg{
					Keyword: "timers",
					Mode:    alfred.ModeDo,
					Data:    alfred.Stringify(timerCfg{ToUpdate: &updateTimer}),
				}
			} else {
				dlog.Printf("Invalid time: %s\n", timeStr)
			}
		}

		items = append(items, item)
	}

	if !entry.IsRunning() {
		if alfred.FuzzyMatches("stop:", parts[0]) {
			command := "Stop"
			parts := alfred.CleanSplitN(query, " ", 2)

			var stopTime string
			if !entry.StopTime().IsZero() {
				stopTime = entry.StopTime().Local().Format("15:04")
			}

			item := alfred.Item{
				Title:        command + ": " + stopTime,
				Autocomplete: command + ": ",
				Subtitle:     "Change the stop time",
			}

			if len(parts) > 1 {
				timeStr := parts[1]

				if newTime, err := time.Parse("15:04", timeStr); err == nil {
					newStop := getNewTime(entry.StopTime().Local(), newTime)

					updateTimer := entry.Copy()
					updateTimer.Stop = &newStop

					item.Title = command + ": " + timeStr
					item.Subtitle = "Press enter to change start time (end time will also be adjusted)"
					item.Arg = &alfred.ItemArg{
						Keyword: "timers",
						Mode:    alfred.ModeDo,
						Data:    alfred.Stringify(timerCfg{ToUpdate: &updateTimer}),
					}
				} else {
					dlog.Printf("Invalid time: %s\n", timeStr)
				}
			}

			items = append(items, item)
		}

		if alfred.FuzzyMatches("duration:", parts[0]) {
			command := "Duration"
			parts := alfred.CleanSplitN(query, " ", 2)
			duration := float32(entry.Duration) / 60.0 / 60.0

			item := alfred.Item{
				Title:        fmt.Sprintf("%s: %.2f", command, duration),
				Autocomplete: command + ": ",
				Subtitle:     "Change the duration",
			}

			// Add an option to round the duration down to a time increment
			roundedDuration := float32(roundDuration(entry.Duration, true)) / 100

			updateTimer := entry.Copy()
			updateTimer.Duration = int64(roundedDuration * 60 * 60)

			item.AddMod(alfred.ModAlt, alfred.ItemMod{
				Subtitle: fmt.Sprintf("Round down to %.2f", roundedDuration),
				Arg: &alfred.ItemArg{
					Keyword: "timers",
					Mode:    alfred.ModeDo,
					Data:    alfred.Stringify(timerCfg{ToUpdate: &updateTimer}),
				},
			})

			if strings.ToLower(parts[0]) == "duration:" {
				item.Subtitle = "Change duration (end time will be adjusted)"
			}

			if len(parts) > 1 {
				newDuration := parts[1]
				if val, err := strconv.ParseFloat(newDuration, 64); err == nil {
					updateTimer.Duration = int64(val * 60 * 60)
					item.Title = fmt.Sprintf("%s: %.2f", command, val)
					item.Subtitle = "Press enter to change duration (end time will be adjusted)"
					item.Arg = &alfred.ItemArg{
						Keyword: "timers",
						Mode:    alfred.ModeDo,
						Data:    alfred.Stringify(timerCfg{ToUpdate: &updateTimer}),
					}
				}
			}

			items = append(items, item)
		}

		if alfred.FuzzyMatches("continue", query) {
			items = append(items, alfred.Item{
				Title:    "Continue",
				Subtitle: "Continue this time entry",
				Arg: &alfred.ItemArg{
					Keyword: "timers",
					Mode:    alfred.ModeDo,
					Data:    alfred.Stringify(timerCfg{ToToggle: &entry.ID}),
				},
				Autocomplete: "Start",
			})
		}
	} else {
		if alfred.FuzzyMatches("stop", query) {
			items = append(items, alfred.Item{
				Title:    "Stop",
				Subtitle: "Stop this time entry",
				Arg: &alfred.ItemArg{
					Keyword: "timers",
					Mode:    alfred.ModeDo,
					Data:    alfred.Stringify(timerCfg{ToToggle: &entry.ID}),
				},
				Autocomplete: "Stop",
			})
		}
	}

	if alfred.FuzzyMatches("delete", query) {
		items = append(items, alfred.Item{
			Title:    "Delete",
			Subtitle: "Delete this time entry",
			Arg: &alfred.ItemArg{
				Keyword: "timers",
				Mode:    alfred.ModeDo,
				Data:    alfred.Stringify(timerCfg{ToDelete: &entry.ID}),
			},
			Autocomplete: "Delete",
		})
	}

	return
}
