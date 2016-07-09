package main

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-toggl"
)

// tag filter --------------------------------------------

// TagCommand is a command
type TagCommand struct{}

// IsEnabled returns true if the command is enabled
func (c TagCommand) IsEnabled() bool {
	return config.APIKey != ""
}

// About returns information about this command
func (c TagCommand) About() *alfred.CommandDef {
	return &alfred.CommandDef{
		Keyword:     "tag",
		Description: "List your tags",
		WithSpace:   true,
	}
}

// Items returns a list of filter items
func (c TagCommand) Items(arg, data string) (items []*alfred.Item, err error) {
	dlog.Printf("getting tag items")
	if err = checkRefresh(); err != nil {
		return
	}

	parts := alfred.TrimAllLeft(strings.Split(arg, " "))
	log.Printf("parts: %d", len(parts))

	if len(parts) > 1 {
		tag, found := findTagByName(parts[0])
		if !found {
			items = append(items, &alfred.Item{Title: "Invalid tag '" + parts[0] + "'"})
		}

		subcommand := parts[1]

		switch subcommand {
		case "timers":
			// list time entries with this tag
			entries := getLatestTimeEntriesForTag(tag.Name)
			if len(entries) == 0 {
				items = append(items, &alfred.Item{
					Title: "No time entries",
				})
			} else {
				for _, entry := range entries {
					items = append(items, &alfred.Item{
						Title: entry.Description,
					})
				}
			}

		case "name":
			name := tag.Name
			// arg := ""
			if len(parts) > 2 && parts[2] != "" {
				name = parts[2]
				updateTag := tag
				updateTag.Name = name
				// dataString, _ := json.Marshal(updateTag)
				// arg = "update-tag " + string(dataString)
			}
			items = append(items, &alfred.Item{
				Title: "name: " + name,
				// Arg:   arg,
			})
		default:
			if alfred.FuzzyMatches("name", subcommand) {
				items = append(items, &alfred.Item{
					Title:        "name: " + tag.Name,
					Autocomplete: "name" + " ",
				})
			}
			if alfred.FuzzyMatches("timers", subcommand) {
				title := "timers: "
				if tagHasTimeEntries(tag.Name) {
					title += "..."
				} else {
					title += "<None>"
				}
				items = append(items, &alfred.Item{
					Title:        title,
					Autocomplete: "timers",
				})
			}
			if alfred.FuzzyMatches("delete", subcommand) {
				// data := deleteMessage{Type: "tag", ID: tag.ID}
				// dataString, _ := json.Marshal(data)

				items = append(items, &alfred.Item{
					Title:        "delete",
					Autocomplete: "delete",
					// Arg:          "delete " + string(dataString),
				})
			}
		}
	} else {
		for _, entry := range cache.Account.Data.Tags {
			if alfred.FuzzyMatches(entry.Name, arg) {
				items = append(items, &alfred.Item{
					Title:        entry.Name,
					Autocomplete: entry.Name + " ",
				})
			}
		}

		if len(items) == 0 {
			// data := createTagMessage{Name: parts[0]}
			// dataString, _ := json.Marshal(data)

			items = append(items, &alfred.Item{
				Title:    parts[0],
				Subtitle: "New tag",
				// Arg:      "create-tag " + string(dataString),
			})
		}
	}

	return items, nil
}

// Do runs the command
func (c TagCommand) Do(arg, data string) (out string, err error) {
	var cfg tagCfg

	if data != "" {
		if err := json.Unmarshal([]byte(data), &cfg); err != nil {
			dlog.Printf("Error unmarshaling tag data: %v", err)
		}
	}

	session := toggl.OpenSession(config.APIKey)

	if cfg.ToUpdate != nil {
		if _, err = session.UpdateTag(*cfg.ToUpdate); err != nil {
			return
		}
	}

	if cfg.ToCreate != nil {
		var tag toggl.Tag
		if cfg.ToCreate.WID == 0 {
			cfg.ToCreate.WID = cache.Account.Data.Workspaces[0].ID
		}
		if tag, err = session.CreateTag(cfg.ToCreate.Name, cfg.ToCreate.WID); err == nil {
			cache.Account.Data.Tags = append(cache.Account.Data.Tags, tag)
			if err := alfred.SaveJSON(cacheFile, &cache); err != nil {
				log.Printf("Error saving cache: %s\n", err)
			}
		} else {
			return
		}
	}

	// Since tags are referenced by name, updating one can cause changes in
	// multiple time entries. The simplest way to handle that is just to
	// refresh everything.
	refresh()

	return "Updated tag", nil
}

// support -------------------------------------------------------------------

type tagCfg struct {
	ToCreate *struct {
		Name string
		WID  int
	}
	ToUpdate *toggl.Tag
}
