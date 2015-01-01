package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-toggl"
)

var cacheFile string
var configFile string
var config Config
var cache Cache
var workflow *alfred.Workflow

type Config struct {
	ApiKey string `json:"api_key"`
}

type Cache struct {
	Workspace int
	Account   toggl.Account
	Time      time.Time
}

func main() {
	var err error

	workflow, err = alfred.OpenWorkflow(".", true)
	if err != nil {
		fmt.Printf("Error: %s", err)
		os.Exit(1)
	}

	toggl.AppName = "alfred-toggl"

	configFile = path.Join(workflow.DataDir(), "config.json")
	cacheFile = path.Join(workflow.CacheDir(), "cache.json")

	log.Println("Using config file", configFile)
	log.Println("Using cache file", cacheFile)

	err = alfred.LoadJson(configFile, &config)
	if err != nil {
		log.Println("Error loading config:", err)
	}
	log.Println("loaded config:", config)

	err = alfred.LoadJson(cacheFile, &cache)
	log.Println("loaded cache")

	commands := []alfred.Command{
		LoginCommand{},
		TimeEntryFilter{},
		ProjectFilter{},
		TagFilter{},
		ReportFilter{},
		SyncFilter{},
		LogoutCommand{},
		ResetCommand{},
		StartAction{},
		UpdateTimeEntryAction{},
		UpdateProjectAction{},
		CreateProjectAction{},
		UpdateTagAction{},
		CreateTagAction{},
		ToggleAction{},
		DeleteAction{},
		TagAction{},
	}

	workflow.Run(commands)
}

// login -------------------------------------------------

type LoginCommand struct{}

func (c LoginCommand) Keyword() string {
	return "login"
}

func (c LoginCommand) IsEnabled() bool {
	return config.ApiKey == ""
}

func (c LoginCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword(),
		Arg:          "login",
		SubtitleAll:  "Login to toggl.com",
	}
}

func (c LoginCommand) Items(prefix, query string) ([]alfred.Item, error) {
	item := c.MenuItem()
	item.Arg = "login"
	return []alfred.Item{item}, nil
}

func (c LoginCommand) Do(query string) (string, error) {
	btn, username, err := workflow.GetInput("Email address", "", false)
	if err != nil {
		return "", err
	}

	if btn != "Ok" {
		log.Println("User didn't click OK")
		return "", nil
	}
	log.Printf("username: %s", username)

	btn, password, err := workflow.GetInput("Password", "", true)
	if btn != "Ok" {
		log.Println("User didn't click OK")
		return "", nil
	}
	log.Printf("password: *****")

	session, err := toggl.NewSession(username, password)
	if err != nil {
		return "", err
	}

	config.ApiKey = session.ApiToken
	err = alfred.SaveJson(configFile, &config)
	if err != nil {
		return "", err
	}

	workflow.ShowMessage("Login successful!")
	return "", nil
}

// logout ------------------------------------------------

type LogoutCommand struct{}

func (c LogoutCommand) Keyword() string {
	return "logout"
}

func (c LogoutCommand) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c LogoutCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword(),
		SubtitleAll:  "Logout of Toggl, but keep any local config",
	}
}

func (c LogoutCommand) Items(prefix, query string) ([]alfred.Item, error) {
	item := c.MenuItem()
	item.Arg = "logout"
	return []alfred.Item{item}, nil
}

func (c LogoutCommand) Do(query string) (string, error) {
	config.ApiKey = ""
	err := alfred.SaveJson(configFile, &config)
	if err != nil {
		return "", err
	}

	workflow.ShowMessage("You are now logged out of Toggl")
	return "", nil
}

// reset -------------------------------------------------

type ResetCommand struct{}

func (c ResetCommand) Keyword() string {
	return "reset"
}

func (c ResetCommand) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c ResetCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword(),
		SubtitleAll:  "Reset this workflow, erasing all local data",
	}
}

func (c ResetCommand) Items(prefix, query string) ([]alfred.Item, error) {
	item := c.MenuItem()
	item.Arg = "reset"
	return []alfred.Item{item}, nil
}

func (c ResetCommand) Do(query string) (string, error) {
	err1 := os.Remove(configFile)
	err2 := os.Remove(cacheFile)

	if err1 != nil || err2 != nil {
		workflow.ShowMessage("One or more data files could not be removed")
	} else {
		workflow.ShowMessage("Workflow data cleared")
	}

	return "", nil
}

// time entries ------------------------------------------

type TimeEntryFilter struct{}

func (c TimeEntryFilter) Keyword() string {
	return "entries"
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
	parts := alfred.SplitAndTrimQuery(query)
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
				updateEntry := entry
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
						items = append(items, alfred.Item{
							Title:        proj.Name,
							Autocomplete: complete + proj.Name + alfred.Terminator,
						})
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
						title := tag.Name
						icon := ""
						data := tagMessage{Tag: tag.Name, TimeEntry: entry.Id, Add: true}

						if entry.HasTag(tag.Name) {
							data.Add = false
							icon = "running.png"
						}

						dataString, _ := json.Marshal(data)
						items = append(items, alfred.Item{
							Title: title,
							Arg:   "tag " + string(dataString),
							Icon:  icon,
						})
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
			if !entry.Start.IsZero() {
				startTime = entry.Start.Local().Format("15:04")
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
					originalStart := entry.Start.Local()
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
				stopTime := entry.Stop
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
							Duration: int(val * 60 * 60),
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
				var seconds int

				startTime := entry.StartTime()
				if entry.Duration < 0 {
					seconds = int(time.Now().Sub(startTime).Seconds())
				} else {
					seconds = entry.Duration
				}

				duration := float32(roundDuration(seconds)) / 100.0
				subtitle := fmt.Sprintf("%.2f, %s from ", duration, toHumanDateString(startTime))
				subtitle += startTime.Local().Format("3:04pm") + " to "
				if entry.Duration < 0 {
					subtitle += "now"
				} else if !entry.Stop.IsZero() {
					subtitle += entry.Stop.Local().Format("3:04pm")
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

// projects ----------------------------------------------

type ProjectFilter struct{}

func (c ProjectFilter) Keyword() string {
	return "projects"
}

func (c ProjectFilter) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c ProjectFilter) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword() + " ",
		Valid:        alfred.Invalid,
		SubtitleAll:  "List your projects, add new ones",
	}
}

func (c ProjectFilter) Items(prefix, query string) ([]alfred.Item, error) {
	var items []alfred.Item
	if err := checkRefresh(); err != nil {
		return items, err
	}

	parts := alfred.SplitAndTrimQuery(query)
	log.Printf("parts: %s", parts)

	projectName := parts[0]
	project, _ := findProjectByName(projectName)

	if len(parts) > 1 {
		subcommand := parts[1]
		log.Printf("subcommand: " + subcommand)

		switch subcommand {
		case "entries":
			// list time entries and let user start a new one
			var timerName string
			if len(parts) > 2 {
				timerName = parts[2]
			}

			prefix += projectName + alfred.Separator + " entries" + alfred.Separator
			entries := getLatestTimeEntriesForProject(project.Id)

			addItem := func(title, subtitle, autocomplete string) {
				data := startMessage{
					Pid:         project.Id,
					Description: title,
				}

				dataString, _ := json.Marshal(data)

				item := alfred.Item{
					Title:        title,
					SubtitleAll:  subtitle,
					Autocomplete: autocomplete,
					Arg:          "start " + string(dataString),
				}
				log.Println("arg:", item.Arg)
				items = append(items, item)
			}

			for _, entry := range entries {
				if alfred.FuzzyMatches(entry.Description, timerName) {
					addItem(entry.Description, "", prefix+" "+entry.Description)
				}
			}

			if len(items) == 0 && timerName != "" {
				addItem(timerName, "New timer for "+projectName, "")
			}

			if len(items) == 0 {
				if timerName != "" {
					addItem(timerName, "New timer for "+projectName, "")
				} else {
					items = append(items, alfred.Item{Title: "No time entries"})
				}
			}
		case "name":
			name := projectName
			arg := ""
			if len(parts) > 2 && parts[2] != "" {
				name = parts[2]
				updateProject := project
				updateProject.Name = name
				dataString, _ := json.Marshal(updateProject)
				arg = "update-project " + string(dataString)
			}
			items = append(items, alfred.Item{
				Title: "name: " + name,
				Arg:   arg,
			})
		default:
			if alfred.FuzzyMatches("entries", subcommand) {
				items = append(items, alfred.Item{
					Title:        "entries",
					Autocomplete: prefix + project.Name + alfred.Separator + " entries" + alfred.Separator + " ",
					Valid:        alfred.Invalid,
				})
			}
			if alfred.FuzzyMatches("name", subcommand) {
				items = append(items, alfred.Item{
					Title:        "name",
					Autocomplete: prefix + project.Name + alfred.Separator + " name" + alfred.Separator + " ",
					Valid:        alfred.Invalid,
				})
			}
			if alfred.FuzzyMatches("delete", subcommand) {
				data := deleteMessage{Type: "project", Id: project.Id}
				dataString, _ := json.Marshal(data)

				items = append(items, alfred.Item{
					Title:        "delete",
					Arg:          "delete " + string(dataString),
					Autocomplete: prefix + project.Name + alfred.Separator + " delete",
				})
			}

			if len(items) == 0 {
				items = append(items, alfred.Item{
					Title: fmt.Sprintf("Unknown keyword '%s'", subcommand),
					Valid: alfred.Invalid,
				})
			}
		}
	} else {
		// list projects
		var matchEntries []matchEntry
		matchQuery := strings.ToLower(query)
		runningTimer, isRunning := getRunningTimer()

		for _, entry := range cache.Account.Data.Projects {
			idx := strings.Index(strings.ToLower(entry.Name), matchQuery)
			if idx != -1 {
				matchEntries = append(matchEntries, matchEntry{
					title:   entry.Name,
					start:   idx,
					id:      entry.Id,
					portion: float32(len(query)) / float32(len(entry.Name)),
				})
			}
		}

		if len(matchEntries) == 0 && query != "" {
			data := createProjectMessage{Name: parts[0]}
			dataString, _ := json.Marshal(data)

			items = append(items, alfred.Item{
				Title:       parts[0],
				SubtitleAll: "New project",
				Arg:         "create-project " + string(dataString),
			})
		} else {
			sort.Sort(sort.Reverse(byMatchId(matchEntries)))
			sort.Stable(sort.Reverse(byBestMatch(matchEntries)))

			for _, entry := range matchEntries {
				item := alfred.Item{
					Title:        entry.title,
					SubtitleAll:  entry.subtitle,
					Valid:        alfred.Invalid,
					Autocomplete: prefix + entry.title + alfred.Separator + " ",
				}

				if isRunning && runningTimer.Pid == entry.id {
					item.Icon = "running.png"
				}

				items = append(items, item)
			}
		}

		if len(items) == 0 {
			items = append(items, alfred.Item{Title: "No matching projects"})
		}
	}

	return items, nil
}

// tags --------------------------------------------------

type TagFilter struct{}

func (c TagFilter) Keyword() string {
	return "tags"
}

func (c TagFilter) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c TagFilter) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword() + " ",
		Valid:        alfred.Invalid,
		SubtitleAll:  "List your tags",
	}
}

func (c TagFilter) Items(prefix, query string) ([]alfred.Item, error) {
	var items []alfred.Item
	if err := checkRefresh(); err != nil {
		return items, err
	}

	parts := alfred.SplitAndTrimQuery(query)
	log.Printf("parts: %d", len(parts))

	if len(parts) > 1 {
		tag, found := findTagByName(parts[0])
		if !found {
			items = append(items, alfred.Item{Title: "Invalid tag '" + parts[0] + "'"})
		}

		prefix += tag.Name + alfred.Separator
		subcommand := parts[1]

		switch subcommand {
		case "entries":
			// list time entries with this tag
			entries := getLatestTimeEntriesForTag(tag.Name)
			if len(entries) == 0 {
				items = append(items, alfred.Item{
					Title: "No time entries",
					Valid: alfred.Invalid,
				})
			} else {
				for _, entry := range entries {
					items = append(items, alfred.Item{
						Title: entry.Description,
						Valid: alfred.Invalid,
					})
				}
			}

		case "name":
			name := tag.Name
			arg := ""
			if len(parts) > 2 && parts[2] != "" {
				name = parts[2]
				updateTag := tag
				updateTag.Name = name
				dataString, _ := json.Marshal(updateTag)
				arg = "update-tag " + string(dataString)
			}
			items = append(items, alfred.Item{
				Title: "name: " + name,
				Arg:   arg,
			})
		default:
			if alfred.FuzzyMatches("entries", subcommand) {
				items = append(items, alfred.Item{
					Title:        "entries",
					Autocomplete: prefix + " entries",
					Valid:        alfred.Invalid,
				})
			}
			if alfred.FuzzyMatches("name", subcommand) {
				items = append(items, alfred.Item{
					Title:        "name",
					Autocomplete: prefix + " name" + alfred.Separator + " ",
					Valid:        alfred.Invalid,
				})
			}
			if alfred.FuzzyMatches("delete", subcommand) {
				data := deleteMessage{Type: "tag", Id: tag.Id}
				dataString, _ := json.Marshal(data)

				items = append(items, alfred.Item{
					Title:        "delete",
					Autocomplete: prefix + " delete",
					Arg:          "delete " + string(dataString),
				})
			}
		}
	} else {
		for _, entry := range cache.Account.Data.Tags {
			if alfred.FuzzyMatches(entry.Name, query) {
				items = append(items, alfred.Item{
					Title:        entry.Name,
					Autocomplete: prefix + entry.Name + alfred.Separator + " ",
					Valid:        alfred.Invalid,
				})
			}
		}

		if len(items) == 0 {
			data := createTagMessage{Name: parts[0]}
			dataString, _ := json.Marshal(data)

			items = append(items, alfred.Item{
				Title:       parts[0],
				SubtitleAll: "New tag",
				Arg:         "create-tag " + string(dataString),
			})
		}
	}

	return items, nil
}

// report ------------------------------------------------

type ReportFilter struct{}

func (c ReportFilter) Keyword() string {
	return "report"
}

func (c ReportFilter) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c ReportFilter) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword() + " ",
		Valid:        alfred.Invalid,
		SubtitleAll:  "Generate summary reports",
	}
}

func (c ReportFilter) Items(prefix, query string) ([]alfred.Item, error) {
	var items []alfred.Item
	if err := checkRefresh(); err != nil {
		return items, err
	}

	log.Printf("tell report with query '%s'", query)

	var since time.Time
	var until time.Time
	var span string
	parts := alfred.SplitAndTrimQuery(query)

	if parts[0] == "today" {
		since = toDayStart(time.Now())
		until = toDayEnd(since)
		span = "today"
	} else if parts[0] == "yesterday" {
		since = toDayStart(time.Now().AddDate(0, 0, -1))
		until = toDayEnd(since)
		span = "yesterday"
	} else if parts[0] == "this week" {
		start := time.Now()
		if start.Weekday() >= 1 {
			delta := 1 - int(start.Weekday())
			since = toDayStart(start.AddDate(0, 0, delta))
			until = toDayEnd(time.Now())
		}
		span = "this week"
	} else {
		for _, value := range []string{"today", "yesterday", "this week"} {
			if alfred.FuzzyMatches(value, query) {
				items = append(items, alfred.Item{
					Valid:        alfred.Invalid,
					Autocomplete: prefix + span + value + alfred.Separator + " ",
					Title:        value,
					SubtitleAll:  "Generate a report for " + value,
				})
			}
		}

		if len(items) == 0 {
			items = append(items, alfred.Item{
				Valid: alfred.Invalid,
				Title: "Enter a valid date or range",
			})
		}

		return items, nil
	}

	if !since.IsZero() && !until.IsZero() {
		reportItems, err := createReportItems(prefix, parts, since, until)
		if err != nil {
			return items, err
		}
		items = append(items, reportItems...)
	}

	if len(items) == 0 {
		items = append(items, alfred.Item{
			Title: "No entries",
			Valid: alfred.Invalid,
		})
	}

	return items, nil
}

// sync --------------------------------------------------

type SyncFilter struct{}

func (c SyncFilter) Keyword() string {
	return "sync"
}

func (c SyncFilter) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c SyncFilter) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword(),
		SubtitleAll:  "Sync with toggl.com",
		Valid:        alfred.Invalid,
	}
}

func (c SyncFilter) Items(prefix, query string) ([]alfred.Item, error) {
	err := refresh()
	if err != nil {
		return []alfred.Item{}, err
	}
	return []alfred.Item{alfred.Item{
		Title: "Synchronized!",
		Valid: alfred.Invalid,
	}}, nil
}

// toggle ------------------------------------------------

type ToggleAction struct{}

func (c ToggleAction) Keyword() string {
	return "toggle"
}

func (c ToggleAction) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c ToggleAction) Do(query string) (string, error) {
	log.Printf("doToggle(%s)", query)
	id, err := strconv.Atoi(query)
	if err != nil {
		return "", err
	}

	adata := &cache.Account.Data
	session := toggl.OpenSession(config.ApiKey)

	for i := 0; i < len(adata.TimeEntries); i++ {
		entry := &adata.TimeEntries[i]
		if entry.Id == id {
			var err error
			var operation string
			var updatedEntry toggl.TimeEntry

			if entry.IsRunning() {
				// two p's so we get "Stopped"
				operation = "Stopp"
				updatedEntry, err = session.StopTimeEntry(*entry)
				adata.TimeEntries[i] = updatedEntry
			} else {
				operation = "Start"
				updatedEntry, err = session.ContinueTimeEntry(*entry)
				adata.TimeEntries = append(adata.TimeEntries, updatedEntry)
			}

			if err != nil {
				return "", err
			}

			err = alfred.SaveJson(cacheFile, &cache)
			if err != nil {
				log.Printf("Error saving cache: %v\n", err)
			}

			return operation + "ed " + entry.Description, nil
		}
	}

	return "", fmt.Errorf("Invalid time entry ID %d", id)
}

// start -------------------------------------------------

type startMessage struct {
	Description string
	Pid         int
}

type StartAction struct{}

func (c StartAction) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c StartAction) Keyword() string {
	return "start"
}

func (c StartAction) Do(query string) (string, error) {
	log.Printf("start '%s'", query)

	var data startMessage
	err := json.Unmarshal([]byte(query), &data)
	if err != nil {
		return "", err
	}

	session := toggl.OpenSession(config.ApiKey)
	if err != nil {
		return "", err
	}

	var entry toggl.TimeEntry
	if data.Pid != 0 {
		entry, err = session.StartTimeEntryForProject(data.Description, data.Pid)
	} else {
		entry, err = session.StartTimeEntry(data.Description)
	}

	if err == nil {
		log.Printf("Got entry: %#v\n", entry)
		cache.Account.Data.TimeEntries = append(cache.Account.Data.TimeEntries, entry)
		err := alfred.SaveJson(cacheFile, &cache)
		if err != nil {
			log.Printf("Error saving cache: %s\n", err)
		}
	}

	return "Created time entry", err
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

// update-project ----------------------------------------

type UpdateProjectAction struct{}

func (c UpdateProjectAction) Keyword() string {
	return "update-project"
}

func (c UpdateProjectAction) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c UpdateProjectAction) Do(query string) (string, error) {
	log.Printf("update-project %s", query)

	var project toggl.Project
	err := json.Unmarshal([]byte(query), &project)
	if err != nil {
		return "", fmt.Errorf("Invalid time entry %v", query)
	}

	session := toggl.OpenSession(config.ApiKey)

	project, err = session.UpdateProject(project)
	if err != nil {
		return "", err
	}

	adata := &cache.Account.Data

	for i, p := range adata.Projects {
		if p.Id == project.Id {
			adata.Projects[i] = project
			err = alfred.SaveJson(cacheFile, &cache)
			if err != nil {
				log.Printf("Error saving cache: %v\n", err)
			}
			break
		}
	}

	return fmt.Sprintf("Updated '%s'", project.Name), nil
}

// update-tag --------------------------------------------

type UpdateTagAction struct{}

func (c UpdateTagAction) Keyword() string {
	return "update-tag"
}

func (c UpdateTagAction) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c UpdateTagAction) Do(query string) (string, error) {
	log.Printf("update-tag %s", query)

	var tag toggl.Tag
	err := json.Unmarshal([]byte(query), &tag)
	if err != nil {
		return "", fmt.Errorf("Invalid time entry %v", query)
	}

	session := toggl.OpenSession(config.ApiKey)

	tag, err = session.UpdateTag(tag)
	if err != nil {
		return "", err
	}

	// Since tags are referenced by name, updating one can cause changes in
	// multiple time entries. The simplest way to handle that is just to
	// refresh everything.
	refresh()

	return fmt.Sprintf("Updated '%s'", tag.Name), nil
}

// delete ------------------------------------------------

type deleteMessage struct {
	Type string
	Id   int
}

type DeleteAction struct{}

func (c DeleteAction) Keyword() string {
	return "delete"
}

func (c DeleteAction) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c DeleteAction) Do(query string) (string, error) {
	log.Printf("delete %s", query)

	var message deleteMessage
	err := json.Unmarshal([]byte(query), &message)
	if err != nil {
		return "", err
	}

	session := toggl.OpenSession(config.ApiKey)
	accountData := &cache.Account.Data
	var result struct {
		found   bool
		name    string
		deleted bool
	}

	switch message.Type {
	case "time entry":
		for i, entry := range accountData.TimeEntries {
			if entry.Id == message.Id {
				result.found = true
				result.name = entry.Description
				prompt := fmt.Sprintf("Are you sure you want to delete timer '%s'?", entry.Description)
				yes, _ := workflow.GetConfirmation(prompt, false)

				if yes {
					log.Printf("Deleting '" + entry.Description + "'")
					_, err := session.DeleteTimeEntry(entry)
					if err != nil {
						return "", err
					}

					if i < len(accountData.TimeEntries)-1 {
						accountData.TimeEntries = append(accountData.TimeEntries[:i], accountData.TimeEntries[i+1:]...)
					} else {
						accountData.TimeEntries = accountData.TimeEntries[:i]
					}

					result.deleted = true
				}
			}
		}
	case "project":
		for i, project := range accountData.Projects {
			if project.Id == message.Id {
				result.found = true
				result.name = project.Name
				prompt := fmt.Sprintf("Are you sure you want to delete project '%s'?", project.Name)
				yes, _ := workflow.GetConfirmation(prompt, false)

				if yes {
					log.Printf("Deleting\n")
					_, err := session.DeleteProject(project)
					if err != nil {
						return "", err
					}

					if i < len(accountData.Projects)-1 {
						accountData.Projects = append(accountData.Projects[:i], accountData.Projects[i+1:]...)
					} else {
						accountData.Projects = accountData.Projects[:i]
					}

					result.deleted = true
				}
			}
		}
	case "tag":
		for i, tag := range accountData.Tags {
			if tag.Id == message.Id {
				result.found = true
				result.name = tag.Name
				prompt := fmt.Sprintf("Are you sure you want to delete tag '%s'?", tag.Name)
				yes, _ := workflow.GetConfirmation(prompt, false)

				if yes {
					log.Printf("Deleting\n")
					_, err := session.DeleteTag(tag)
					if err != nil {
						return "", err
					}

					if i < len(accountData.Tags)-1 {
						accountData.Tags = append(accountData.Tags[:i], accountData.Tags[i+1:]...)
					} else {
						accountData.Tags = accountData.Tags[:i]
					}

					result.deleted = true
				}
			}
		}
	}

	if result.found {
		if result.deleted {
			refresh()
		}

		return "Deleted " + result.name, nil
	}

	return "", fmt.Errorf("Invalid %s ID %d", message.Type, message.Id)
}

// create-project ----------------------------------------

type createProjectMessage struct {
	Name string
	Wid  int
}

type CreateProjectAction struct{}

func (c CreateProjectAction) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c CreateProjectAction) Keyword() string {
	return "create-project"
}

func (c CreateProjectAction) Do(query string) (string, error) {
	log.Printf("create-project '%s'", query)

	var data createProjectMessage
	err := json.Unmarshal([]byte(query), &data)
	if err != nil {
		return "", err
	}

	session := toggl.OpenSession(config.ApiKey)
	if err != nil {
		return "", err
	}

	var project toggl.Project
	if data.Wid == 0 {
		data.Wid = cache.Account.Data.Workspaces[0].Id
	}
	project, err = session.CreateProject(data.Name, data.Wid)

	if err == nil {
		log.Printf("Got project: %#v\n", project)
		cache.Account.Data.Projects = append(cache.Account.Data.Projects, project)
		err := alfred.SaveJson(cacheFile, &cache)
		if err != nil {
			log.Printf("Error saving cache: %s\n", err)
		}
	}

	return fmt.Sprintf("Created project '%s'", data.Name), err
}

// create-tag --------------------------------------------

type createTagMessage struct {
	Name string
	Wid  int
}

type CreateTagAction struct{}

func (c CreateTagAction) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c CreateTagAction) Keyword() string {
	return "create-tag"
}

func (c CreateTagAction) Do(query string) (string, error) {
	log.Printf("create-tag '%s'", query)

	var data createTagMessage
	err := json.Unmarshal([]byte(query), &data)
	if err != nil {
		return "", err
	}

	session := toggl.OpenSession(config.ApiKey)
	if err != nil {
		return "", err
	}

	var tag toggl.Tag
	if data.Wid == 0 {
		data.Wid = cache.Account.Data.Workspaces[0].Id
	}
	tag, err = session.CreateTag(data.Name, data.Wid)

	if err == nil {
		log.Printf("Got tag: %#v\n", tag)
		cache.Account.Data.Tags = append(cache.Account.Data.Tags, tag)
		err := alfred.SaveJson(cacheFile, &cache)
		if err != nil {
			log.Printf("Error saving cache: %s\n", err)
		}
	}

	return fmt.Sprintf("Created tag '%s'", data.Name), err
}

// tag ---------------------------------------------------

type tagMessage struct {
	Tag       string
	TimeEntry int
	Add       bool
}

type TagAction struct{}

func (c TagAction) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c TagAction) Keyword() string {
	return "tag"
}

func (c TagAction) Do(query string) (string, error) {
	log.Printf("tag '%s'", query)

	var data tagMessage
	err := json.Unmarshal([]byte(query), &data)
	if err != nil {
		return "", err
	}

	session := toggl.OpenSession(config.ApiKey)
	if err != nil {
		return "", err
	}

	entry, err := session.AddRemoveTag(data.TimeEntry, data.Tag, data.Add)
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

	return fmt.Sprintf("Updated tag '%s' for %d", data.Tag, data.TimeEntry), err
}

// support functions /////////////////////////////////////////////////////////

func checkRefresh() error {
	if time.Now().Sub(cache.Time).Minutes() < 5.0 {
		return nil
	}

	log.Println("Refreshing cache...")
	err := refresh()
	if err != nil {
		log.Println("Error refreshing cache:", err)
	}
	return err
}

func refresh() error {
	s := toggl.OpenSession(config.ApiKey)
	account, err := s.GetAccount()
	if err != nil {
		return err
	}

	log.Printf("got account: %#v", account)

	cache.Time = time.Now()
	cache.Account = account
	cache.Workspace = account.Data.Workspaces[0].Id
	return alfred.SaveJson(cacheFile, &cache)
}

func getRunningTimer() (toggl.TimeEntry, bool) {
	for _, entry := range cache.Account.Data.TimeEntries {
		if entry.IsRunning() {
			return entry, true
		}
	}

	return toggl.TimeEntry{}, false
}

func getProjectsById() map[int]toggl.Project {
	projectsById := map[int]toggl.Project{}
	for _, proj := range cache.Account.Data.Projects {
		projectsById[proj.Id] = proj
	}
	return projectsById
}

func getProjectsByName() map[string]toggl.Project {
	projectsByName := map[string]toggl.Project{}
	for _, proj := range cache.Account.Data.Projects {
		projectsByName[proj.Name] = proj
	}
	return projectsByName
}

func findProjectByName(name string) (toggl.Project, bool) {
	for _, proj := range cache.Account.Data.Projects {
		if proj.Name == name {
			return proj, true
		}
	}
	return toggl.Project{}, false
}

func findTimerById(id int) (toggl.TimeEntry, bool) {
	for _, entry := range cache.Account.Data.TimeEntries[:] {
		if entry.Id == id {
			return entry, true
		}
	}
	return toggl.TimeEntry{}, false
}

func findTagByName(name string) (toggl.Tag, bool) {
	for _, tag := range cache.Account.Data.Tags {
		if tag.Name == name {
			return tag, true
		}
	}
	return toggl.Tag{}, false
}

func getTimeEntriesForQuery(query string) []toggl.TimeEntry {
	entries := cache.Account.Data.TimeEntries[:]
	matchQuery := strings.ToLower(query)
	matched := []toggl.TimeEntry{}

	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Description), matchQuery) {
			matched = append(matched, entry)
		}
	}

	sort.Sort(sort.Reverse(byTime(matched)))
	return matched
}

func getLatestTimeEntriesForProject(pid int) []toggl.TimeEntry {
	entries := cache.Account.Data.TimeEntries[:]
	matched := map[string]toggl.TimeEntry{}

	for _, entry := range entries {
		if entry.Pid == pid {
			e, ok := matched[entry.Description]
			if !ok || (!entry.StartTime().IsZero() && !e.StartTime().IsZero() && entry.StartTime().After(e.StartTime())) {
				matched[entry.Description] = entry
			}
		}
	}

	matchedArr := []toggl.TimeEntry{}
	for _, value := range matched {
		matchedArr = append(matchedArr, value)
	}

	sort.Sort(sort.Reverse(byTime(matchedArr)))
	return matchedArr
}

func getLatestTimeEntriesForTag(tag string) []toggl.TimeEntry {
	entries := cache.Account.Data.TimeEntries[:]
	matched := []toggl.TimeEntry{}

	for _, entry := range entries {
		for _, t := range entry.Tags {
			if t == tag {
				matched = append(matched, entry)
				break
			}
		}
	}

	sort.Sort(sort.Reverse(byTime(matched)))
	return matched
}

func createReportItems(prefix string, parts []string, since, until time.Time) ([]alfred.Item, error) {
	var items []alfred.Item

	report, err := generateReport(since, until)
	if err != nil {
		return items, err
	}

	span := parts[0]

	log.Printf("got report with %d projects\n", len(report.projects))
	log.Printf("parts: %#v\n", parts)

	if len(report.projects) > 0 {
		var total int
		var totalName string
		var name string

		if len(parts) > 1 {
			name = parts[1]
		}

		terminator := strings.Index(name, alfred.Terminator)
		if terminator != -1 {
			name := name[:terminator]

			var project *projectEntry
			for _, proj := range report.projects {
				if proj.name == name {
					project = proj
					break
				}
			}

			if project == nil {
				return items, fmt.Errorf("Couldn't find project '%s'", name)
			}

			for _, entry := range project.entries {
				item := alfred.Item{
					Valid:       alfred.Invalid,
					Title:       entry.description,
					SubtitleAll: fmt.Sprintf("%.2f", float32(entry.total)/100.0)}

				if entry.running {
					item.Icon = "running.png"
				}

				items = append(items, item)
			}

			total = project.total
			totalName = span + " for " + project.name
		} else {
			// no project name terminator, so filter projects by name
			for _, project := range report.projects {
				entryTitle := project.name
				if alfred.FuzzyMatches(entryTitle, name) {
					item := alfred.Item{
						Valid:        alfred.Invalid,
						Autocomplete: prefix + span + alfred.Separator + " " + entryTitle + alfred.Terminator,
						Title:        entryTitle,
						SubtitleAll:  fmt.Sprintf("%.2f", float32(project.total)/100.0),
					}

					if project.running {
						item.Icon = "running.png"
					}

					items = append(items, item)
					total += project.total
				}
			}

			if name == "" {
				totalName = prefix
			}
		}

		sort.Sort(alfred.ByTitle(items))

		if totalName != "" {
			item := alfred.Item{
				Title:        fmt.Sprintf("Total hours %s: %.2f", totalName, float32(total)/100.0),
				Valid:        alfred.Invalid,
				Autocomplete: prefix + strings.Join(parts, alfred.Separator+" "),
				SubtitleAll:  alfred.Line,
			}
			items = alfred.InsertItem(items, item, 0)
		}
	}

	return items, nil
}

func generateReport(since, until time.Time) (*summaryReport, error) {
	log.Printf("Generating report from %s to %s", since, until)

	report := summaryReport{projects: map[string]*projectEntry{}}
	projects := getProjectsById()

	for _, entry := range cache.Account.Data.TimeEntries {
		var start time.Time
		if entry.Start != nil {
			start = *entry.Start
		}

		if !start.Before(since) && !until.Before(start) {
			var projectName string

			if entry.Pid == 0 {
				projectName = "<No project>"
			} else {
				proj, _ := projects[entry.Pid]
				projectName = proj.Name
				log.Printf("  Project for %v: %v", entry.Pid, proj)
			}

			if _, ok := report.projects[projectName]; !ok {
				report.projects[projectName] = &projectEntry{
					name:    projectName,
					id:      entry.Pid,
					entries: map[string]*timeEntry{}}
			}

			project := report.projects[projectName]
			duration := entry.Duration

			if duration < 0 {
				duration = int(time.Now().Sub(entry.StartTime()).Seconds())
				project.running = true
			}

			duration = roundDuration(duration)
			log.Printf("  duration: %v", duration)

			if _, ok := project.entries[entry.Description]; !ok {
				project.entries[entry.Description] = &timeEntry{description: entry.Description}
			}

			if project.running {
				project.entries[entry.Description].running = true
			}

			project.entries[entry.Description].total += duration
			project.total += duration
			report.total += duration
			log.Printf("  report.total = %v", report.total)
		}
	}

	return &report, nil
}

// return a datetime at the minimum time on the given date
func toDayStart(date time.Time) time.Time {
	date = date.In(time.Local)
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
}

// return a datetime at the maximum time on the given date
func toDayEnd(date time.Time) time.Time {
	date = date.In(time.Local)
	return time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 999999999, time.Local)
}

// is date1's date before date2's date
func isDateBefore(date1 time.Time, date2 time.Time) bool {
	return (date1.Year() == date2.Year() && date1.YearDay() < date2.YearDay()) || date1.Year() < date1.Year()
}

// is date1's date after date2's date
func isDateAfter(date1 time.Time, date2 time.Time) bool {
	return (date1.Year() == date2.Year() && date2.YearDay() < date1.YearDay()) || date2.Year() < date1.Year()
}

// do date1 and date2 refer to the same date
func isSameDate(date1 time.Time, date2 time.Time) bool {
	return date1.Year() == date2.Year() && date1.Month() == date2.Month() && date1.Day() == date2.Day()
}

func isSameWeek(date1 time.Time, date2 time.Time) bool {
	y1, w1 := date1.ISOWeek()
	y2, w2 := date2.ISOWeek()
	return y1 == y2 && w1 == w2
}

func toIsoDateString(date time.Time) string {
	return fmt.Sprintf("%d-%02d-%02d", date.Year(), date.Month(), date.Day())
}

func toHumanDateString(date time.Time) string {
	date = date.Local()
	today := time.Now()

	if isSameDate(date, today) {
		return "today"
	} else if isSameDate(date, today.AddDate(0, 0, -1)) {
		return "yesterday"
	} else if isSameWeek(date, today) {
		return date.Weekday().String()
	} else if isDateAfter(date, today.AddDate(0, 0, -7)) {
		return "last " + date.Weekday().String()
	} else {
		return toIsoDateString(date)
	}
}

// convert a number of milliseconds to a fractional hour, as an int
// 1.25 hours = 125
// 0.25 hours = 25
func roundDuration(duration int) int {
	quarterHours := int(math.Ceil(float64(duration) / 900.0))
	return quarterHours * 25
}

type timeEntry struct {
	total       int
	running     bool
	description string
}

type projectEntry struct {
	total   int
	name    string
	id      int
	running bool
	entries map[string]*timeEntry
}

type summaryReport struct {
	total    int
	projects map[string]*projectEntry
}

type byTime []toggl.TimeEntry

func (b byTime) Len() int {
	return len(b)
}

func (b byTime) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byTime) Less(i, j int) bool {
	if b[i].Start.IsZero() {
		return true
	} else if b[j].Start.IsZero() {
		return false
	} else {
		return b[i].StartTime().Before(b[j].StartTime())
	}
}

type matchEntry struct {
	portion  float32
	start    int
	title    string
	subtitle string
	id       int
}

type byBestMatch []matchEntry

func (b byBestMatch) Len() int {
	return len(b)
}

func (b byBestMatch) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byBestMatch) Less(i, j int) bool {
	if b[i].portion < b[j].portion {
		return true
	}
	if b[j].portion < b[i].portion {
		return false
	}
	return false
}

type byMatchId []matchEntry

func (b byMatchId) Len() int {
	return len(b)
}

func (b byMatchId) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byMatchId) Less(i, j int) bool {
	return b[i].id < b[j].id
}
