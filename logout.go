package main

import "github.com/jason0x43/go-alfred"

type LogoutCommand struct{}

func (c LogoutCommand) Keyword() string {
	return "logout"
}

func (c LogoutCommand) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c LogoutCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword(),
		SubtitleAll:  "Logout of Toggl, but keep any local config",
	}
}

func (c LogoutCommand) Items(prefix, query string) ([]alfred.Item, error) {
	item := c.MenuItem()
	item.Arg = "logout"
	return []alfred.Item{item}, nil
}

func (c LogoutCommand) Do(query string) (string, error) {
	config.ApiKey = ""
	err := alfred.SaveJson(configFile, &config)
	if err != nil {
		return "", err
	}

	workflow.ShowMessage("You are now logged out of Toggl")
	return "", nil
}
