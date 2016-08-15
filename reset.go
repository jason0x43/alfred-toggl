package main

import (
	"os"

	"github.com/jason0x43/go-alfred"
)

// ResetCommand is a command
type ResetCommand struct{}

// About returns information about this command
func (c ResetCommand) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     "reset",
		Description: "Reset this workflow, erasing all local data",
		IsEnabled:   true,
		Arg: &alfred.ItemArg{
			Keyword: "reset",
			Mode:    alfred.ModeDo,
		},
	}
}

// Do runs the command
func (c ResetCommand) Do(data string) (string, error) {
	err1 := os.Remove(configFile)
	err2 := os.Remove(cacheFile)

	if err1 != nil || err2 != nil {
		workflow.ShowMessage("One or more data files could not be removed")
	} else {
		workflow.ShowMessage("Workflow data cleared")
	}

	return "", nil
}
