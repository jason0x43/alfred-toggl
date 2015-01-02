package main

import (
	"log"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-toggl"
)

type LoginCommand struct{}

func (c LoginCommand) Keyword() string {
	return "login"
}

func (c LoginCommand) IsEnabled() bool {
	return config.ApiKey == ""
}

func (c LoginCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword(),
		Arg:          "login",
		SubtitleAll:  "Login to toggl.com",
	}
}

func (c LoginCommand) Items(prefix, query string) ([]alfred.Item, error) {
	return []alfred.Item{c.MenuItem()}, nil
}

func (c LoginCommand) Do(query string) (string, error) {
	btn, username, err := workflow.GetInput("Email address", "", false)
	if err != nil {
		return "", err
	}

	if btn != "Ok" {
		log.Println("User didn't click OK")
		return "", nil
	}
	log.Printf("username: %s", username)

	btn, password, err := workflow.GetInput("Password", "", true)
	if btn != "Ok" {
		log.Println("User didn't click OK")
		return "", nil
	}
	log.Printf("password: *****")

	session, err := toggl.NewSession(username, password)
	if err != nil {
		return "", err
	}

	config.ApiKey = session.ApiToken
	err = alfred.SaveJson(configFile, &config)
	if err != nil {
		return "", err
	}

	workflow.ShowMessage("Login successful!")
	return "", nil
}
