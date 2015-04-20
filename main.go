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

var cacheFile string
var configFile string
var config Config
var cache Cache
var workflow alfred.Workflow

type Config struct {
	ApiKey   string `json:"api_key"`
	TestMode bool
}

type Cache struct {
	Workspace int
	Account   toggl.Account
	Time      time.Time
}

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

	log.Println("Using config file", configFile)
	log.Println("Using cache file", cacheFile)

	err = alfred.LoadJson(configFile, &config)
	if err != nil {
		log.Println("Error loading config:", err)
	}
	log.Println("loaded config:", config)

	err = alfred.LoadJson(cacheFile, &cache)
	log.Println("loaded cache")

	commands := []alfred.Command{
		LoginCommand{},
		TokenCommand{},
		TimeEntryFilter{},
		ProjectFilter{},
		TagFilter{},
		ReportFilter{},
		SyncFilter{},
		OptionsCommand{},
		LogoutCommand{},
		ResetCommand{},
		StartAction{},
		UpdateTimeEntryAction{},
		UpdateProjectAction{},
		CreateProjectAction{},
		UpdateTagAction{},
		CreateTagAction{},
		ToggleAction{},
		DeleteAction{},
	}

	workflow.Run(commands)
}
