package main

import "github.com/jason0x43/go-alfred"

type SyncFilter struct{}

func (c SyncFilter) Keyword() string {
	return "sync"
}

func (c SyncFilter) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c SyncFilter) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword(),
		SubtitleAll:  "Sync with toggl.com",
		Valid:        alfred.Invalid,
	}
}

func (c SyncFilter) Items(prefix, query string) ([]alfred.Item, error) {
	err := refresh()
	if err != nil {
		return []alfred.Item{}, err
	}
	return []alfred.Item{alfred.Item{
		Title: "Synchronized!",
		Valid: alfred.Invalid,
	}}, nil
}
