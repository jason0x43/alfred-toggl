package main

import (
	"encoding/json"
	"fmt"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-toggl"
)

// ProjectCommand is a command for handling projects
type ProjectCommand struct{}

// About returns information about a command
func (c ProjectCommand) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     "projects",
		Description: "List your projects, add new ones",
		IsEnabled:   config.APIKey != "",
	}
}

// Items returns a list of filter items
func (c ProjectCommand) Items(arg, data string) (items []alfred.Item, err error) {
	if err = checkRefresh(); err != nil {
		return
	}

	var cfg projectCfg

	if data != "" {
		if err = json.Unmarshal([]byte(data), &cfg); err != nil {
			dlog.Printf("Error unmarshalling projects var: %v", err)
		}
	}

	pid := -1
	if cfg.Project != nil {
		pid = *cfg.Project
	}

	runningTimer, isRunning := getRunningTimer()

	if pid != -1 {
		// List menu for a project
		if project, _, ok := getProjectByID(pid); ok {
			return projectItems(project, arg)
		}
	} else {
		projectCfg := projectCfg{}

		for _, entry := range cache.Account.Data.Projects {
			if entry.IsActive() && alfred.FuzzyMatches(entry.Name, arg) {
				projectCfg.Project = &entry.ID

				item := alfred.Item{
					UID:          fmt.Sprintf("%s.project.%d", workflow.BundleID(), entry.ID),
					Title:        entry.Name,
					Subtitle:     "",
					Autocomplete: entry.Name,
					Icon:         "off.png",
					Arg: &alfred.ItemArg{
						Keyword: "projects",
						Data:    alfred.Stringify(projectCfg),
					},
				}

				if isRunning && runningTimer.Pid == entry.ID {
					item.Icon = "running.png"
				}

				items = append(items, item)
			}
		}

		if len(items) == 0 && arg != "" {
			projectCfg.ToCreate = &createProjectMessage{Name: arg}

			items = append(items, alfred.Item{
				Title:    arg,
				Subtitle: "New project",
				Arg: &alfred.ItemArg{
					Keyword: "projects",
					Mode:    alfred.ModeDo,
					Data:    alfred.Stringify(projectCfg),
				},
			})
		}

		if len(items) == 0 {
			items = append(items, alfred.Item{Title: "No matching projects"})
		}
	}

	return items, nil
}

// Do runs the command
func (c ProjectCommand) Do(data string) (out string, err error) {
	var cfg projectCfg

	if data != "" {
		if err := json.Unmarshal([]byte(data), &cfg); err != nil {
			dlog.Printf("Error unmarshaling project data: %v", err)
		}
	}

	if cfg.Default != nil {
		dlog.Printf("setting default project to %v", cfg.Default)
		config.DefaultProjectID = *cfg.Default
		if err := alfred.SaveJSON(configFile, &config); err != nil {
			return "Error saving config", err
		}
		return fmt.Sprintf(`Set default project to %d`, cfg.Default), nil
	}

	if cfg.ToCreate != nil {
		dlog.Printf("creating project %v", cfg.ToCreate)
		var project toggl.Project
		if project, err = createProject(cfg.ToCreate); err != nil {
			return
		}
		return fmt.Sprintf(`Created project "%s"`, project.Name), nil
	}

	if cfg.ToUpdate != nil {
		dlog.Printf("updating project %v", cfg.ToUpdate)
		var project toggl.Project
		if project, err = updateProject(cfg.ToUpdate); err != nil {
			return
		}
		return fmt.Sprintf(`Updated project "%s"`, project.Name), nil
	}

	return "Unrecognized input", nil
}

// support -------------------------------------------------------------------

type projectCfg struct {
	Project  *int                  `json:"project,omitempty"`
	Default  *int                  `json:"default,omitempty"`
	ToCreate *createProjectMessage `json:"create,omitempty"`
	ToUpdate *toggl.Project        `json:"update,omitempty"`
}

type createProjectMessage struct {
	Name string
	WID  int
}

func createProject(msg *createProjectMessage) (project toggl.Project, err error) {
	session := toggl.OpenSession(config.APIKey)

	if msg.WID == 0 {
		msg.WID = cache.Account.Data.Workspaces[0].ID
	}

	if project, err = session.CreateProject(msg.Name, msg.WID); err == nil {
		cache.Account.Data.Projects = append(cache.Account.Data.Projects, project)
		if err := alfred.SaveJSON(cacheFile, &cache); err != nil {
			dlog.Printf("Error saving cache: %s\n", err)
		}
	}

	return
}

func updateProject(p *toggl.Project) (project toggl.Project, err error) {
	session := toggl.OpenSession(config.APIKey)

	if project, err = session.UpdateProject(*p); err != nil {
		return
	}

	adata := &cache.Account.Data

	for i, p := range adata.Projects {
		if p.ID == project.ID {
			adata.Projects[i] = project
			if err := alfred.SaveJSON(cacheFile, &cache); err != nil {
				dlog.Printf("Error saving cache: %v\n", err)
			}
			break
		}
	}

	return
}

func projectItems(project toggl.Project, arg string) (items []alfred.Item, err error) {
	if alfred.FuzzyMatches("name:", arg) {
		item := alfred.Item{}
		_, name := alfred.SplitCmd(arg)

		if name != "" {
			updateEntry := project
			updateEntry.Name = name
			item.Title = fmt.Sprintf("Change name to '%s'", name)
			item.Subtitle = "Name: " + project.Name
			item.Arg = &alfred.ItemArg{
				Keyword: "projects",
				Data:    alfred.Stringify(projectCfg{ToUpdate: &updateEntry}),
			}

		} else {
			item.Title = "Name: " + project.Name
			item.Autocomplete = "Name: "
		}

		items = append(items, item)
	}

	if project.ID != config.DefaultProjectID {
		if alfred.FuzzyMatches("Make default", arg) {
			c := config
			c.DefaultProjectID = project.ID
			items = append(items, alfred.Item{
				Title:        "Make default",
				Subtitle:     "Make this the default project",
				Autocomplete: "Make default",
				Arg: &alfred.ItemArg{
					Keyword: "options",
					Mode:    alfred.ModeDo,
					Data:    alfred.Stringify(c),
				},
			})
		}
	} else {
		if alfred.FuzzyMatches("Clear default", arg) {
			c := config
			c.DefaultProjectID = 0
			items = append(items, alfred.Item{
				Title:        "Clear default",
				Subtitle:     "Clear the default project",
				Autocomplete: "Clear default",
				Arg: &alfred.ItemArg{
					Keyword: "options",
					Mode:    alfred.ModeDo,
					Data:    alfred.Stringify(c),
				},
			})
		}
	}

	if alfred.FuzzyMatches("timers", arg) {
		items = append(items, alfred.Item{
			Title:        "Time entries...",
			Subtitle:     "List associated time entries",
			Autocomplete: "Time entries...",
			Arg: &alfred.ItemArg{
				Keyword: "timers",
				Data:    alfred.Stringify(timerCfg{Project: &project.ID}),
			},
		})
	}

	return
}
