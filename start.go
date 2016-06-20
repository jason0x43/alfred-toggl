package main

import (
	"encoding/json"
	"log"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-toggl"
)

type startMessage struct {
	Description string
	Pid         int
}

// StartAction is a command
type StartAction struct{}

// Keyword returns the command's keyword
func (c StartAction) Keyword() string {
	return "start"
}

// IsEnabled returns true if the command is enabled
func (c StartAction) IsEnabled() bool {
	return config.APIKey != ""
}

// Do runs the command
func (c StartAction) Do(args []string) (string, error) {
	var query string
	if len(args) > 0 {
		query = args[0]
	}

	log.Printf("start '%s'", query)

	var data startMessage
	err := json.Unmarshal([]byte(query), &data)
	if err != nil {
		return "", err
	}

	session := toggl.OpenSession(config.APIKey)
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
		err := alfred.SaveJSON(cacheFile, &cache)
		if err != nil {
			log.Printf("Error saving cache: %s\n", err)
		}
	}

	return "Created time entry", err
}
