package main

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-toggl"
)

func checkRefresh() error {
	if config.TestMode {
		log.Printf("Test mode is active; not auto-refreshing")
		return nil
	}

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

func projectHasTimeEntries(pid int) bool {
	entries := cache.Account.Data.TimeEntries
	for i, _ := range entries {
		if entries[i].Pid == pid {
			return true
		}
	}
	return false
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

func tagHasTimeEntries(tag string) bool {
	entries := cache.Account.Data.TimeEntries
	for i, _ := range entries {
		if entries[i].HasTag(tag) {
			return true
		}
	}
	return false
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
				totalName = strings.TrimRight(prefix, " ")
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
		start := entry.StartTime()

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
	if b[i].StartTime().IsZero() {
		return true
	} else if b[j].StartTime().IsZero() {
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
