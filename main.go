package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"

	"github.com/jason0x43/go-alfred"
)

var dlog = log.New(os.Stderr, "[toggl] ", log.LstdFlags)

var cacheFile string
var configFile string
var config struct {
	APIKey           string `desc:"Toggl API key"`
	DurationOnly     bool   `desc:"Extend time entries instead of creating new ones."`
	Rounding         int    `desc:"Minutes to round to, 0 to disable rounding."`
	DefaultProjectID int    `desc:"Optional default project ID; set to 0 to clear"`
	TestMode         bool   `desc:"If true, disable auto refresh"`
}
var cache struct {
	Workspace int
	Account   toggl.Account
	Time      time.Time
}
var workflow *alfred.Workflow

func main() {
	var err error

	if !alfred.IsDebugging() {
		dlog.SetOutput(ioutil.Discard)
		dlog.SetFlags(0)
	}

	workflow, err = alfred.OpenWorkflow(".", true)
	if err != nil {
		fmt.Printf("Error: %s", err)
		os.Exit(1)
	}

	toggl.AppName = "alfred-toggl"

	configFile = path.Join(workflow.DataDir(), "config.json")
	cacheFile = path.Join(workflow.CacheDir(), "cache.json")

	dlog.Printf("Using config file: %s", configFile)
	dlog.Printf("Using cache file: %s", cacheFile)

	err = alfred.LoadJSON(configFile, &config)
	if err != nil {
		dlog.Println("Error loading config:", err)
	}
	dlog.Printf("loaded config")

	err = alfred.LoadJSON(cacheFile, &cache)
	dlog.Println("loaded cache")

	commands := []alfred.Command{
		StatusFilter{},
		// LoginCommand{},
		// TokenCommand{},
		TimeEntryCommand{},
		ProjectCommand{},
		TagCommand{},
		ReportFilter{},
		OptionsCommand{},
		// LogoutCommand{},
		// ResetCommand{},
	}

	workflow.Run(commands)
}
