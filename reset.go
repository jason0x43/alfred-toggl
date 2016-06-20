package main

import (
	"os"

	"github.com/jason0x43/go-alfred"
)

// ResetCommand is a command
type ResetCommand struct{}

// Keyword returns the command's keyword
func (c ResetCommand) Keyword() string {
	return "reset"
}

// IsEnabled returns true if the command is enabled
func (c ResetCommand) IsEnabled() bool {
	return config.APIKey != ""
}

// MenuItem returns the command's menu item
func (c ResetCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword(),
		Subtitle:     "Reset this workflow, erasing all local data",
	}
}

// Items returns a list of filter items
func (c ResetCommand) Items(args []string) ([]alfred.Item, error) {
	item := c.MenuItem()
	item.Arg = "reset"
	return []alfred.Item{item}, nil
}

// Do runs the command
func (c ResetCommand) Do(args []string) (string, error) {
	err1 := os.Remove(configFile)
	err2 := os.Remove(cacheFile)

	if err1 != nil || err2 != nil {
		workflow.ShowMessage("One or more data files could not be removed")
	} else {
		workflow.ShowMessage("Workflow data cleared")
	}

	return "", nil
}
