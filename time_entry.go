package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-toggl"
)

// entry -------------------------------------------------

type TimeEntryFilter struct{}

func (c TimeEntryFilter) Keyword() string {
	return "timers"
}

func (c TimeEntryFilter) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c TimeEntryFilter) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword() + " ",
		Valid:        alfred.Invalid,
		SubtitleAll:  "List and modify recent time entries, add new ones",
	}
}

func (c TimeEntryFilter) Items(prefix, query string) (items []alfred.Item, err error) {
	if err = checkRefresh(); err != nil {
		return items, err
	}

	projects := getProjectsById()
	parts := alfred.TrimAllLeft(strings.Split(query, alfred.Separator))
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		id = 0
	}
	entry, ok := findTimerById(id)

	if ok {
		var property string
		if len(parts) > 1 {
			property = strings.ToLower(parts[1])
		}

		addItem := func(title, subtitle, keyword string, hasNext, showSubtitle bool) {
			item := alfred.Item{
				Title:        title,
				Autocomplete: prefix + parts[0] + alfred.Separator + " " + keyword,
			}

			if showSubtitle {
				item.Subtitle = subtitle
			}

			if hasNext {
				item.Autocomplete += alfred.Separator + " "
			}

			if len(parts) > 2 {
				item.Arg = parts[2]
			} else {
				item.Valid = alfred.Invalid
			}

			items = append(items, item)
		}

		if alfred.FuzzyMatches("description", property) {
			subcommand := "description"

			if len(parts) > 2 && parts[2] != "" {
				updateEntry := entry.Copy()
				updateEntry.Description = parts[2]
				dataString, _ := json.Marshal(updateEntry)

				item := alfred.Item{
					Title:    subcommand + ": " + parts[2],
					Subtitle: "Change description",
					Arg:      "update-entry " + string(dataString),
				}

				items = append(items, item)
			} else {
				addItem(subcommand+": "+entry.Description, "Change description", subcommand, true, property == "description")
			}
		}

		if alfred.FuzzyMatches("project", property) {
			subcommand := "project"

			if len(parts) > 2 {
				var matches func(string, string) bool
				name := parts[2]
				complete := prefix + parts[0] + alfred.Separator + " " + subcommand + alfred.Separator + " "
				terminator := strings.Index(parts[2], alfred.Terminator)

				if terminator != -1 {
					name = name[:terminator]
					matches = func(query, test string) bool {
						return query == test
					}
				} else {
					matches = alfred.FuzzyMatches
				}

				for _, proj := range projects {
					if matches(proj.Name, name) {
						updateEntry := entry.Copy()
						if entry.Pid == proj.Id {
							updateEntry.Pid = 0
						} else {
							updateEntry.Pid = proj.Id
						}
						dataString, _ := json.Marshal(updateEntry)

						items = append(items, alfred.MakeChoice(alfred.Item{
							Title:        proj.Name,
							Autocomplete: complete + proj.Name + alfred.Terminator,
							Arg:          "update-entry " + string(dataString),
						}, entry.Pid == proj.Id))
					}
				}
			} else {
				title := subcommand + ": "
				if project, ok := projects[entry.Pid]; ok {
					title += project.Name
				} else {
					title += "<None>"
				}
				addItem(title, "Change project", subcommand, true, property == "project")
			}
		}

		if alfred.FuzzyMatches("tags", property) {
			subcommand := "tags"

			if len(parts) > 2 {
				// We have the "tags" subcommand followed by a query
				for _, tag := range cache.Account.Data.Tags {
					if alfred.FuzzyMatches(tag.Name, parts[2]) {
						item := alfred.MakeChoice(alfred.Item{
							Title:        tag.Name,
							Autocomplete: prefix + tag.Name,
						}, entry.HasTag(tag.Name))

						updateEntry := entry.Copy()
						if entry.HasTag(tag.Name) {
							updateEntry.RemoveTag(tag.Name)
						} else {
							updateEntry.AddTag(tag.Name)
						}
						dataString, _ := json.Marshal(updateEntry)

						item.Arg = "update-entry " + string(dataString)
						items = append(items, item)
					}
				}
			} else {
				// We have a fuzzy match for the "tags" subcommand
				title := subcommand + ": "
				if len(entry.Tags) > 0 {
					title += strings.Join(entry.Tags, ", ")
				} else {
					title += "<None>"
				}
				addItem(title, "Change tags", subcommand, true, property == "tags")
			}
		}

		if alfred.FuzzyMatches("start", property) {
			subcommand := "start"

			var startTime string
			if !entry.StartTime().IsZero() {
				startTime = entry.StartTime().Local().Format("15:04")
			}

			item := alfred.Item{
				Title:        subcommand + ": " + startTime,
				Valid:        alfred.Invalid,
				Autocomplete: prefix + parts[0] + alfred.Separator + " " + subcommand + alfred.Separator + " ",
			}

			if property == subcommand {
				item.Subtitle = "Change start time"
			}

			if len(parts) > 2 && parts[2] != "" {
				newTime, err := time.Parse("15:04", parts[2])
				if err == nil {
					originalStart := entry.StartTime().Local()
					originalMinutes := originalStart.Hour()*60 + originalStart.Minute()
					newMinutes := newTime.Hour()*60 + newTime.Minute()

					delta, _ := time.ParseDuration(fmt.Sprintf("%dm", newMinutes-originalMinutes))
					newStart := originalStart.Add(delta)

					updateTimer := toggl.TimeEntry{
						Id:    entry.Id,
						Start: &newStart,
					}

					if !entry.IsRunning() {
						updateTimer.Duration = entry.Duration
					}

					dataString, _ := json.Marshal(updateTimer)
					log.Printf("marshaled entry to: %s\n", dataString)

					item.Title = subcommand + ": " + parts[2]
					item.Subtitle = "Press enter to change start time (end time will also be adjusted)"
					item.Arg = "update-entry " + string(dataString)
					item.Valid = ""
				} else {
					log.Printf("Invalid time: %s\n", parts[2])
				}
			}

			items = append(items, item)
		}

		if !entry.IsRunning() {
			if alfred.FuzzyMatches("stop", property) {
				subcommand := "stop"
				stopTime := entry.StopTime()
				addItem(subcommand+": "+stopTime.Local().Format("15:04"), "", subcommand, false, false)
			}

			if alfred.FuzzyMatches("duration", property) {
				subcommand := "duration"
				duration := float32(entry.Duration) / 60.0 / 60.0

				item := alfred.Item{
					Title:        fmt.Sprintf("%s: %.2f", subcommand, duration),
					Valid:        alfred.Invalid,
					Autocomplete: prefix + parts[0] + alfred.Separator + " " + subcommand + alfred.Separator + " ",
				}

				if property == subcommand {
					item.Subtitle = "Change duration (end time will be adjusted)"
				}

				if len(parts) > 2 && parts[2] != "" {
					val, err := strconv.ParseFloat(parts[2], 64)
					if err == nil {
						updateTimer := toggl.TimeEntry{
							Id:       entry.Id,
							Duration: int64(val * 60 * 60),
						}

						dataString, _ := json.Marshal(updateTimer)
						log.Printf("marshaled entry to: %s\n", dataString)

						item.Title = fmt.Sprintf("%s: %.2f", subcommand, val)
						item.Subtitle = "Press enter to change duration (end time will be adjusted)"
						item.Arg = "update-entry " + string(dataString)
						item.Valid = ""
					}
				}

				items = append(items, item)
			}
		}

		if alfred.FuzzyMatches("delete", property) {
			subcommand := "delete"
			data := deleteMessage{Type: "time entry", Id: entry.Id}
			dataString, _ := json.Marshal(data)

			item := alfred.Item{
				Title:        subcommand,
				Arg:          "delete " + string(dataString),
				Autocomplete: prefix + parts[0] + alfred.Separator + " " + subcommand,
			}

			if property == subcommand {
				item.Subtitle = "Delete this time entry"
			}

			items = append(items, item)
		}
	} else {
		entries := getTimeEntriesForQuery(parts[0])

		if len(entries) == 0 {
			data := startMessage{Description: parts[0]}
			dataString, _ := json.Marshal(data)

			items = append(items, alfred.Item{
				Title:       parts[0],
				SubtitleAll: "New entry",
				Arg:         "start " + string(dataString),
			})
		} else {
			for _, entry := range entries {
				var seconds int64

				startTime := entry.StartTime()
				if entry.Duration < 0 {
					seconds = int64(time.Now().Sub(startTime).Seconds())
				} else {
					seconds = entry.Duration
				}

				duration := float32(roundDuration(seconds)) / 100.0
				subtitle := fmt.Sprintf("%.2f, %s from ", duration, toHumanDateString(startTime))
				subtitle += startTime.Local().Format("3:04pm") + " to "
				if entry.Duration < 0 {
					subtitle += "now"
				} else if !entry.StopTime().IsZero() {
					subtitle += entry.StopTime().Local().Format("3:04pm")
				} else {
					log.Printf("No duration or stop time")
				}

				if project, ok := projects[entry.Pid]; ok {
					subtitle = "[" + project.Name + "] " + subtitle
				}

				item := alfred.Item{
					Title:        entry.Description,
					SubtitleAll:  subtitle,
					Arg:          fmt.Sprintf("toggle %v", entry.Id),
					Autocomplete: prefix + fmt.Sprintf("%d%s ", entry.Id, alfred.Separator),
				}

				if entry.IsRunning() {
					item.Icon = "running.png"
				}

				items = append(items, item)
			}
		}
	}

	return items, nil
}

// update-entry ------------------------------------------

type UpdateTimeEntryAction struct{}

func (c UpdateTimeEntryAction) Keyword() string {
	return "update-entry"
}

func (c UpdateTimeEntryAction) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c UpdateTimeEntryAction) Do(query string) (string, error) {
	log.Printf("update-entry %s", query)

	var entry toggl.TimeEntry
	err := json.Unmarshal([]byte(query), &entry)
	if err != nil {
		return "", fmt.Errorf("Invalid time entry %v", query)
	}

	session := toggl.OpenSession(config.ApiKey)

	entry, err = session.UpdateTimeEntry(entry)
	if err != nil {
		return "", err
	}

	adata := &cache.Account.Data

	for i, e := range adata.TimeEntries {
		if e.Id == entry.Id {
			adata.TimeEntries[i] = entry
			err = alfred.SaveJson(cacheFile, &cache)
			if err != nil {
				log.Printf("Error saving cache: %v\n", err)
			}
			break
		}
	}

	return fmt.Sprintf("Updated '%s'", entry.Description), nil
}
