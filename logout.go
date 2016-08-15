package main

import "github.com/jason0x43/go-alfred"

// LogoutCommand is a command
type LogoutCommand struct{}

// About returns information about this command
func (c LogoutCommand) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     "logout",
		Description: "Logout of Toggl",
		IsEnabled:   config.APIKey != "",
		Arg: &alfred.ItemArg{
			Keyword: "logout",
			Mode:    alfred.ModeDo,
		},
	}
}

// Do runs the command
func (c LogoutCommand) Do(data string) (out string, err error) {
	config.APIKey = ""
	err = alfred.SaveJSON(configFile, &config)
	if err != nil {
		return
	}

	workflow.ShowMessage("You are now logged out of Toggl")
	return
}
