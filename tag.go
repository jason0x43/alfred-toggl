package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-toggl"
)

// tag filter --------------------------------------------

// TagCommand is a command
type TagCommand struct{}

// About returns information about this command
func (c TagCommand) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     "tags",
		Description: "List your tags",
		IsEnabled:   config.APIKey != "",
	}
}

// Items returns a list of filter items
func (c TagCommand) Items(arg, data string) (items []alfred.Item, err error) {
	dlog.Printf("getting tag items")
	if err = checkRefresh(); err != nil {
		return
	}

	var cfg tagCfg

	if data != "" {
		if err = json.Unmarshal([]byte(data), &cfg); err != nil {
			dlog.Printf("Error unmarshalling tag config: %v", err)
		}
	}

	tid := -1
	if cfg.Tag != nil {
		tid = *cfg.Tag
	}

	if tid != -1 {
		// List menu for a project
		if tag, _, ok := getTagByID(tid); ok {
			return tagItems(tag, arg)
		}
	} else {
		tagCfg := tagCfg{}

		for _, entry := range cache.Account.Tags {
			if alfred.FuzzyMatches(entry.Name, arg) {
				tagCfg.Tag = &entry.ID

				items = append(items, alfred.Item{
					UID:          fmt.Sprintf("%s.tag.%d", workflow.BundleID(), entry.ID),
					Title:        entry.Name,
					Autocomplete: entry.Name,
					Arg: &alfred.ItemArg{
						Keyword: "tags",
						Data:    alfred.Stringify(tagCfg),
					},
				})
			}
		}

		if len(items) == 0 && arg != "" {
			tagCfg.ToCreate = &createTagMessage{Name: arg}

			items = append(items, alfred.Item{
				Title:    arg,
				Subtitle: "New tag",
				Arg: &alfred.ItemArg{
					Keyword: "tags",
					Mode:    alfred.ModeDo,
					Data:    alfred.Stringify(tagCfg),
				},
			})
		}

		if len(items) == 0 {
			items = append(items, alfred.Item{Title: "No matching tags"})
		}
	}

	return items, nil
}

// Do runs the command
func (c TagCommand) Do(data string) (out string, err error) {
	var cfg tagCfg

	if data != "" {
		if err := json.Unmarshal([]byte(data), &cfg); err != nil {
			dlog.Printf("Error unmarshaling tag data: %v", err)
		}
	}

	session := toggl.OpenSession(config.APIKey)

	if cfg.ToUpdate != nil {
		if _, err = session.UpdateTag(*cfg.ToUpdate); err != nil {
			out += "Updated tag"
		}
	}

	if cfg.ToCreate != nil {
		var tag toggl.Tag
		if cfg.ToCreate.WID == 0 {
			cfg.ToCreate.WID = cache.Account.Workspaces[0].ID
		}
		if tag, err = session.CreateTag(cfg.ToCreate.Name, cfg.ToCreate.WID); err == nil {
			cache.Account.Tags = append(cache.Account.Tags, tag)
			if err := alfred.SaveJSON(cacheFile, &cache); err != nil {
				log.Printf("Error saving cache: %s\n", err)
			}
		}

		if out != "" {
			out += ", created tag"
		} else {
			out = "Created tag"
		}
	}

	if cfg.ToDelete != nil {
		var ok bool
		var tag toggl.Tag
		var index int
		var id = *cfg.ToDelete
		if tag, index, ok = getTagByID(id); !ok {
			err = fmt.Errorf(`Tag %d does not exist`, id)
			return
		}

		if _, err = session.DeleteTag(tag); err == nil {
			adata := &cache.Account
			if index < len(adata.Tags)-1 {
				adata.Tags = append(adata.Tags[:index], adata.Tags[index+1:]...)
			} else {
				adata.Tags = adata.Tags[:index]
			}
			if err := alfred.SaveJSON(cacheFile, &cache); err != nil {
				dlog.Printf("Error saving cache: %s\n", err)
			}
		}

		if out != "" {
			out += ", deleted tag"
		} else {
			out = "Deleted tag"
		}
	}

	// Since tags are referenced by name, updating one can cause changes in
	// multiple time entries. The simplest way to handle that is just to
	// refresh everything.
	refresh()

	return
}

// support -------------------------------------------------------------------

type tagCfg struct {
	Tag      *int
	ToCreate *createTagMessage
	ToUpdate *toggl.Tag
	ToDelete *int
}

type createTagMessage struct {
	Name string
	WID  int
}

func tagItems(tag toggl.Tag, arg string) (items []alfred.Item, err error) {
	if alfred.FuzzyMatches("timers", arg) {
		items = append(items, alfred.Item{
			Title:        "Time entries...",
			Subtitle:     "List associated time entries",
			Autocomplete: "Time entries...",
			Arg: &alfred.ItemArg{
				Keyword: "timers",
				Data:    alfred.Stringify(timerCfg{Tag: &tag.ID}),
			},
		})
	}

	if alfred.FuzzyMatches("name", arg) {
		item := alfred.Item{}
		_, name := alfred.SplitCmd(arg)

		if name != "" {
			updateEntry := tag
			updateEntry.Name = name
			item.Title = fmt.Sprintf("Change name to '%s'", name)
			item.Subtitle = "Name: " + tag.Name
			item.Arg = &alfred.ItemArg{
				Keyword: "tags",
				Mode:    alfred.ModeDo,
				Data:    alfred.Stringify(tagCfg{ToUpdate: &updateEntry}),
			}

		} else {
			item.Title = "Name: " + tag.Name
			item.Autocomplete = "Name: "
		}

		items = append(items, item)
	}

	if alfred.FuzzyMatches("delete", arg) {
		items = append(items, alfred.Item{
			Title:        "Delete",
			Autocomplete: "Delete",
			Arg: &alfred.ItemArg{
				Keyword: "tags",
				Mode:    alfred.ModeDo,
				Data:    alfred.Stringify(tagCfg{ToDelete: &tag.ID}),
			},
		})
	}

	return
}
