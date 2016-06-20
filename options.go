package main

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strconv"

	"github.com/jason0x43/go-alfred"
)

// OptionsCommand is a command
type OptionsCommand struct{}

// Keyword returns the command's keyword
func (c OptionsCommand) Keyword() string {
	return "options"
}

// IsEnabled returns true if the command is enabled
func (c OptionsCommand) IsEnabled() bool {
	return config.APIKey != ""
}

// MenuItem returns the command's menu item
func (c OptionsCommand) MenuItem() alfred.Item {
	return alfred.NewKeywordItem(c.Keyword(), "Set options")
}

// Items returns a list of filter items
func (c OptionsCommand) Items(args []string) (items []alfred.Item, err error) {
	ct := reflect.TypeOf(config)
	cfg := reflect.Indirect(reflect.ValueOf(config))

	var query string
	if len(args) > 0 {
		query = args[0]
	}

	prefix := c.Keyword() + " "

	dlog.Printf("options args: %#v", args)

	for i := 0; i < ct.NumField(); i++ {
		field := ct.Field(i)
		desc := field.Tag.Get("desc")
		if desc == "" {
			continue
		}

		parts := alfred.CleanSplitN(query, " ", 2)
		if !alfred.FuzzyMatches(field.Name, parts[0]) {
			continue
		}

		item := alfred.Item{
			Title:        field.Name + ": ",
			Subtitle:     desc,
			Autocomplete: prefix + field.Name + " ",
		}

		if field.Type.Name() == "bool" {
			f := cfg.FieldByName(field.Name)
			item.Title += fmt.Sprintf("%v", f.Bool())

			// copy the current options, update them, and use as the arg
			opts := config
			o := reflect.Indirect(reflect.ValueOf(&opts))
			newVal := !f.Bool()
			o.FieldByName(field.Name).SetBool(newVal)
			dataString, _ := json.Marshal(opts)
			item.Arg = "options " + string(dataString)
		} else if field.Type.Name() == "int" {
			if len(parts) > 1 && parts[1] != "" {
				help := field.Tag.Get("help")
				log.Printf("help: %s", help)
				val, err := strconv.Atoi(parts[1])
				if err != nil {
					return items, err
				}
				item.Title += fmt.Sprintf(help, val)

				// copy the current options, update them, and use as the arg
				opts := config
				o := reflect.Indirect(reflect.ValueOf(&opts))
				o.FieldByName(field.Name).SetInt(int64(val))
				dataString, _ := json.Marshal(opts)
				item.Arg = "options " + string(dataString)
			} else {
				f := cfg.FieldByName(field.Name)
				// copy the current options, update them, and use as the arg
				val := f.Int()
				if val == 0 {
					item.Title += "Not rounding"
				} else {
					item.Title += fmt.Sprintf("%d minute increments", val)
				}

				item.Invalid = true
			}
		}

		items = append(items, item)
	}
	items = alfred.SortItemsForKeyword(items, query)
	return
}

// Do runs the command
func (c OptionsCommand) Do(args []string) (string, error) {
	var query string
	if len(args) > 0 {
		query = args[0]
	}

	log.Printf("options '%s'", query)

	err := json.Unmarshal([]byte(query), &config)
	if err != nil {
		return "", err
	}

	err = alfred.SaveJSON(configFile, &config)
	if err != nil {
		log.Printf("Error saving config: %s\n", err)
	}

	return "", err
}
