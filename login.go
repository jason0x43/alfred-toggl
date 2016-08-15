package main

import "github.com/jason0x43/go-alfred"

// LoginCommand is a command
type LoginCommand struct{}

// About returns information about this command
func (c LoginCommand) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     "login",
		Description: "Login to Toggl",
		IsEnabled:   config.APIKey == "",
		Arg: &alfred.ItemArg{
			Keyword: "login",
			Mode:    alfred.ModeDo,
		},
	}
}

// Do runs the command
func (c LoginCommand) Do(data string) (out string, err error) {
	var btn, username string
	if btn, username, err = workflow.GetInput("Email address", "", false); err != nil {
		return
	}

	if btn != "Ok" {
		dlog.Println("User didn't click OK")
		return
	}
	dlog.Printf("username: %s", username)

	var password string
	if btn, password, err = workflow.GetInput("Password", "", true); btn != "Ok" {
		dlog.Println("User didn't click OK")
		return
	}
	dlog.Printf("password: *****")

	var session Session
	if session, err = NewSession(username, password); err != nil {
		workflow.ShowMessage("Login failed!")
		return
	}

	config.APIKey = session.APIToken
	if err = alfred.SaveJSON(configFile, &config); err != nil {
		return
	}

	workflow.ShowMessage("Login successful!")
	return
}
