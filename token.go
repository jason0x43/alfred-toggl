package main

import (
	"log"

	"github.com/jason0x43/go-alfred"
)

// TokenCommand is a command
type TokenCommand struct{}

// Keyword returns the command's keyword
func (c TokenCommand) Keyword() string {
	return "token"
}

// IsEnabled returns true if the command is enabled
func (c TokenCommand) IsEnabled() bool {
	return config.APIKey == ""
}

// MenuItem returns the command's menu item
func (c TokenCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword(),
		// Arg:          "token",
		Subtitle: "Manually enter toggl.com API token",
	}
}

// Items returns a list of filter items
func (c TokenCommand) Items(args []string) ([]alfred.Item, error) {
	return []alfred.Item{c.MenuItem()}, nil
}

// Do runs the command
func (c TokenCommand) Do(args []string) (string, error) {
	btn, token, err := workflow.GetInput("API token", "", false)
	if err != nil {
		return "", err
	}

	if btn != "Ok" {
		log.Println("User didn't click OK")
		return "", nil
	}
	log.Printf("token: %s", token)

	config.APIKey = token
	err = alfred.SaveJSON(configFile, &config)
	if err != nil {
		return "", err
	}

	workflow.ShowMessage("Token saved!")
	return "", nil
}
