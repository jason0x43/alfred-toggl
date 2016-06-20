package main

import "github.com/jason0x43/go-alfred"

// SyncFilter is a command
type SyncFilter struct{}

// Keyword returns the command's keyword
func (c SyncFilter) Keyword() string {
	return "sync"
}

// IsEnabled returns true if the command is enabled
func (c SyncFilter) IsEnabled() bool {
	return config.APIKey != ""
}

// MenuItem returns the command's menu item
func (c SyncFilter) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword(),
		Subtitle:     "Sync with toggl.com",
		Invalid:      true,
	}
}

// Items returns a list of filter items
func (c SyncFilter) Items(args []string) ([]alfred.Item, error) {
	err := refresh()
	if err != nil {
		return []alfred.Item{}, err
	}
	return []alfred.Item{alfred.Item{
		Title:   "Synchronized!",
		Invalid: true,
	}}, nil
}
