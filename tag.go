package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-toggl"
)

// tag filter --------------------------------------------

type TagFilter struct{}

func (c TagFilter) Keyword() string {
	return "tags"
}

func (c TagFilter) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c TagFilter) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword() + " ",
		Valid:        alfred.Invalid,
		SubtitleAll:  "List your tags",
	}
}

func (c TagFilter) Items(prefix, query string) ([]alfred.Item, error) {
	var items []alfred.Item
	if err := checkRefresh(); err != nil {
		return items, err
	}

	parts := alfred.SplitAndTrimQuery(query)
	log.Printf("parts: %d", len(parts))

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
					Title: "No time entries",
					Valid: alfred.Invalid,
				})
			} else {
				for _, entry := range entries {
					items = append(items, alfred.Item{
						Title: entry.Description,
						Valid: alfred.Invalid,
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
					Valid:        alfred.Invalid,
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
					Valid:        alfred.Invalid,
				})
			}
			if alfred.FuzzyMatches("delete", subcommand) {
				data := deleteMessage{Type: "tag", Id: tag.Id}
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
					Valid:        alfred.Invalid,
				})
			}
		}

		if len(items) == 0 {
			data := createTagMessage{Name: parts[0]}
			dataString, _ := json.Marshal(data)

			items = append(items, alfred.Item{
				Title:       parts[0],
				SubtitleAll: "New tag",
				Arg:         "create-tag " + string(dataString),
			})
		}
	}

	return items, nil
}

// update-tag --------------------------------------------

type UpdateTagAction struct{}

func (c UpdateTagAction) Keyword() string {
	return "update-tag"
}

func (c UpdateTagAction) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c UpdateTagAction) Do(query string) (string, error) {
	log.Printf("update-tag %s", query)

	var tag toggl.Tag
	err := json.Unmarshal([]byte(query), &tag)
	if err != nil {
		return "", fmt.Errorf("Invalid time entry %v", query)
	}

	session := toggl.OpenSession(config.ApiKey)

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

type CreateTagAction struct{}

func (c CreateTagAction) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c CreateTagAction) Keyword() string {
	return "create-tag"
}

func (c CreateTagAction) Do(query string) (string, error) {
	log.Printf("create-tag '%s'", query)

	var data createTagMessage
	err := json.Unmarshal([]byte(query), &data)
	if err != nil {
		return "", err
	}

	session := toggl.OpenSession(config.ApiKey)
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
		err := alfred.SaveJson(cacheFile, &cache)
		if err != nil {
			log.Printf("Error saving cache: %s\n", err)
		}
	}

	return fmt.Sprintf("Created tag '%s'", data.Name), err
}

// tag action --------------------------------------------

type tagMessage struct {
	Tag       string
	TimeEntry int
	Add       bool
}

type TagAction struct{}

func (c TagAction) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c TagAction) Keyword() string {
	return "tag"
}

func (c TagAction) Do(query string) (string, error) {
	log.Printf("tag '%s'", query)

	var data tagMessage
	err := json.Unmarshal([]byte(query), &data)
	if err != nil {
		return "", err
	}

	session := toggl.OpenSession(config.ApiKey)
	if err != nil {
		return "", err
	}

	entry, err := session.AddRemoveTag(data.TimeEntry, data.Tag, data.Add)
	if err != nil {
		return "", err
	}

	adata := &cache.Account.Data

	for i, e := range adata.TimeEntries {
		if e.Id == entry.Id {
			adata.TimeEntries[i] = entry
			err = alfred.SaveJson(cacheFile, &cache)
			if err != nil {
				log.Printf("Error saving cache: %v\n", err)
			}
			break
		}
	}

	return fmt.Sprintf("Updated tag '%s' for %d", data.Tag, data.TimeEntry), err
}
