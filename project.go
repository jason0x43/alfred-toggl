package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-toggl"
)

// project -----------------------------------------------

type ProjectFilter struct{}

func (c ProjectFilter) Keyword() string {
	return "projects"
}

func (c ProjectFilter) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c ProjectFilter) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword() + " ",
		Valid:        alfred.Invalid,
		SubtitleAll:  "List your projects, add new ones",
	}
}

func (c ProjectFilter) Items(prefix, query string) ([]alfred.Item, error) {
	var items []alfred.Item
	if err := checkRefresh(); err != nil {
		return items, err
	}

	parts := alfred.TrimAllLeft(strings.Split(query, alfred.Separator))
	log.Printf("parts: %s", parts)

	projectName := parts[0]
	project, _ := findProjectByName(projectName)

	if len(parts) > 1 {
		subcommand := parts[1]
		log.Printf("subcommand: " + subcommand)

		switch subcommand {
		case "timers":
			// list time entries and let user start a new one
			var timerName string
			if len(parts) > 2 {
				timerName = parts[2]
			}

			prefix += projectName + alfred.Separator + " entries" + alfred.Separator
			entries := getLatestTimeEntriesForProject(project.Id)

			addItem := func(title, subtitle, autocomplete string) {
				data := startMessage{
					Pid:         project.Id,
					Description: title,
				}

				dataString, _ := json.Marshal(data)

				item := alfred.Item{
					Title:        title,
					SubtitleAll:  subtitle,
					Autocomplete: autocomplete,
					Arg:          "start " + string(dataString),
				}
				log.Println("arg:", item.Arg)
				items = append(items, item)
			}

			for _, entry := range entries {
				if alfred.FuzzyMatches(entry.Description, timerName) {
					addItem(entry.Description, "", prefix+" "+entry.Description)
				}
			}

			if len(items) == 0 && timerName != "" {
				addItem(timerName, "New timer for "+projectName, "")
			}

			if len(items) == 0 {
				if timerName != "" {
					addItem(timerName, "New timer for "+projectName, "")
				} else {
					items = append(items, alfred.Item{Title: "No time entries"})
				}
			}

		case "name":
			name := projectName
			arg := ""
			if len(parts) > 2 && parts[2] != "" {
				name = parts[2]
				updateProject := project
				updateProject.Name = name
				dataString, _ := json.Marshal(updateProject)
				arg = "update-project " + string(dataString)
			}
			items = append(items, alfred.Item{
				Title: "name: " + name,
				Arg:   arg,
			})

		default:
			if alfred.FuzzyMatches("name", subcommand) {
				items = append(items, alfred.Item{
					Title:        "name: " + project.Name,
					Autocomplete: prefix + project.Name + alfred.Separator + " name" + alfred.Separator + " ",
					Valid:        alfred.Invalid,
				})
			}
			if alfred.FuzzyMatches("timers", subcommand) {
				title := "timers: "
				if projectHasTimeEntries(project.Id) {
					title += "..."
				} else {
					title += "<None>"
				}
				items = append(items, alfred.Item{
					Title:        title,
					Autocomplete: prefix + project.Name + alfred.Separator + " timers" + alfred.Separator + " ",
					Valid:        alfred.Invalid,
				})
			}
			if alfred.FuzzyMatches("delete", subcommand) {
				data := deleteMessage{Type: "project", Id: project.Id}
				dataString, _ := json.Marshal(data)

				items = append(items, alfred.Item{
					Title:        "delete",
					Arg:          "delete " + string(dataString),
					Autocomplete: prefix + project.Name + alfred.Separator + " delete",
				})
			}

			if len(items) == 0 {
				items = append(items, alfred.Item{
					Title: fmt.Sprintf("Unknown keyword '%s'", subcommand),
					Valid: alfred.Invalid,
				})
			}
		}
	} else {
		// list projects
		var matchEntries []matchEntry
		matchQuery := strings.ToLower(query)
		runningTimer, isRunning := getRunningTimer()

		for _, entry := range cache.Account.Data.Projects {
			idx := strings.Index(strings.ToLower(entry.Name), matchQuery)
			if idx != -1 {
				matchEntries = append(matchEntries, matchEntry{
					title:   entry.Name,
					start:   idx,
					id:      entry.Id,
					portion: float32(len(query)) / float32(len(entry.Name)),
				})
			}
		}

		if len(matchEntries) == 0 && query != "" {
			data := createProjectMessage{Name: parts[0]}
			dataString, _ := json.Marshal(data)

			items = append(items, alfred.Item{
				Title:       parts[0],
				SubtitleAll: "New project",
				Arg:         "create-project " + string(dataString),
			})
		} else {
			sort.Sort(sort.Reverse(byMatchId(matchEntries)))
			sort.Stable(sort.Reverse(byBestMatch(matchEntries)))

			for _, entry := range matchEntries {
				item := alfred.Item{
					Title:        entry.title,
					SubtitleAll:  entry.subtitle,
					Valid:        alfred.Invalid,
					Autocomplete: prefix + entry.title + alfred.Separator + " ",
				}

				if isRunning && runningTimer.Pid == entry.id {
					item.Icon = "running.png"
				}

				items = append(items, item)
			}
		}

		if len(items) == 0 {
			items = append(items, alfred.Item{Title: "No matching projects"})
		}
	}

	return items, nil
}

// update-project ----------------------------------------

type UpdateProjectAction struct{}

func (c UpdateProjectAction) Keyword() string {
	return "update-project"
}

func (c UpdateProjectAction) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c UpdateProjectAction) Do(query string) (string, error) {
	log.Printf("update-project %s", query)

	var project toggl.Project
	err := json.Unmarshal([]byte(query), &project)
	if err != nil {
		return "", fmt.Errorf("Invalid time entry %v", query)
	}

	session := toggl.OpenSession(config.ApiKey)

	project, err = session.UpdateProject(project)
	if err != nil {
		return "", err
	}

	adata := &cache.Account.Data

	for i, p := range adata.Projects {
		if p.Id == project.Id {
			adata.Projects[i] = project
			err = alfred.SaveJson(cacheFile, &cache)
			if err != nil {
				log.Printf("Error saving cache: %v\n", err)
			}
			break
		}
	}

	return fmt.Sprintf("Updated '%s'", project.Name), nil
}

// create-project ----------------------------------------

type createProjectMessage struct {
	Name string
	Wid  int
}

type CreateProjectAction struct{}

func (c CreateProjectAction) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c CreateProjectAction) Keyword() string {
	return "create-project"
}

func (c CreateProjectAction) Do(query string) (string, error) {
	log.Printf("create-project '%s'", query)

	var data createProjectMessage
	err := json.Unmarshal([]byte(query), &data)
	if err != nil {
		return "", err
	}

	session := toggl.OpenSession(config.ApiKey)
	if err != nil {
		return "", err
	}

	var project toggl.Project
	if data.Wid == 0 {
		data.Wid = cache.Account.Data.Workspaces[0].Id
	}
	project, err = session.CreateProject(data.Name, data.Wid)

	if err == nil {
		log.Printf("Got project: %#v\n", project)
		cache.Account.Data.Projects = append(cache.Account.Data.Projects, project)
		err := alfred.SaveJson(cacheFile, &cache)
		if err != nil {
			log.Printf("Error saving cache: %s\n", err)
		}
	}

	return fmt.Sprintf("Created project '%s'", data.Name), err
}
