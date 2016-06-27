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

var optionsDef = alfred.CommandDef{
	Keyword:     "options",
	Description: "Sets options",
	WithSpace:   true,
}

// About returns information about a command
func (c OptionsCommand) About() *alfred.CommandDef {
	return &optionsDef
}

// IsEnabled returns true if the command is enabled
func (c OptionsCommand) IsEnabled() bool {
	return config.APIKey != ""
}

// Items returns a list of filter items
func (c OptionsCommand) Items(arg, data string) (items []*alfred.Item, err error) {
	ct := reflect.TypeOf(config)
	cfg := reflect.Indirect(reflect.ValueOf(config))

	for i := 0; i < ct.NumField(); i++ {
		field := ct.Field(i)
		desc := field.Tag.Get("desc")
		if desc == "" {
			continue
		}

		name, value := alfred.SplitCmd(arg)
		if !alfred.FuzzyMatches(field.Name, name) {
			continue
		}

		item := &alfred.Item{
			Title:        field.Name + ": ",
			Subtitle:     desc,
			Autocomplete: field.Name,
			Arg: &alfred.ItemArg{
				Keyword: "options",
				Mode:    alfred.ModeDo,
			},
		}

		switch field.Type.Name() {
		case "bool":
			f := cfg.FieldByName(field.Name)
			item.Title += fmt.Sprintf("%v", f.Bool())
			if name == field.Name {
				item.Title += " (press Enter to toggle)"
			}

			// copy the current options, update them, and use as the arg
			opts := config
			o := reflect.Indirect(reflect.ValueOf(&opts))
			newVal := !f.Bool()
			o.FieldByName(field.Name).SetBool(newVal)
			item.Arg.Data = alfred.Stringify(opts)
		case "int":
			item.Autocomplete += " "

			if value != "" {
				val, err := strconv.Atoi(value)
				if err != nil {
					return items, err
				}
				item.Title += fmt.Sprintf("%d", val)

				// copy the current options, update them, and use as the arg
				opts := config
				o := reflect.Indirect(reflect.ValueOf(&opts))
				o.FieldByName(field.Name).SetInt(int64(val))
				item.Arg.Data = alfred.Stringify(opts)
			} else {
				f := cfg.FieldByName(field.Name)
				val := f.Int()
				item.Title += fmt.Sprintf("%v", val)
				if name == field.Name {
					item.Title += " (type a new value to change)"
				}
			}
		case "string":
			f := cfg.FieldByName(field.Name)
			item.Autocomplete += " "
			item.Title += f.String()
		}

		items = append(items, item)
	}
	return
}

// Do runs the command
func (c OptionsCommand) Do(arg, data string) (out string, err error) {
	log.Printf("options '%s'", arg)

	if err = json.Unmarshal([]byte(data), &config); err != nil {
		return
	}

	if err = alfred.SaveJSON(configFile, &config); err != nil {
		log.Printf("Error saving config: %s\n", err)
		return "Error updating options", err
	}

	return "Updated options", err
}
