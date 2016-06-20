package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"strconv"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-toggl"
)

// ProjectCommand is a command for handling projects
type ProjectCommand struct{}

// Keyword returns the command's keyword
func (c ProjectCommand) Keyword() string {
	return "projects"
}

// IsEnabled returns true if the command is enabled
func (c ProjectCommand) IsEnabled() bool {
	return config.APIKey != ""
}

// MenuItem returns the command's menu item
func (c ProjectCommand) MenuItem() alfred.Item {
	return alfred.NewKeywordItem(c.Keyword(), "List your projects, add new ones")
}

// Items returns a list of filter items
func (c ProjectCommand) Items(args []string) (items []alfred.Item, err error) {
	if err = checkRefresh(); err != nil {
		return
	}

	var projectID int

	flags := flag.NewFlagSet("projectFlags", flag.ContinueOnError)
	flags.IntVar(&projectID, "project", -1, "Project ID")
	flags.Parse(args)

	query := flags.Arg(0)
	dlog.Printf("query: %s", query)

	runningTimer, isRunning := getRunningTimer()

	if projectID != -1 {
		// List menu for a project
		if project, ok := findProjectByID(projectID); ok {
			return projectItems(project, query)
		}
	} else {
		for _, entry := range cache.Account.Data.Projects {
			if alfred.FuzzyMatches(entry.Name, query) {
				item := alfred.Item{
					Title:        entry.Name,
					Subtitle:     "",
					Autocomplete: entry.Name,
					Arg:          fmt.Sprintf("%d", entry.Id),
				}

				if isRunning && runningTimer.Pid == entry.Id {
					item.Icon = "running.png"
				}

				items = append(items, item)
			}
		}

		if len(items) == 0 && query != "" {
			data := createProjectMessage{Name: query}
			dataString, _ := json.Marshal(data)

			items = append(items, alfred.Item{
				Title:    query,
				Subtitle: "New project",
				Arg:      "-create " + strconv.Quote(string(dataString)),
			})
		} else if query != "" {
			items = alfred.SortItemsForKeyword(items, query)
		}

		if len(items) == 0 {
			items = append(items, alfred.Item{Title: "No matching projects"})
		}
	}

	return items, nil
}

// Do runs the command
func (c ProjectCommand) Do(args []string) (out string, err error) {
	var defaultID int
	var toCreate string
	var toUpdate string

	flags := flag.NewFlagSet("projectFlags", flag.ContinueOnError)
	flags.IntVar(&defaultID, "default", -1, "Project ID to set as default")
	flags.StringVar(&toCreate, "create", "", "New project data")
	flags.StringVar(&toUpdate, "update", "", "Updated project data")
	flags.Parse(args)

	if defaultID != -1 {
		dlog.Printf("setting default project to %v", defaultID)
		config.DefaultProjectID = defaultID
		alfred.SaveJSON(configFile, &config)
	}

	if toCreate != "" {
		dlog.Printf("creating project %v", toCreate)
		var project toggl.Project
		if project, err = createProject(toCreate); err != nil {
			return
		}
		return fmt.Sprintf(`Created project "%s"`, project.Name), nil
	}

	if toUpdate != "" {
		dlog.Printf("updating project %v", toUpdate)
		var project toggl.Project
		if project, err = updateProject(toUpdate); err != nil {
			return
		}
		return fmt.Sprintf(`Updated project "%s"`, project.Name), nil
	}

	return "Unrecognized input", nil
}

// support -------------------------------------------------------------------

type createProjectMessage struct {
	Name string
	WID  int
}

func createProject(dataString string) (project toggl.Project, err error) {
	var message createProjectMessage
	if err = json.Unmarshal([]byte(dataString), &message); err != nil {
		return
	}

	session := toggl.OpenSession(config.APIKey)

	if message.WID == 0 {
		message.WID = cache.Account.Data.Workspaces[0].Id
	}

	if project, err = session.CreateProject(message.Name, message.WID); err == nil {
		dlog.Printf("Got project: %#v\n", project)
		cache.Account.Data.Projects = append(cache.Account.Data.Projects, project)
		if err := alfred.SaveJSON(cacheFile, &cache); err != nil {
			dlog.Printf("Error saving cache: %s\n", err)
		}
	}

	return
}

func updateProject(dataString string) (project toggl.Project, err error) {
	if err = json.Unmarshal([]byte(dataString), &project); err != nil {
		return
	}

	session := toggl.OpenSession(config.APIKey)

	if project, err = session.UpdateProject(project); err != nil {
		return
	}

	adata := &cache.Account.Data

	for i, p := range adata.Projects {
		if p.Id == project.Id {
			adata.Projects[i] = project
			if err := alfred.SaveJSON(cacheFile, &cache); err != nil {
				dlog.Printf("Error saving cache: %v\n", err)
			}
			break
		}
	}

	return
}

func projectItems(project toggl.Project, query string) (items []alfred.Item, err error) {
	if alfred.PartiallyPrefixes("name:", query) {
		item := alfred.Item{}
		parts := alfred.CleanSplitN(query, " ", 2)

		if len(parts) > 1 {
			newName := parts[1]
			updateEntry := project
			updateEntry.Name = newName
			dataString, _ := json.Marshal(updateEntry)
			item.Title = fmt.Sprintf("Change name to '%s'", newName)
			item.Subtitle = "Name: " + project.Name
			item.Arg = "-update " + strconv.Quote(string(dataString))
		} else {
			item.Title = "Name: " + project.Name
			item.Autocomplete = "Name: "
			item.Invalid = true
		}

		dlog.Printf("name item: %#v", item)

		items = append(items, item)
	}

	if alfred.PartiallyPrefixes("Make default", query) {
		items = append(items, alfred.Item{
			Title:        "Make default",
			Subtitle:     "Make this the default project",
			Autocomplete: "Make default",
			Arg:          fmt.Sprintf("-default=%d", project.Id),
		})
	}

	if alfred.PartiallyPrefixes("timers", query) {
		items = append(items, alfred.Item{
			Title:        "Timers...",
			Subtitle:     "List associated time entries",
			Autocomplete: "Timers...",
			Arg:          "-timers",
		})
	}

	return
}
