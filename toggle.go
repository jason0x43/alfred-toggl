package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-toggl"
)

// ToggleAction toggles a time entry's running state.
type ToggleAction struct{}

// Keyword return's the action's keyword.
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
	running, isRunning := getRunningTimer()

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
				updatedEntry, err = session.ContinueTimeEntry(*entry, config.DurationOnly)
				log.Printf("Updated entry: %v", updatedEntry)
				if updatedEntry.Id == entry.Id {
					adata.TimeEntries[i] = updatedEntry
				} else {
					adata.TimeEntries = append(adata.TimeEntries, updatedEntry)
				}
			}

			if err != nil {
				return "", err
			}

			if isRunning && running.Id != updatedEntry.Id {
				// If a different timer was previously running, refresh everything
				err = refresh()
			} else {
				err = alfred.SaveJson(cacheFile, &cache)
			}

			if err != nil {
				log.Printf("Error saving cache: %v\n", err)
			}

			return operation + "ed " + entry.Description, nil
		}
	}

	return "", fmt.Errorf("Invalid time entry ID %d", id)
}
