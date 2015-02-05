package main

import (
	"log"
	"strings"
	"time"

	"github.com/jason0x43/go-alfred"
)

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
	parts := alfred.TrimAllLeft(strings.Split(query, alfred.Separator))

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
			Title: "No time entries",
			Valid: alfred.Invalid,
		})
	}

	return items, nil
}
