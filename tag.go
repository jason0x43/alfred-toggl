package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-toggl"
)

// tag filter --------------------------------------------

// TagFilter is a command
type TagFilter struct{}

// Keyword returns the command's keyword
func (c TagFilter) Keyword() string {
	return "tags"
}

// IsEnabled returns true if the command is enabled
func (c TagFilter) IsEnabled() bool {
	return config.APIKey != ""
}

// MenuItem returns the command's menu item
func (c TagFilter) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword() + " ",
		Invalid:      true,
		Subtitle:     "List your tags",
	}
}

// Items returns a list of filter items
func (c TagFilter) Items(args []string) ([]alfred.Item, error) {
	var items []alfred.Item
	if err := checkRefresh(); err != nil {
		return items, err
	}

	var query string
	if len(args) > 0 {
		query = args[0]
	}

	parts := alfred.TrimAllLeft(strings.Split(query, alfred.Separator))
	log.Printf("parts: %d", len(parts))

	// TODO: get rid of this
	prefix := ""

	if len(parts) > 1 {
		tag, found := findTagByName(parts[0])
		if !found {
			items = append(items, alfred.Item{Title: "Invalid tag '" + parts[0] + "'"})
		}

		prefix += tag.Name + alfred.Separator
		subcommand := parts[1]

		switch subcommand {
		case "timers":
			// list time entries with this tag
			entries := getLatestTimeEntriesForTag(tag.Name)
			if len(entries) == 0 {
				items = append(items, alfred.Item{
					Title:   "No time entries",
					Invalid: true,
				})
			} else {
				for _, entry := range entries {
					items = append(items, alfred.Item{
						Title:   entry.Description,
						Invalid: true,
					})
				}
			}

		case "name":
			name := tag.Name
			arg := ""
			if len(parts) > 2 && parts[2] != "" {
				name = parts[2]
				updateTag := tag
				updateTag.Name = name
				dataString, _ := json.Marshal(updateTag)
				arg = "update-tag " + string(dataString)
			}
			items = append(items, alfred.Item{
				Title: "name: " + name,
				Arg:   arg,
			})
		default:
			if alfred.FuzzyMatches("name", subcommand) {
				items = append(items, alfred.Item{
					Title:        "name: " + tag.Name,
					Autocomplete: prefix + " name" + alfred.Separator + " ",
					Invalid:      true,
				})
			}
			if alfred.FuzzyMatches("timers", subcommand) {
				title := "timers: "
				if tagHasTimeEntries(tag.Name) {
					title += "..."
				} else {
					title += "<None>"
				}
				items = append(items, alfred.Item{
					Title:        title,
					Autocomplete: prefix + " timers",
					Invalid:      true,
				})
			}
			if alfred.FuzzyMatches("delete", subcommand) {
				data := deleteMessage{Type: "tag", ID: tag.Id}
				dataString, _ := json.Marshal(data)

				items = append(items, alfred.Item{
					Title:        "delete",
					Autocomplete: prefix + " delete",
					Arg:          "delete " + string(dataString),
				})
			}
		}
	} else {
		for _, entry := range cache.Account.Data.Tags {
			if alfred.FuzzyMatches(entry.Name, query) {
				items = append(items, alfred.Item{
					Title:        entry.Name,
					Autocomplete: prefix + entry.Name + alfred.Separator + " ",
					Invalid:      true,
				})
			}
		}

		if len(items) == 0 {
			data := createTagMessage{Name: parts[0]}
			dataString, _ := json.Marshal(data)

			items = append(items, alfred.Item{
				Title:    parts[0],
				Subtitle: "New tag",
				Arg:      "create-tag " + string(dataString),
			})
		}
	}

	return items, nil
}

// update-tag --------------------------------------------

// UpdateTagAction is a command
type UpdateTagAction struct{}

// Keyword returns the command's keyword
func (c UpdateTagAction) Keyword() string {
	return "update-tag"
}

// IsEnabled returns true if the command is enabled
func (c UpdateTagAction) IsEnabled() bool {
	return config.APIKey != ""
}

// Do runs the command
func (c UpdateTagAction) Do(args []string) (string, error) {
	var query string
	if len(args) > 0 {
		query = args[0]
	}

	log.Printf("update-tag %s", query)

	var tag toggl.Tag
	err := json.Unmarshal([]byte(query), &tag)
	if err != nil {
		return "", fmt.Errorf("Invalid time entry %v", query)
	}

	session := toggl.OpenSession(config.APIKey)

	tag, err = session.UpdateTag(tag)
	if err != nil {
		return "", err
	}

	// Since tags are referenced by name, updating one can cause changes in
	// multiple time entries. The simplest way to handle that is just to
	// refresh everything.
	refresh()

	return fmt.Sprintf("Updated '%s'", tag.Name), nil
}

// create-tag --------------------------------------------

type createTagMessage struct {
	Name string
	Wid  int
}

// CreateTagAction is a command
type CreateTagAction struct{}

// Keyword returns the command's keyword
func (c CreateTagAction) Keyword() string {
	return "create-tag"
}

// IsEnabled returns true if the command is enabled
func (c CreateTagAction) IsEnabled() bool {
	return config.APIKey != ""
}

// Do runs the command
func (c CreateTagAction) Do(query string) (string, error) {
	log.Printf("create-tag '%s'", query)

	var data createTagMessage
	err := json.Unmarshal([]byte(query), &data)
	if err != nil {
		return "", err
	}

	session := toggl.OpenSession(config.APIKey)
	if err != nil {
		return "", err
	}

	var tag toggl.Tag
	if data.Wid == 0 {
		data.Wid = cache.Account.Data.Workspaces[0].Id
	}
	tag, err = session.CreateTag(data.Name, data.Wid)

	if err == nil {
		log.Printf("Got tag: %#v\n", tag)
		cache.Account.Data.Tags = append(cache.Account.Data.Tags, tag)
		err := alfred.SaveJSON(cacheFile, &cache)
		if err != nil {
			log.Printf("Error saving cache: %s\n", err)
		}
	}

	return fmt.Sprintf("Created tag '%s'", data.Name), err
}
