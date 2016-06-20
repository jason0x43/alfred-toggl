package main

import (
	"flag"
	"fmt"
	"log"
	"regexp"
	"sort"
	"time"

	"github.com/jason0x43/go-alfred"
)

// ReportFilter is a command
type ReportFilter struct{}

// Keyword returns the command's keyword
func (c ReportFilter) Keyword() string {
	return "report"
}

// IsEnabled returns true if the command is enabled
func (c ReportFilter) IsEnabled() bool {
	return config.APIKey != ""
}

// MenuItem returns the command's menu item
func (c ReportFilter) MenuItem() alfred.Item {
	return alfred.NewKeywordItem(c.Keyword(), "Generate summary reports")
}

// Items returns a list of filter items
func (c ReportFilter) Items(args []string) (items []alfred.Item, err error) {
	if err = checkRefresh(); err != nil {
		return
	}

	var projectID int
	var span string
	var query string

	flags := flag.NewFlagSet("reportFlags", flag.ContinueOnError)
	flags.IntVar(&projectID, "project", -1, "Project ID")
	flags.StringVar(&span, "span", "", "Report date or span")
	flags.Parse(args)

	if span == "" {
		span = flags.Arg(0)
	} else {
		query = flags.Arg(0)
	}

	log.Printf("tell report with span '%s'", span)

	var since time.Time
	var until time.Time

	if span == "today" {
		since = toDayStart(time.Now())
		until = toDayEnd(since)
	} else if span == "yesterday" {
		since = toDayStart(time.Now().AddDate(0, 0, -1))
		until = toDayEnd(since)
	} else if span == "week" {
		start := time.Now()
		if start.Weekday() >= 1 {
			delta := 1 - int(start.Weekday())
			since = toDayStart(start.AddDate(0, 0, delta))
			until = toDayEnd(time.Now())
		}
	} else if layout := getDateLayout(span); layout != "" {
		if since, err = time.Parse(layout, span); err != nil {
			return
		}
		year := since.Year()
		if year == 0 {
			year = time.Now().Year()
		}
		since = time.Date(year, since.Month(), since.Day(), 0, 0, 0, 0, time.Local)
		until = time.Date(year, since.Month(), since.Day(), 23, 59, 59, 999999999, time.Local)
	} else {
		for _, value := range []string{"today", "yesterday", "week"} {
			if alfred.PartiallyPrefixes(value, span) {
				subtitle := "Generate a report for "
				if value == "week" {
					subtitle += "this "
				}
				items = append(items, alfred.Item{
					Invalid:      true,
					Autocomplete: value,
					Title:        value,
					Subtitle:     subtitle + value,
				})
			}
		}

		if len(items) == 0 {
			items = append(items, alfred.Item{
				Invalid: true,
				Title:   "Enter a valid date or range",
			})
		}

		return items, nil
	}

	if !since.IsZero() && !until.IsZero() {
		reportItems, err := createReportItems(query, span, since, until, projectID)
		if err != nil {
			return items, err
		}
		items = append(items, reportItems...)
	}

	if len(items) == 0 {
		items = append(items, alfred.Item{
			Title:   "No time entries",
			Invalid: true,
		})
	}

	return items, nil
}

// support -------------------------------------------------------------------

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
}

func createReportItems(query, span string, since, until time.Time, projectID int) (items []alfred.Item, err error) {
	var report *summaryReport
	if report, err = generateReport(since, until); err != nil {
		return
	}

	log.Printf("got report with %d projects\n", len(report.projects))

	var total int64
	var totalName string

	for _, project := range report.projects {
		if projectID != -1 {
			if project.id != projectID {
				continue
			}
			totalName = fmt.Sprintf("for %s over %s", project.name, span)
			for desc, entry := range project.entries {
				dlog.Printf("getting info for %#v", entry)
				entryTitle := desc
				if alfred.FuzzyMatches(entryTitle, query) {
					item := alfred.Item{
						Title:    entryTitle,
						Subtitle: fmt.Sprintf("%.2f", float32(entry.total)/100.0),
						Invalid:  true,
					}

					if entry.running {
						item.Icon = "running.png"
					}

					items = append(items, item)
				}
			}
		} else {
			totalName = "for " + span
			entryTitle := project.name
			if alfred.FuzzyMatches(entryTitle, query) {
				item := alfred.Item{
					Title:    entryTitle,
					Subtitle: fmt.Sprintf("%.2f", float32(project.total)/100.0),
					Arg:      fmt.Sprintf(`-project=%d -span="%s"`, project.id, span),
				}

				if project.running {
					item.Icon = "running.png"
				}

				items = append(items, item)
				total += project.total
			}
		}
	}

	sort.Sort(alfred.ByTitle(items))

	if totalName != "" {
		item := alfred.Item{
			Title:    fmt.Sprintf("Total hours %s: %.2f", totalName, float32(total)/100.0),
			Invalid:  true,
			Subtitle: alfred.Line,
		}
		items = alfred.InsertItem(items, item, 0)
	}

	return
}

func generateReport(since, until time.Time) (*summaryReport, error) {
	log.Printf("Generating report from %s to %s", since, until)

	report := summaryReport{projects: map[string]*projectEntry{}}
	projects := getProjectsByID()

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
				duration = int64(time.Now().Sub(entry.StartTime()).Seconds())
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

var dateFormats = map[string]*regexp.Regexp{
	"1/2":      regexp.MustCompile(`^\d\d?\/\d\d?$`),
	"1/2/06":   regexp.MustCompile(`^\d\d?\/\d\d?\/\d\d$`),
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
