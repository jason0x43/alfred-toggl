package main

import (
	"os"

	"github.com/jason0x43/go-alfred"
)

type ResetCommand struct{}

func (c ResetCommand) Keyword() string {
	return "reset"
}

func (c ResetCommand) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c ResetCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword(),
		SubtitleAll:  "Reset this workflow, erasing all local data",
	}
}

func (c ResetCommand) Items(prefix, query string) ([]alfred.Item, error) {
	item := c.MenuItem()
	item.Arg = "reset"
	return []alfred.Item{item}, nil
}

func (c ResetCommand) Do(query string) (string, error) {
	err1 := os.Remove(configFile)
	err2 := os.Remove(cacheFile)

	if err1 != nil || err2 != nil {
		workflow.ShowMessage("One or more data files could not be removed")
	} else {
		workflow.ShowMessage("Workflow data cleared")
	}

	return "", nil
}
