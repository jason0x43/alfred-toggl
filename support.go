package main

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-toggl"
)

func checkRefresh() error {
	if config.TestMode {
		dlog.Printf("Test mode is active; not auto-refreshing")
		return nil
	}

	if time.Now().Sub(cache.Time).Minutes() < 5.0 {
		return nil
	}

	dlog.Println("Refreshing cache...")
	err := refresh()
	if err != nil {
		dlog.Println("Error refreshing cache:", err)
	}
	return err
}

func refresh() error {
	s := toggl.OpenSession(config.APIKey)
	account, err := s.GetAccount()
	if err != nil {
		return err
	}

	dlog.Printf("got account: %#v", account)

	cache.Time = time.Now()
	cache.Account = account
	cache.Workspace = account.Data.Workspaces[0].ID
	return alfred.SaveJSON(cacheFile, &cache)
}

func getRunningTimer() (toggl.TimeEntry, bool) {
	for _, entry := range cache.Account.Data.TimeEntries {
		if entry.IsRunning() {
			return entry, true
		}
	}

	return toggl.TimeEntry{}, false
}

func getProjectsByID() map[int]toggl.Project {
	projectsByID := map[int]toggl.Project{}
	for _, proj := range cache.Account.Data.Projects {
		projectsByID[proj.ID] = proj
	}
	return projectsByID
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

func getProjectByID(id int) (toggl.Project, int, bool) {
	for i, proj := range cache.Account.Data.Projects {
		if proj.ID == id {
			return proj, i, true
		}
	}
	return toggl.Project{}, 0, false
}

func getTimerByID(id int) (toggl.TimeEntry, int, bool) {
	for i, entry := range cache.Account.Data.TimeEntries[:] {
		if entry.ID == id {
			return entry, i, true
		}
	}
	return toggl.TimeEntry{}, 0, false
}

func findTimersByProjectID(pid int) []toggl.TimeEntry {
	var entries []toggl.TimeEntry
	for _, entry := range cache.Account.Data.TimeEntries[:] {
		if entry.Pid == pid {
			entries = append(entries, entry)
		}
	}
	return entries
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
	for i := range entries {
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
	for i := range entries {
		if entries[i].HasTag(tag) {
			return true
		}
	}
	return false
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
	return date.Format("2006-01-02")
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

// convert a number of seconds to a fractional hour, as an int
// 1.25 hours = 125
// 0.25 hours = 25
func roundDuration(duration int64) int64 {
	if config.Rounding != 0 {
		incr := float64(config.Rounding * 60)
		frac := 60.0 / float64(config.Rounding)
		fracHours := int64(math.Ceil(float64(duration) / incr))
		return fracHours * int64((100.0 / frac))
	}

	hours := float64(duration) / 3600.0
	return int64(hours * 100)
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
