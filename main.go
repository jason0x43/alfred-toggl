package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-toggl"
)

var dlog = log.New(os.Stderr, "[toggl] ", log.LstdFlags)
var cacheFile string
var configFile string
var config struct {
	APIKey           string `desc:"Toggl API key"`
	DurationOnly     bool   `desc:"Extend time entries instead of creating new ones."`
	Rounding         int    `desc:"Minutes to round to, 0 to disable rounding." help:"%v minute increments"`
	DefaultProjectID int    `desc:"Optional default project ID"`
	TestMode         bool   `desc:"If true, disable auto refresh"`
}
var cache struct {
	Workspace int
	Account   toggl.Account
	Time      time.Time
}
var workflow alfred.Workflow

func main() {
	var err error

	workflow, err = alfred.OpenWorkflow(".", true)
	if err != nil {
		fmt.Printf("Error: %s", err)
		os.Exit(1)
	}

	toggl.AppName = "alfred-toggl"

	configFile = path.Join(workflow.DataDir(), "config.json")
	cacheFile = path.Join(workflow.CacheDir(), "cache.json")

	dlog.Println("Using config file", configFile)
	dlog.Println("Using cache file", cacheFile)

	err = alfred.LoadJSON(configFile, &config)
	if err != nil {
		dlog.Println("Error loading config:", err)
	}
	dlog.Println("loaded config:", config)

	err = alfred.LoadJSON(cacheFile, &cache)
	dlog.Println("loaded cache")

	commands := []alfred.Command{
		LoginCommand{},
		TokenCommand{},
		TimeEntryCommand{},
		ProjectCommand{},
		TagFilter{},
		ReportFilter{},
		SyncFilter{},
		OptionsCommand{},
		LogoutCommand{},
		ResetCommand{},
		StartAction{},
		UpdateTagAction{},
		CreateTagAction{},
		ToggleAction{},
		DeleteAction{},
	}

	workflow.Run(commands)
}
