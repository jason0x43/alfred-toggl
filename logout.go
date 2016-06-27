package main

import "github.com/jason0x43/go-alfred"

// LogoutCommand is a command
type LogoutCommand struct{}

// Keyword returns the command's keyword
func (c LogoutCommand) Keyword() string {
	return "logout"
}

// IsEnabled returns true if the command is enabled
func (c LogoutCommand) IsEnabled() bool {
	return config.APIKey != ""
}

// MenuItem returns the command's menu item
func (c LogoutCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword(),
		Subtitle:     "Logout of Toggl, but keep any local config",
		// Arg:          "logout",
	}
}

// Items returns a list of filter items
func (c LogoutCommand) Items(args []string) ([]alfred.Item, error) {
	return []alfred.Item{c.MenuItem()}, nil
}

// Do runs the command
func (c LogoutCommand) Do(args []string) (out string, err error) {
	config.APIKey = ""
	err = alfred.SaveJSON(configFile, &config)
	if err != nil {
		return
	}

	workflow.ShowMessage("You are now logged out of Toggl")
	return
}
