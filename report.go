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
func (c ReportFilter) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     "report",
		Description: "Generate summary reports",
		IsEnabled:   config.APIKey != "",
	}
}

// Items returns a list of filter items
func (c ReportFilter) Items(arg, data string) (items []alfred.Item, err error) {
	if err = checkRefresh(); err != nil {
		return
	}

	var cfg reportCfg
	if data != "" {
		if err = json.Unmarshal([]byte(data), &cfg); err != nil {
			dlog.Printf("Error unmarshalling data: %v", err)
		}
	}

	var span span
	if cfg.Span != nil {
		span = *cfg.Span
	} else {
		var spanArg string
		spanArg, arg = alfred.SplitCmd(arg)

		for _, value := range []string{"today", "yesterday", "week"} {
			if alfred.FuzzyMatches(value, spanArg) {
				span, _ := getSpan(value)
				items = append(items, createReportMenuItem(span))
			}
		}

		if matched, _ := regexp.MatchString(`^\d`, spanArg); matched {
			if span, err = getSpan(spanArg); err == nil {
				items = append(items, createReportMenuItem(span))
			}
		}

		if len(items) == 0 {
			items = append(items, alfred.Item{
				Title: "Enter a valid date or range",
			})
		}

		return items, nil
	}

	var reportItems []alfred.Item
	if reportItems, err = createReportItems(arg, data, &cfg, span); err != nil {
		return
	}

	items = append(items, reportItems...)

	if len(items) == 0 {
		cfg.Span = nil

		item := alfred.Item{
			Title: "No time entries for " + span.Name,
			Arg: &alfred.ItemArg{
				Keyword: "report",
				Data:    alfred.Stringify(&cfg),
			},
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
	Project    *int            `json:"project,omitempty"`
	EntryTitle *string         `json:"entrytitle,omitempty"`
	Span       *span           `json:"span,omitempty"`
	Grouping   *reportGrouping `json:"grouping,omitempty"`
	Previous   *reportCfg      `json:"previous,omitempty"`
}

type span struct {
	Name     string
	Label    string
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

func createReportMenuItem(s span) (item alfred.Item) {
	cfg := reportCfg{Span: &s}

	subtitle := "Generate a report for "
	if s.Label != "" {
		subtitle += s.Label
	} else {
		subtitle += s.Name
	}

	item = alfred.Item{
		Autocomplete: s.Name,
		Title:        s.Name,
		Subtitle:     subtitle,
		Arg: &alfred.ItemArg{
			Keyword: "report",
			Data:    alfred.Stringify(&cfg),
		},
	}

	if s.MultiDay {
		grouping := groupByDay
		cfg.Grouping = &grouping
		item.AddMod(alfred.ModAlt, alfred.ItemMod{
			Subtitle: item.Subtitle + ", grouping entries by day",
			Arg: &alfred.ItemArg{
				Keyword: "report",
				Data:    alfred.Stringify(&cfg),
			},
		})
	}

	return
}

func createReportItems(arg, data string, cfg *reportCfg, span span) (items []alfred.Item, err error) {
	projectID := -1
	if cfg.Project != nil {
		projectID = *cfg.Project
	}

	entryTitle := ""
	if cfg.EntryTitle != nil {
		entryTitle = *cfg.EntryTitle
	}

	var grouping reportGrouping
	if cfg.Grouping != nil {
		grouping = *cfg.Grouping
	}

	var report *summaryReport
	if report, err = generateReport(span.Start, span.End, projectID, entryTitle); err != nil {
		return
	}

	dlog.Printf("creating report with data %#v", data)

	newCfg := reportCfg{Span: &span, Previous: cfg}

	var total int64
	var totalName string

	spanName := span.Name
	if span.Label != "" {
		spanName = span.Label
	}

	if grouping == groupByDay {
		// By-day report

		dlog.Printf("checking %d dates", len(report.dates))
		for _, date := range report.dates {
			totalName = "for " + spanName
			if entryTitle != "" {
				totalName += " for " + entryTitle
			}

			dateName := date.name

			if alfred.FuzzyMatches(dateName, arg) {
				if span, e := getSpan(date.name); e == nil {
					newCfg.Span = &span
				} else {
					dlog.Printf("Error getting span for %s: %v", date.name, e)
				}

				items = append(items, alfred.Item{
					Title:    dateName,
					Subtitle: formatDuration(date.total),
					Arg: &alfred.ItemArg{
						Keyword: "report",
						Data:    alfred.Stringify(&newCfg),
					},
				})

				total += date.total
			}
		}
	} else {
		// By-project report

		dlog.Printf("checking %d projects", len(report.projects))

		for _, project := range report.projects {
			if projectID != -1 {
				// By-project report for a single project

				dlog.Printf("have projectID: %d", projectID)

				totalName = fmt.Sprintf("for %s for %s", spanName, project.name)

				grouping := groupByDay
				newCfg.Grouping = &grouping

				for desc, entry := range project.entries {
					dlog.Printf("getting info for %#v", entry)
					entryTitle := desc
					newCfg.EntryTitle = &entryTitle

					if alfred.FuzzyMatches(entryTitle, arg) {
						item := alfred.Item{
							Title:    entryTitle,
							Subtitle: formatDuration(entry.total),
							Arg: &alfred.ItemArg{
								Keyword: "report",
								Data:    alfred.Stringify(&newCfg),
							},
						}

						if entry.running {
							item.Icon = "running.png"
						}

						items = append(items, item)
					}

					total += entry.total
				}
			} else {
				// By-project report for all projects

				totalName = "for " + spanName
				if entryTitle != "" {
					totalName += " for " + entryTitle
				}

				projectName := project.name

				newCfg.Project = &project.id
				dlog.Printf("checking if '%s' fuzzyMatches '%s'", arg, projectName)

				if alfred.FuzzyMatches(projectName, arg) {
					item := alfred.Item{
						Title:    projectName,
						Subtitle: formatDuration(project.total),
						Arg: &alfred.ItemArg{
							Keyword: "report",
							Data:    alfred.Stringify(&newCfg),
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

	// Add the Total line at the top
	if totalName != "" && arg == "" {
		title := fmt.Sprintf("Total time %s: %s", totalName, formatDuration(total))
		item := alfred.Item{
			Title:    title,
			Subtitle: alfred.Line,
		}

		if newCfg.EntryTitle != nil {
			newCfg.EntryTitle = nil
		} else if newCfg.Project != nil {
			newCfg.Project = nil
		} else {
			newCfg.Span = nil
		}

		if cfg.Previous != nil {
			item.Arg = &alfred.ItemArg{
				Keyword: "report",
				Data:    alfred.Stringify(cfg.Previous),
			}
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
		s.Name = "week"
		s.Label = "this week"
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

func generateReport(since, until time.Time, projectID int, entryTitle string) (*summaryReport, error) {
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

			if entryTitle != "" && entry.Description != entryTitle {
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
				duration = round(time.Now().Sub(entry.StartTime()).Seconds())
				project.running = true
			}

			duration = roundDuration(duration, false)

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
