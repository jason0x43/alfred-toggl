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

type Config struct {
	ApiKey string `json:"api_key"`
}

type Cache struct {
	Workspace int
	Account   toggl.Account
	Time      time.Time
}

func main() {
	workflow, err := alfred.OpenWorkflow(".", true)
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
		TimerFilter{},
		ProjectsFilter{},
		TagsFilter{},
		ReportFilter{},
		SyncFilter{},
		LogoutCommand{},
		StartAction{},
		UpdateTimerAction{},
		ToggleAction{},
		DeleteAction{},
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
	btn, username, err := alfred.GetInput("Toggl", "Username", "", false)
	if err != nil {
		return "", err
	}

	if btn != "Ok" {
		log.Println("User didn't click OK")
		return "", nil
	}
	log.Printf("username: %s", username)

	btn, password, err := alfred.GetInput("Toggl", "Password", "", true)
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

	alfred.ShowMessage("Toggl", "Login successful!")
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
		SubtitleAll:  "Logout of Toggl",
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

	alfred.ShowMessage("Toggl", "You are now logged out of Toggl")
	return "", nil
}

// timers ------------------------------------------------

type TimerFilter struct{}

func (c TimerFilter) Keyword() string {
	return "timer"
}

func (c TimerFilter) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c TimerFilter) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword() + " ",
		SubtitleAll:  "List and modify recent time entries, add new ones",
	}
}

func (c TimerFilter) Items(prefix, query string) (items []alfred.Item, err error) {
	if err = checkRefresh(); err != nil {
		return items, err
	}

	projects := getProjectsById()
	parts := alfred.SplitAndTrimQuery(query)
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		id = 0
	}
	timer, ok := findTimerById(id)

	if ok {
		var property string
		if len(parts) > 1 {
			property = strings.ToLower(parts[1])
		}

		addItem := func(title, subtitle, keyword string, hasNext, showSubtitle bool) {
			item := alfred.Item{
				Title:        title,
				Autocomplete: prefix + parts[0] + alfred.SEPARATOR + " " + keyword}

			if showSubtitle {
				item.Subtitle = subtitle
			}

			if hasNext {
				item.Autocomplete += alfred.SEPARATOR + " "
			}

			if len(parts) > 2 {
				item.Arg = parts[2]
			} else {
				item.Valid = alfred.INVALID
			}

			items = append(items, item)
		}

		if alfred.FuzzyMatches("description", property) {
			if len(parts) > 2 && parts[2] != "" {
				updateTimer := toggl.TimeEntry{
					Id:          timer.Id,
					Description: parts[2],
				}

				dataString, _ := json.Marshal(updateTimer)

				item := alfred.Item{
					Title:    "Description: " + parts[2],
					Subtitle: "Change description",
					Arg:      "update-timer " + string(dataString),
				}

				items = append(items, item)
			} else {
				addItem("Description: "+timer.Description, "Change description", "Description", true, property == "description")
			}
		}

		if alfred.FuzzyMatches("project", property) {
			if len(parts) > 2 {
				var matches func(string, string) bool
				name := parts[2]
				complete := prefix + parts[0] + alfred.SEPARATOR + " " + "Project" + alfred.SEPARATOR + " "
				terminator := strings.Index(parts[2], alfred.TERMINATOR)

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
							Autocomplete: complete + proj.Name + alfred.TERMINATOR})
					}
				}
			} else {
				title := "Project: "
				if project, ok := projects[timer.Pid]; ok {
					title += project.Name
				} else {
					title += "<None>"
				}
				addItem(title, "Change project", "Project", true, property == "project")
			}
		}

		if alfred.FuzzyMatches("start", property) {
			var startTime string
			if timer.Start != nil {
				startTime = timer.Start.Local().Format("15:04")
			}

			item := alfred.Item{
				Title:        "Start: " + startTime,
				Valid:        alfred.INVALID,
				Autocomplete: prefix + parts[0] + alfred.SEPARATOR + " Start" + alfred.SEPARATOR + " ",
			}

			if property == "start" {
				item.Subtitle = "Change start time"
			}

			if len(parts) > 2 && parts[2] != "" {
				newTime, err := time.Parse("15:04", parts[2])
				if err == nil {
					originalStart := timer.Start.Local()
					originalMinutes := originalStart.Hour()*60 + originalStart.Minute()
					newMinutes := newTime.Hour()*60 + newTime.Minute()

					delta, _ := time.ParseDuration(fmt.Sprintf("%dm", newMinutes-originalMinutes))
					newStart := originalStart.Add(delta)

					updateTimer := toggl.TimeEntry{
						Id:    timer.Id,
						Start: &newStart,
					}

					if !timer.IsRunning() {
						updateTimer.Duration = timer.Duration
					}

					dataString, _ := json.Marshal(updateTimer)
					log.Printf("marshaled timer to: %s\n", dataString)

					item.Title = "Start: " + parts[2]
					item.Subtitle = "Press enter to change start time (end time will also be adjusted)"
					item.Arg = "update-timer " + string(dataString)
					item.Valid = ""
				} else {
					log.Printf("Invalid time: %s\n", parts[2])
				}
			}

			items = append(items, item)
		}

		if !timer.IsRunning() {
			if alfred.FuzzyMatches("stop", property) {
				stopTime := timer.Stop
				addItem("Stop: "+stopTime.Local().Format("15:04"), "", "Stop", false, false)
			}

			if alfred.FuzzyMatches("duration", property) {
				duration := float32(timer.Duration) / 60.0 / 60.0

				item := alfred.Item{
					Title:        fmt.Sprintf("Duration: %.2f", duration),
					Valid:        alfred.INVALID,
					Autocomplete: prefix + parts[0] + alfred.SEPARATOR + " Duration" + alfred.SEPARATOR + " ",
				}

				if property == "duration" {
					item.Subtitle = "Change duration (end time will be adjusted)"
				}

				if len(parts) > 2 && parts[2] != "" {
					val, err := strconv.ParseFloat(parts[2], 64)
					if err == nil {
						updateTimer := toggl.TimeEntry{
							Id:       timer.Id,
							Duration: int(val * 60 * 60),
						}

						dataString, _ := json.Marshal(updateTimer)
						log.Printf("marshaled timer to: %s\n", dataString)

						item.Title = fmt.Sprintf("Duration: %.2f", val)
						item.Subtitle = "Press enter to change duration (end time will be adjusted)"
						item.Arg = "update-timer " + string(dataString)
						item.Valid = ""
					}
				}

				items = append(items, item)
			}
		}

		if alfred.FuzzyMatches("delete", property) {
			item := alfred.Item{
				Title:        "Delete",
				Arg:          fmt.Sprintf("delete %d", timer.Id),
				Autocomplete: prefix + parts[0] + alfred.SEPARATOR + " Delete",
			}

			if property == "delete" {
				item.Subtitle = "Delete this time entry"
			}

			items = append(items, item)
		}
	} else {
		timers := getTimersForQuery(parts[0])

		if len(timers) == 0 {
			data := startMessage{Description: parts[0]}
			dataString, _ := json.Marshal(data)

			items = append(items, alfred.Item{
				Title:       parts[0],
				SubtitleAll: "New timer",
				Arg:         "start " + string(dataString),
			})
		} else {
			for _, entry := range timers {
				var seconds int

				startTime := entry.Start
				if entry.Duration < 0 {
					seconds = int(time.Now().Sub(*startTime).Seconds())
				} else {
					seconds = entry.Duration
				}

				duration := float32(roundDuration(seconds)) / 100.0
				subtitle := fmt.Sprintf("%.2f, %s from ", duration, toHumanDateString(*startTime))
				subtitle += startTime.Local().Format("3:04pm") + " to "
				if entry.Duration < 0 {
					subtitle += "now"
				} else {
					subtitle += entry.Stop.Local().Format("3:04pm")
				}

				if project, ok := projects[entry.Pid]; ok {
					subtitle = "[" + project.Name + "] " + subtitle
				}

				item := alfred.Item{
					Title:        entry.Description,
					SubtitleAll:  subtitle,
					Arg:          fmt.Sprintf("toggle %v", entry.Id),
					Autocomplete: prefix + fmt.Sprintf("%d%s ", entry.Id, alfred.SEPARATOR),
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

type ProjectsFilter struct{}

func (c ProjectsFilter) Keyword() string {
	return "projects"
}

func (c ProjectsFilter) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c ProjectsFilter) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword() + " ",
		SubtitleAll:  "List your projects, add new ones",
	}
}

func (c ProjectsFilter) Items(prefix, query string) ([]alfred.Item, error) {
	var items []alfred.Item
	if err := checkRefresh(); err != nil {
		return items, err
	}

	parts := alfred.SplitAndTrimQuery(query)

	if len(parts) > 1 {
		// list timers and let user start a new one
		projectName := parts[0]
		project, _ := findProjectByName(projectName)
		timerName := parts[1]
		timers := getLatestTimerForProject(project.Id)

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

		for _, entry := range timers {
			if alfred.FuzzyMatches(entry.Description, timerName) {
				addItem(entry.Description, "", prefix+projectName+alfred.SEPARATOR+" "+entry.Description)
			}
		}

		if len(items) == 0 && parts[1] != "" {
			addItem(timerName, "New timer for "+projectName, "")
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

		sort.Sort(sort.Reverse(byMatchId(matchEntries)))
		sort.Stable(sort.Reverse(byBestMatch(matchEntries)))

		for _, entry := range matchEntries {
			item := alfred.Item{
				Title:        entry.title,
				SubtitleAll:  entry.subtitle,
				Valid:        alfred.INVALID,
				Autocomplete: prefix + entry.title + alfred.SEPARATOR + " ",
			}

			if isRunning && runningTimer.Pid == entry.id {
				item.Icon = "running.png"
			}

			items = append(items, item)
		}
	}

	return items, nil
}

// tags --------------------------------------------------

type TagsFilter struct{}

func (c TagsFilter) Keyword() string {
	return "tags"
}

func (c TagsFilter) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c TagsFilter) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword() + " ",
		SubtitleAll:  "List your tags",
	}
}

func (c TagsFilter) Items(prefix, query string) ([]alfred.Item, error) {
	var items []alfred.Item
	if err := checkRefresh(); err != nil {
		return items, err
	}

	for _, entry := range cache.Account.Data.Tags {
		if alfred.FuzzyMatches(entry.Name, query) {
			items = append(items, alfred.Item{
				Title:        entry.Name,
				Autocomplete: prefix + entry.Name,
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
		SubtitleAll:  "Generate summary reports",
	}
}

func (c ReportFilter) Items(prefix, query string) ([]alfred.Item, error) {
	var items []alfred.Item
	if err := checkRefresh(); err != nil {
		return items, err
	}

	log.Printf("tell report with query '%s'", query)

	var since string
	var until string
	var span string
	parts := alfred.SplitAndTrimQuery(query)

	if parts[0] == "today" {
		since = toIsoDateString(time.Now())
		until = since
		span = "today"
	} else if parts[0] == "yesterday" {
		since = toIsoDateString(time.Now().AddDate(0, 0, -1))
		until = since
		span = "yesterday"
	} else if parts[0] == "this week" {
		start := time.Now()
		if start.Weekday() >= 1 {
			delta := 1 - int(start.Weekday())
			since = toIsoDateString(start.AddDate(0, 0, delta))
			until = toIsoDateString(time.Now())
		}
		span = "this week"
	} else {
		for _, value := range []string{"today", "yesterday", "this week"} {
			if alfred.FuzzyMatches(value, query) {
				items = append(items, alfred.Item{
					Valid:        alfred.INVALID,
					Autocomplete: prefix + span + value + alfred.SEPARATOR + " ",
					Title:        value,
					SubtitleAll:  "Generate a report for " + value,
				})
			}
		}

		if len(items) == 0 {
			items = append(items, alfred.Item{
				Valid: alfred.INVALID,
				Title: "Enter a valid date or range",
			})
		}

		return items, nil
	}

	if since != "" && until != "" {
		reportItems, err := createReportItems(prefix, parts, since, until)
		if err != nil {
			return items, err
		}
		items = append(items, reportItems...)
	}

	if len(items) == 0 {
		items = append(items, alfred.Item{
			Title: "No entries",
			Valid: alfred.INVALID,
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
	}
}

func (c SyncFilter) Items(prefix, query string) ([]alfred.Item, error) {
	err := refresh()
	if err != nil {
		return []alfred.Item{}, err
	}
	return []alfred.Item{alfred.Item{Title: "Synchronized!"}}, nil
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
				updatedEntry, err = session.StopTimer(*entry)
				adata.TimeEntries[i] = updatedEntry
			} else {
				operation = "Start"
				updatedEntry, err = session.ContinueTimer(*entry)
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
		entry, err = session.StartTimerForProject(data.Description, data.Pid)
	} else {
		entry, err = session.StartTimer(data.Description)
	}

	if err == nil {
		log.Printf("Got entry: %#v\n", entry)
		cache.Account.Data.TimeEntries = append(cache.Account.Data.TimeEntries, entry)
		err := alfred.SaveJson(cacheFile, &cache)
		if err != nil {
			log.Printf("Error saving cache: %s\n", err)
		}
	}

	return "Started timer", err
}

// update-timer ------------------------------------------

type UpdateTimerAction struct{}

func (c UpdateTimerAction) Keyword() string {
	return "update-timer"
}

func (c UpdateTimerAction) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c UpdateTimerAction) Do(query string) (string, error) {
	log.Printf("update-timer %s", query)

	var entry toggl.TimeEntry
	err := json.Unmarshal([]byte(query), &entry)
	if err != nil {
		return "", fmt.Errorf("Invalid time entry %v", query)
	}

	session := toggl.OpenSession(config.ApiKey)

	entry, err = session.UpdateTimer(entry)
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

// delete ------------------------------------------------

type DeleteAction struct{}

func (c DeleteAction) Keyword() string {
	return "delete"
}

func (c DeleteAction) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c DeleteAction) Do(query string) (string, error) {
	session := toggl.OpenSession(config.ApiKey)

	log.Printf("delete %s", query)
	id, err := strconv.Atoi(query)

	if err == nil {
		accountData := &cache.Account.Data
		for i := 0; i < len(accountData.TimeEntries); i++ {
			entry := &accountData.TimeEntries[i]
			if entry.Id == id {
				prompt := fmt.Sprintf("Are you sure you want to delete '%s'?", entry.Description)
				yes, _ := alfred.GetConfirmation("Confirm", prompt, false)

				if yes {
					log.Printf("Deleting\n")
					_, err := session.DeleteTimer(*entry)
					if err != nil {
						return "", err
					}

					if i < len(accountData.TimeEntries)-1 {
						accountData.TimeEntries = append(accountData.TimeEntries[:i], accountData.TimeEntries[i+1:]...)
					} else {
						accountData.TimeEntries = accountData.TimeEntries[:i]
					}

					err = alfred.SaveJson(cacheFile, &cache)
					if err != nil {
						log.Printf("Error saving cache: %v\n", err)
					}
				} else {
					log.Printf("Not deleting\n")
				}

				return "Deleted " + entry.Description, nil
			}
		}
	}

	return "", fmt.Errorf("Invalid time entry ID %d", id)
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

func getTimersForQuery(query string) []toggl.TimeEntry {
	timers := cache.Account.Data.TimeEntries[:]
	matchQuery := strings.ToLower(query)
	matched := []toggl.TimeEntry{}

	for _, entry := range timers {
		if strings.Contains(strings.ToLower(entry.Description), matchQuery) {
			matched = append(matched, entry)
		}
	}

	sort.Sort(sort.Reverse(byTime(matched)))
	return matched
}

func getLatestTimerForProject(pid int) []toggl.TimeEntry {
	timers := cache.Account.Data.TimeEntries[:]
	matched := map[string]toggl.TimeEntry{}

	for _, entry := range timers {
		if entry.Pid == pid {
			e, ok := matched[entry.Description]
			if !ok || (entry.Start != nil && e.Start != nil && entry.Start.After(*e.Start)) {
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

func createReportItems(prefix string, parts []string, since, until string) ([]alfred.Item, error) {
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

		terminator := strings.Index(name, alfred.TERMINATOR)
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
					Valid:       alfred.INVALID,
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
						Valid:        alfred.INVALID,
						Autocomplete: prefix + span + alfred.SEPARATOR + " " + entryTitle + alfred.TERMINATOR,
						Title:        entryTitle,
						SubtitleAll:  fmt.Sprintf("%.2f", float32(project.total)/100.0)}

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
				Valid:        alfred.INVALID,
				Autocomplete: prefix + strings.Join(parts, alfred.SEPARATOR+" "),
				SubtitleAll:  alfred.LINE}
			items = alfred.InsertItem(items, item, 0)
		}
	}

	return items, nil
}

func generateReport(since string, until string) (*summaryReport, error) {
	log.Printf("Generating report from %s to %s", since, until)

	report := summaryReport{projects: map[string]*projectEntry{}}
	projects := getProjectsById()

	for _, entry := range cache.Account.Data.TimeEntries {
		var start string
		if entry.Start != nil {
			start = entry.Start.Format("2006-01-02")
		}

		if start >= since && start <= until {
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
				duration = int(time.Now().Sub(*entry.Start).Seconds())
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
	if b[i].Start == nil {
		return true
	} else if b[j].Start == nil {
		return false
	} else {
		return b[i].Start.Before(*b[j].Start)
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
