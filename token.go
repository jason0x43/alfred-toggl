package main

import (
	"log"

	"github.com/jason0x43/go-alfred"
)

// TokenCommand is a command
type TokenCommand struct{}

// About returns information about this command
func (c TokenCommand) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     "token",
		Description: "Manually enter a Toggl API token",
		IsEnabled:   config.APIKey == "",
		Arg: &alfred.ItemArg{
			Keyword: "token",
			Mode:    alfred.ModeDo,
		},
	}
}

// Do runs the command
func (c TokenCommand) Do(data string) (string, error) {
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
