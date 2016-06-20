package main

import (
	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-toggl"
)

// LoginCommand is a command
type LoginCommand struct{}

// Keyword returns the command's keyword
func (c LoginCommand) Keyword() string {
	return "login"
}

// IsEnabled returns true if the command is enabled
func (c LoginCommand) IsEnabled() bool {
	return config.APIKey == ""
}

// MenuItem returns the command's menu item
func (c LoginCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword(),
		Arg:          "login",
		Subtitle:     "Login to toggl.com",
	}
}

// Items returns a list of filter items
func (c LoginCommand) Items(args []string) ([]alfred.Item, error) {
	return []alfred.Item{c.MenuItem()}, nil
}

// Do runs the command
func (c LoginCommand) Do(args []string) (out string, err error) {
	var btn, username string
	btn, username, err = workflow.GetInput("Email address", "", false)
	if err != nil {
		return
	}

	if btn != "Ok" {
		dlog.Println("User didn't click OK")
		return
	}
	dlog.Printf("username: %s", username)

	var password string
	btn, password, err = workflow.GetInput("Password", "", true)
	if btn != "Ok" {
		dlog.Println("User didn't click OK")
		return
	}
	dlog.Printf("password: *****")

	var session toggl.Session
	session, err = toggl.NewSession(username, password)
	if err != nil {
		return
	}

	config.APIKey = session.ApiToken
	err = alfred.SaveJSON(configFile, &config)
	if err != nil {
		return
	}

	workflow.ShowMessage("Login successful!")
	return
}
