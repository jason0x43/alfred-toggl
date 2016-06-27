package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jason0x43/go-alfred"
)

// ReportFilter is a command
type ReportFilter struct{}

// About returns information about a command
func (c ReportFilter) About() *alfred.CommandDef {
	return &alfred.CommandDef{
		Keyword:     "report",
		Description: "Generate summary reports",
		WithSpace:   true,
	}
}

// IsEnabled returns true if the command is enabled
func (c ReportFilter) IsEnabled() bool {
	return config.APIKey != ""
}

// Items returns a list of filter items
func (c ReportFilter) Items(arg, data string) (items []*alfred.Item, err error) {
	if err = checkRefresh(); err != nil {
		return
	}

	pid := -1
	var span span

	var cfg reportCfg
	if data != "" {
		if err = json.Unmarshal([]byte(data), &cfg); err != nil {
			dlog.Printf("Error unmarshalling data: %v", err)
		}
	}

	if cfg.Project != nil {
		pid = *cfg.Project
	}

	if cfg.Span != nil {
		span = *cfg.Span
	} else {
		var spanArg string
		spanArg, arg = alfred.SplitCmd(arg)

		for _, value := range []string{"today", "yesterday", "week"} {
			if alfred.FuzzyMatches(value, spanArg) {
				span, _ := getSpan(value)
				items = append(items, createReportMenuItem(&span))
			}
		}

		if matched, _ := regexp.MatchString(`^\d`, spanArg); matched {
			if span, err = getSpan(spanArg); err == nil {
				items = append(items, createReportMenuItem(&span))
			}
		}

		if len(items) == 0 {
			items = append(items, &alfred.Item{
				Title: "Enter a valid date or range",
			})
		}

		return items, nil
	}

	var grouping reportGrouping
	if cfg.Grouping != nil {
		grouping = *cfg.Grouping
	}

	var reportItems []*alfred.Item
	if reportItems, err = createReportItems(arg, data, span, pid, grouping); err != nil {
		return
	}

	items = append(items, reportItems...)

	if len(items) == 0 {
		item := &alfred.Item{
			Title: "No time entries for " + span.Name,
		}
		items = append(items, item)
	}

	return items, nil
}

// support -------------------------------------------------------------------

type reportGrouping string

const (
	groupByDay     reportGrouping = "day"
	groupByProject reportGrouping = "project"
)

type reportCfg struct {
	Project  *int            `json:"project,omitempty"`
	Span     *span           `json:"span,omitempty"`
	Grouping *reportGrouping `json:"grouping,omitempty"`
}

type span struct {
	Name     string
	Start    time.Time
	End      time.Time
	MultiDay bool
}

type dateEntry struct {
	total   int64
	name    string
	entries map[string]*timeEntry
}

type projectEntry struct {
	total   int64
	name    string
	id      int
	running bool
	entries map[string]*timeEntry
}

type timeEntry struct {
	total       int64
	running     bool
	description string
}

type summaryReport struct {
	total    int64
	projects map[string]*projectEntry
	dates    map[string]*dateEntry
}

func createReportMenuItem(s *span) (item *alfred.Item) {
	cfg := reportCfg{Span: s}

	item = &alfred.Item{
		Autocomplete: s.Name + " ",
		Title:        s.Name,
		Subtitle:     "Generate a report for " + s.Name,
		Arg: &alfred.ItemArg{
			Keyword: "report",
			Data:    alfred.Stringify(&cfg),
		},
	}

	if s.MultiDay {
		grouping := groupByDay
		cfg.Grouping = &grouping
		item.AddMod(alfred.ModAlt, item.Subtitle+", grouping entries by day", &alfred.ItemArg{
			Keyword: "report",
			Data:    alfred.Stringify(&cfg),
		})
	}

	return
}

func createReportItems(arg, data string, span span, projectID int, grouping reportGrouping) (items []*alfred.Item, err error) {
	var report *summaryReport
	if report, err = generateReport(span.Start, span.End, projectID); err != nil {
		return
	}

	dlog.Printf("creating report with data %#v", data)

	cfg := reportCfg{Span: &span}

	var total int64
	var totalName string

	if grouping == groupByDay {
		dlog.Printf("checking %d dates", len(report.dates))
		for _, date := range report.dates {
			totalName = "for " + span.Name
			entryTitle := date.name

			if alfred.FuzzyMatches(entryTitle, arg) {
				if span, e := getSpan(date.name); e == nil {
					cfg.Span = &span
				} else {
					dlog.Printf("Error getting span for %s: %v", date.name, e)
				}

				items = append(items, &alfred.Item{
					Title:    entryTitle,
					Subtitle: fmt.Sprintf("%.2f", float32(date.total)/100.0),
					// Arg: &alfred.ItemArg{
					// 	Keyword: "report",
					// 	Data:    alfred.Stringify(&cfg),
					// },
				})

				total += date.total
			}
		}
	} else {
		dlog.Printf("checking %d projects", len(report.projects))
		for _, project := range report.projects {
			if projectID != -1 {
				dlog.Printf("have projectID: %d", projectID)

				totalName = fmt.Sprintf("for %s for %s", span.Name, project.name)
				for desc, entry := range project.entries {
					dlog.Printf("getting info for %#v", entry)
					entryTitle := desc
					if alfred.FuzzyMatches(entryTitle, arg) {
						item := &alfred.Item{
							Title:    entryTitle,
							Subtitle: fmt.Sprintf("%.2f", float32(entry.total)/100.0),
						}

						if entry.running {
							item.Icon = "running.png"
						}

						items = append(items, item)
					}
				}
			} else {
				totalName = "for " + span.Name
				entryTitle := project.name

				cfg.Project = &project.id
				dlog.Printf("checking if '%s' fuzzyMatches '%s'", arg, entryTitle)

				if alfred.FuzzyMatches(entryTitle, arg) {
					item := &alfred.Item{
						Title:    entryTitle,
						Subtitle: fmt.Sprintf("%.2f", float32(project.total)/100.0),
						Arg: &alfred.ItemArg{
							Keyword: "report",
							Data:    alfred.Stringify(&cfg),
						},
					}

					if project.running {
						item.Icon = "running.png"
					}

					items = append(items, item)
					total += project.total
				}
			}
		}
	}

	sort.Sort(alfred.ByTitle(items))

	if totalName != "" && arg == "" {
		title := fmt.Sprintf("Total hours %s: %.2f", totalName, float32(total)/100.0)
		item := &alfred.Item{
			Title:    title,
			Subtitle: alfred.Line,
		}

		cfg.Project = nil

		if projectID == -1 {
			cfg.Span = nil
		}

		item.Arg = &alfred.ItemArg{
			Keyword: "report",
			Data:    alfred.Stringify(&cfg),
		}

		items = alfred.InsertItem(items, item, 0)
	}

	return
}

// expand fills in the start and end times for a span
func getSpan(arg string) (s span, err error) {
	if arg == "today" {
		s.Name = arg
		s.Start = toDayStart(time.Now())
		s.End = toDayEnd(s.Start)
	} else if arg == "yesterday" {
		s.Name = arg
		s.Start = toDayStart(time.Now().AddDate(0, 0, -1))
		s.End = toDayEnd(s.Start)
	} else if arg == "week" {
		s.Name = "this week"
		start := time.Now()
		// TODO: consider configurable work week bounds
		delta := -int(start.Weekday())
		s.Start = toDayStart(start.AddDate(0, 0, delta))
		s.End = toDayEnd(time.Now())
		s.MultiDay = true
	} else {
		if strings.Contains(arg, "..") {
			parts := alfred.CleanSplitN(arg, "..", 2)
			if len(parts) == 2 {
				var span1 span
				var span2 span
				if span1, err = getSpan(parts[0]); err == nil {
					if span2, err = getSpan(parts[1]); err == nil {
						s.Name = arg
						s.Start = span1.Start
						s.End = span2.End
						s.MultiDay = true
					}
				}
			}
		} else {
			if layout := getDateLayout(arg); layout != "" {
				if s.Start, err = time.Parse(layout, arg); err != nil {
					return
				}
				year := s.Start.Year()
				if year == 0 {
					year = time.Now().Year()
				}
				s.Name = arg
				s.Start = time.Date(year, s.Start.Month(), s.Start.Day(), 0, 0, 0, 0, time.Local)
				s.End = time.Date(year, s.Start.Month(), s.Start.Day(), 23, 59, 59, 999999999, time.Local)
			}
		}
	}

	if err == nil && s.Name == "" {
		err = fmt.Errorf("Unable to parse span '%s'", arg)
	}

	return
}

func generateReport(since, until time.Time, projectID int) (*summaryReport, error) {
	dlog.Printf("Generating report from %s to %s for %d", since, until, projectID)

	report := summaryReport{
		projects: map[string]*projectEntry{},
		dates:    map[string]*dateEntry{},
	}
	projects := getProjectsByID()

	for _, entry := range cache.Account.Data.TimeEntries {
		start := entry.StartTime()

		if !start.Before(since) && !until.Before(start) {
			if projectID != -1 && entry.Pid != projectID {
				continue
			}

			var projectName string

			if entry.Pid == 0 {
				projectName = "<No project>"
			} else {
				proj, _ := projects[entry.Pid]
				projectName = proj.Name
			}

			if _, ok := report.projects[projectName]; !ok {
				report.projects[projectName] = &projectEntry{
					name:    projectName,
					id:      entry.Pid,
					entries: map[string]*timeEntry{}}
			}

			date := start.Format("1/2")
			if _, ok := report.dates[date]; !ok {
				report.dates[date] = &dateEntry{
					name:    date,
					entries: map[string]*timeEntry{}}
			}

			project := report.projects[projectName]
			dateEntry := report.dates[date]
			duration := entry.Duration

			if duration < 0 {
				duration = int64(time.Now().Sub(entry.StartTime()).Seconds())
				project.running = true
			}

			duration = roundDuration(duration)

			if _, ok := project.entries[entry.Description]; !ok {
				project.entries[entry.Description] = &timeEntry{description: entry.Description}
			}

			if project.running {
				project.entries[entry.Description].running = true
			}

			project.entries[entry.Description].total += duration
			dateEntry.total += duration
			project.total += duration
			report.total += duration
		}
	}

	return &report, nil
}

var dateFormats = map[string]*regexp.Regexp{
	"1/2":      regexp.MustCompile(`^\d\d?\/\d\d?$`),
	"1/2/06":   regexp.MustCompile(`^\d\d?\/\d\d?\/\d\d$`),
	"1/2/2006": regexp.MustCompile(`^\d\d?\/\d\d?\/\d\d\d\d$`),
	"2006-1-2": regexp.MustCompile(`^\d\d\d\d-\d\d?-\d\d$`),
}

// return true if the string can be parsed as a date
func getDateLayout(s string) string {
	for layout, matcher := range dateFormats {
		if matcher.MatchString(s) {
			return layout
		}
	}
	return ""
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
