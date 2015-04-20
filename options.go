package main

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"

	"github.com/jason0x43/go-alfred"
)

type OptionsCommand struct{}

func (c OptionsCommand) Keyword() string {
	return "options"
}

func (c OptionsCommand) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c OptionsCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword() + " ",
		SubtitleAll:  "Set options",
		Valid:        alfred.Invalid,
	}
}

func (c OptionsCommand) Items(prefix, query string) (items []alfred.Item, err error) {
	ct := reflect.TypeOf(config)
	cfg := reflect.Indirect(reflect.ValueOf(config))

	for i := 0; i < ct.NumField(); i++ {
		field := ct.Field(i)
		desc := field.Tag.Get("desc")
		if desc == "" {
			continue
		}

		parts := strings.SplitN(query, " ", 2)
		if !alfred.FuzzyMatches(field.Name, parts[0]) {
			continue
		}

		item := alfred.Item{
			Title:    field.Name + ": ",
			Subtitle: desc,
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

				item.Valid = alfred.Invalid
				item.Autocomplete = prefix + field.Name + " "
			}
		}

		items = append(items, item)
	}
	return
}

func (c OptionsCommand) Do(query string) (string, error) {
	log.Printf("options '%s'", query)

	var data Config
	err := json.Unmarshal([]byte(query), &data)
	if err != nil {
		return "", err
	}

	err = alfred.SaveJson(configFile, &data)
	if err != nil {
		log.Printf("Error saving cache: %s\n", err)
	}

	return "", err
}
