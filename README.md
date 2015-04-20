alfred-toggl
============

An Alfred workflow for accessing the Toggl time-tracking service

![Screenshot](doc/report_yesterday.png?raw=true)

Installation
------------

Download the latest workflow package from the [releases page](https://github.com/jason0x43/alfred-toggl/releases) and double click it — Alfred will take care of the rest.

Usage
-----

The workflow uses one main keyword: “tgl”. The first time you use the keyword, only two items will be available: `login` and `token`. Actioning `login` will prompt you for your username and password, which are used to temporarily authenticate with toggl.com to retrieve your API token. The `token` item can be used to manually enter your API token (available at the bottom of your Toggl profile page).

![Login](doc/initial_menu.png?raw=true)

Once you’ve logged in, a number of commands will be available.

![Main menu](doc/main_menu.png?raw=true)

Items may be either “tabbed” or “actioned”. Tabbing means to press the Tab key, and actioning means to press the Return or Enter key.

### `timers`

The `timers` command lists all user time entries created during the last 9 days, up to 1000. Entries are listed in reverse chronological order by start time. If a timer is active, it’s icon will be green.

![Timers list](doc/timers.png?raw=true)

Actioning a time entry will continue that entry, either by starting a new time entry with the same project and description, or by increasing the duration of the time entry. The specific behavior is user configurable.

Tabbing a time entry will show a list of entry properties (description, project, tags) and subcommands (delete). Tabbing a selected property will show that property’s value. Some properties, such as “description”, can be modified. For these, entering a new value and pressing Enter will update the time entry.

![Timer menu](doc/timer_menu.png?raw=true)

Actioning the “delete” command will first ask the user to confirm the deletion, then delete the time entry.

![Delete confirmation](doc/delete_confirm.png?raw=true)

### `projects`

The `projects` command lists all user projects. Projects are listed in reverse creation order. If a timer is active, its project icon will be green.

![Projects list](doc/projects.png?raw=true)

Tabbing a project will present a list of properties (name) and subcommands (delete). Tabbing a property will show the property’s value, and may allow it to be changed. Actioning a subcommand will execute the subcommand.

A new project may be added by entering a unique project name when the `projects` list is displayed.

![Project menu](doc/project_menu.png?raw=true)

### `tags`

The `tags` command lists all user tags in alphabetical order.

![Tags list](doc/tags.png?raw=true)

Tabbing a tag will present a list of properties (name) and subcommands (delete). Tabbing a property will show the property’s value, and may allow it to be changed. Actioning a subcommand will execute the subcommand.

A new tag may be added by entering a unique project name when the `tags` list is displayed.

![Tag menu](doc/tag_menu.png?raw=true)

### `report`

The `report` command can be used to generate summary time-spent reports for the current or previous days or for the current week (starting on Monday). 

![Report menu](doc/report_menu.png?raw=true)

Tabbing one of the report types will show the total hours for the given time frame, as well as a breakdown of how many hours were spent on each project.

![This week's report](doc/report_this_week.png?raw=true)

Tabbing one of the projects will show how time was spent on that project, broken up by task. Multiple time entries with the same description will be grouped into a single task.

![This week's report for a project](doc/report_this_week_project.png?raw=true)

### `options`

The `options` command lists user-configurable options and allows the user to modify them.

![Options menu](doc/options_menu.png?raw=true)

### `sync`

The `sync` command will download current user data, including account info, tags, projects, and time entries for the last 9 days, from Toggl.com. Tabbing the `sync` item, or just typing “sync”, will begin the sync process. `sync` is not an actionable command, and actioning the `sync` item will simply autocomplete it (just like tabbing). After a few seconds, the item’s message will changed to “Synchronized!”, showing that the sychronization process is complete.

### `logout`

The `logout` commmand will clear the locally stored copy of the user's API token, preventing the workflow from interacting with Toggl.com. Other locally cached data and configuration information is not affected.

### `reset`

The `reset` command will clear all locally cached data and configuration information, including the API token. This returns the workflow to a clean initial state.
