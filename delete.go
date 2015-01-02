package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/jason0x43/go-toggl"
)

type deleteMessage struct {
	Type string
	Id   int
}

type DeleteAction struct{}

func (c DeleteAction) Keyword() string {
	return "delete"
}

func (c DeleteAction) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c DeleteAction) Do(query string) (string, error) {
	log.Printf("delete %s", query)

	var message deleteMessage
	err := json.Unmarshal([]byte(query), &message)
	if err != nil {
		return "", err
	}

	session := toggl.OpenSession(config.ApiKey)
	accountData := &cache.Account.Data
	var result struct {
		found   bool
		name    string
		deleted bool
	}

	switch message.Type {
	case "time entry":
		for i, entry := range accountData.TimeEntries {
			if entry.Id == message.Id {
				result.found = true
				result.name = entry.Description
				prompt := fmt.Sprintf("Are you sure you want to delete timer '%s'?", entry.Description)
				yes, _ := workflow.GetConfirmation(prompt, false)

				if yes {
					log.Printf("Deleting '" + entry.Description + "'")
					_, err := session.DeleteTimeEntry(entry)
					if err != nil {
						return "", err
					}

					if i < len(accountData.TimeEntries)-1 {
						accountData.TimeEntries = append(accountData.TimeEntries[:i], accountData.TimeEntries[i+1:]...)
					} else {
						accountData.TimeEntries = accountData.TimeEntries[:i]
					}

					result.deleted = true
				}
			}
		}
	case "project":
		for i, project := range accountData.Projects {
			if project.Id == message.Id {
				result.found = true
				result.name = project.Name
				prompt := fmt.Sprintf("Are you sure you want to delete project '%s'?", project.Name)
				yes, _ := workflow.GetConfirmation(prompt, false)

				if yes {
					log.Printf("Deleting\n")
					_, err := session.DeleteProject(project)
					if err != nil {
						return "", err
					}

					if i < len(accountData.Projects)-1 {
						accountData.Projects = append(accountData.Projects[:i], accountData.Projects[i+1:]...)
					} else {
						accountData.Projects = accountData.Projects[:i]
					}

					result.deleted = true
				}
			}
		}
	case "tag":
		for i, tag := range accountData.Tags {
			if tag.Id == message.Id {
				result.found = true
				result.name = tag.Name
				prompt := fmt.Sprintf("Are you sure you want to delete tag '%s'?", tag.Name)
				yes, _ := workflow.GetConfirmation(prompt, false)

				if yes {
					log.Printf("Deleting\n")
					_, err := session.DeleteTag(tag)
					if err != nil {
						return "", err
					}

					if i < len(accountData.Tags)-1 {
						accountData.Tags = append(accountData.Tags[:i], accountData.Tags[i+1:]...)
					} else {
						accountData.Tags = accountData.Tags[:i]
					}

					result.deleted = true
				}
			}
		}
	}

	if result.found {
		if result.deleted {
			refresh()
		}

		return "Deleted " + result.name, nil
	}

	return "", fmt.Errorf("Invalid %s ID %d", message.Type, message.Id)
}
