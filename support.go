package main

import (
	"fmt"
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

func getRunningTimer() (timer toggl.TimeEntry, found bool) {
	for _, entry := range cache.Account.Data.TimeEntries {
		if entry.IsRunning() {
			return entry, true
		}
	}

	return
}

func getProjectsByID() (projectsByID map[int]toggl.Project) {
	projectsByID = map[int]toggl.Project{}
	for _, proj := range cache.Account.Data.Projects {
		projectsByID[proj.ID] = proj
	}
	return
}

func getProjectsByName() (projectsByName map[string]toggl.Project) {
	projectsByName = map[string]toggl.Project{}
	for _, proj := range cache.Account.Data.Projects {
		projectsByName[proj.Name] = proj
	}
	return
}

func findProjectByName(name string) (project toggl.Project, found bool) {
	for _, proj := range cache.Account.Data.Projects {
		if proj.Name == name {
			return proj, true
		}
	}
	return
}

func getProjectByID(id int) (project toggl.Project, index int, found bool) {
	for i, proj := range cache.Account.Data.Projects {
		if proj.ID == id {
			return proj, i, true
		}
	}
	return
}

func getTagByID(id int) (tag toggl.Tag, index int, found bool) {
	for i, entry := range cache.Account.Data.Tags[:] {
		if entry.ID == id {
			return entry, i, true
		}
	}
	return
}

func getTimerByID(id int) (timer toggl.TimeEntry, index int, found bool) {
	for i, entry := range cache.Account.Data.TimeEntries[:] {
		if entry.ID == id {
			return entry, i, true
		}
	}
	return
}

func getClientByID(id int) (client toggl.Client, index int, found bool) {
	for i, client := range cache.Account.Data.Clients {
		if client.ID == id {
			return client, i, true
		}
	}
	return
}

func getWorkspaceByID(id int) (workspace toggl.Workspace, index int, found bool) {
	for i, workspace := range cache.Account.Data.Workspaces {
		if workspace.ID == id {
			return workspace, i, true
		}
	}
	return
}

func findTimersByProjectID(pid int) (entries []toggl.TimeEntry) {
	for _, entry := range cache.Account.Data.TimeEntries[:] {
		if entry.Pid == pid {
			entries = append(entries, entry)
		}
	}
	return
}

func findTimersByTag(tag string) (entries []toggl.TimeEntry) {
	for _, entry := range cache.Account.Data.TimeEntries[:] {
		for _, t := range entry.Tags {
			if t == tag {
				entries = append(entries, entry)
				break
			}
		}
	}
	return
}

func findTagByName(name string) (tag toggl.Tag, found bool) {
	for _, tag := range cache.Account.Data.Tags {
		if tag.Name == name {
			return tag, true
		}
	}
	return
}

func findTagNameByID(id int) (name string, found bool) {
	for _, tag := range cache.Account.Data.Tags {
		if tag.ID == id {
			return tag.Name, true
		}
	}
	return
}

func getTimeEntriesForQuery(query string) (matched []toggl.TimeEntry) {
	entries := cache.Account.Data.TimeEntries[:]
	matchQuery := strings.ToLower(query)

	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Description), matchQuery) {
			matched = append(matched, entry)
		}
	}

	sort.Sort(sort.Reverse(byTime(matched)))
	return
}

func getLatestTimeEntriesForProject(pid int) (matchedArr []toggl.TimeEntry) {
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

	for _, value := range matched {
		matchedArr = append(matchedArr, value)
	}

	sort.Sort(sort.Reverse(byTime(matchedArr)))
	return
}

func isWorkspacePremium(id int) bool {
	workspace, _, _ := getWorkspaceByID(id)
	return workspace.Premium
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

func getLatestTimeEntriesForTag(tag string) (matched []toggl.TimeEntry) {
	entries := cache.Account.Data.TimeEntries[:]

	for _, entry := range entries {
		for _, t := range entry.Tags {
			if t == tag {
				matched = append(matched, entry)
				break
			}
		}
	}

	sort.Sort(sort.Reverse(byTime(matched)))
	return
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

// roundDuration converts a number of seconds to a quantized fractional hour, as an int
//
//   1.25 hours = 125
//   0.25 hours = 25
//
// If the `floor` argument is true, any fractional part of the pre-quantized
// value is truncated before quantization.
//
//   floor == false: 1.05 -> 1.25 -> 125
//   floor == true: 1.05 -> 1.00 -> 100
//
func roundDuration(duration int64, floor bool) int64 {
	if config.Rounding != 0 {
		// the number of seconds in the rounding increment
		incr := config.Rounding * 60

		// the number of increments in the duration
		var fracHours int64
		if floor {
			fracHours = int64(math.Floor(float64(duration) / float64(incr)))
		} else {
			fracHours = int64(math.Ceil(float64(duration) / float64(incr)))
		}

		// the fraction of hour that is being rounded to
		frac := 60.0 / float64(config.Rounding)

		return fracHours * int64((100.0 / frac))
	}

	// not rounding, so just return the duration as a number of hours * 100
	hours := float64(duration) / 3600.0
	return int64(hours * 100)
}

// formatTime formats a duration in hours*100 (the return value of
// roundDuration) according to the current configured format (fractional time
// or hh:mm)
func formatDuration(hoursTimes100 int64) string {
	if config.HoursMinutes {
		hours := float64(hoursTimes100) / 100.0
		wholeHours := int64(hours)
		minutes := round((hours - float64(wholeHours)) * 60.0)
		return fmt.Sprintf("%d:%02d", wholeHours, minutes)
	}

	return fmt.Sprintf("%.2f", float64(hoursTimes100)/100.0)
}

// round rounds a float64, returning an int64
func round(value float64) int64 {
	return int64(math.Floor(value + 0.5))
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
	portion  float64
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

type byMatchID []matchEntry

func (b byMatchID) Len() int {
	return len(b)
}

func (b byMatchID) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byMatchID) Less(i, j int) bool {
	return b[i].id < b[j].id
}
