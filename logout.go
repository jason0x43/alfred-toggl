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
		Arg:          "logout",
	}
}

func (c LogoutCommand) Items(prefix, query string) ([]alfred.Item, error) {
	return []alfred.Item{c.MenuItem()}, nil
}

func (c LogoutCommand) Do(query string) (out string, err error) {
	config.ApiKey = ""
	err = alfred.SaveJson(configFile, &config)
	if err != nil {
		return
	}

	workflow.ShowMessage("You are now logged out of Toggl")
	return
}
