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
