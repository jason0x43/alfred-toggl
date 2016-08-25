alfred-toggl
============

An Alfred workflow for accessing the Toggl time-tracking service

![Screenshot](doc/report_dates.png?raw=true)

Installation
------------

Download the latest workflow package from the [releases page](https://github.com/jason0x43/alfred-toggl/releases) and double click it — Alfred will take care of the rest.

Usage
-----

The workflow uses one main keyword: “tgl”. The first time you use the keyword, only two items will be available: `login`, `token`, and `reset`. Actioning `login` will prompt you for your username and password, which are used to temporarily authenticate with toggl.com to retrieve your API token. The `token` item can be used to directly enter your API token (available at the bottom of your Toggl profile page).

![Login](doc/tgl_logged_out.png?raw=true)

Once you’ve logged in, a number of commands will be available.

![Main menu](doc/tgl_logged_in.png?raw=true)

Items may be either “tabbed” or “actioned”. Tabbing means to press the Tab key, and actioning means to press the Return or Enter key. In general, tabbing will auto-complete, while actioning takes an action or enters a new mode (although it may also auto-complete in certain situations).

The workflow can operate in various modes, described below. For example, the timers mode lists timers and allows the user to create, update, and delete them. To enter the timers mode, type `tgl timers` and press Enter. The major modes also have direct keywords. Typing `tgt` is equivalent to typing `tgl timers` and pressing Enter.

### `timers`

The `timers` command (`tgl timers` or `tgt`) lists all user time entries created during the last 9 days, up to 1000. Entries are listed in reverse chronological order by start time. If a timer is active, it’s icon will be green.

![Timers list](doc/timers.png?raw=true)

Actioning a time entry will show various properties for that entry, and also allow the entry to be modified or deleted. Holding `Cmd` while actioning a time entry from the list will continue the time entry (either creating a new instance of the entry or extending its duration, depending on the configured default behavior).

In the timer property list, actioning a property will allow it to be modifed. If the property involves selecting an option or a true/false value, a checklist of possible values will be presented. If the property is a string, number, or time, a new value can be entered directly. Pressing Enter will update the property.

![Timer menu](doc/timer_properties.png?raw=true)

### `projects`

The `projects` (`tgl projects` or `tgp`) command lists all user projects. Projects are listed in reverse creation order. If a timer is active, its project icon will be green.

![Projects list](doc/projects.png?raw=true)

Actioning a project will present a list of properties (name), subcommands (delete), and an item for listing the project’s time entries. Actioning a property will show the property’s value, and may allow it to be changed. Actioning a subcommand will execute the subcommand.

![Project menu](doc/project_properties.png?raw=true)

A new project may be added by entering a unique project name when the `projects` list is displayed.

### `tags`

The `tags` command (`tgl tags`) lists all user tags in alphabetical order. A new tag may be added by entering a unique tag name when the `tags` list is displayed.

### `report`

The `report` command (`tgl report` or `tgr`) can be used to generate summary time-spent reports for the current or previous days or for the current week (starting on Monday). 

![Report menu](doc/report_list.png?raw=true)

Actioning one of the report types will show the total hours for the given time frame, as well as a breakdown of how many hours were spent on each project.

![This week’s report](doc/report_week.png?raw=true)

Actioning one of the projects will show how time was spent on that project, broken up by task. Multiple time entries with the same description will be grouped into a single task. Actioning a time entry will show how that time entry was distributed over the reporting period.

The report date or period may also be specified manually. A single date may be entered using a variety of formats, such as ‘2016-08-12’ or ‘8/12’. A range of dates may be specified by separating two dates with ‘..’ (like ‘8/10..8/15’).

![Custom reporting period](doc/report_manual.png?raw=true)

### `options`

The `options` command (`tgl options` or `tgo`) lists user-configurable options and allows the user to modify them.

![Options menu](doc/options.png?raw=true)

As in other modes, actioning an option will allow a new value to be specified. Values with discrete options will allow the user to pick from a list, while numbers and strings will allow the user to directly enter a new value.

### `status`

The `status` command (`tgl status` or `tgs`) will download current user data, including account info, tags, projects, and time entries for the last 9 days, from Toggl.com, and will show the currently running timer and the total time spent in the current day.

![Current status](doc/status.png?raw=true)

### `logout`

The `logout` commmand will clear the locally stored copy of the user‘s API token, preventing the workflow from interacting with Toggl.com. Other locally cached data and configuration information will not be affected.

### `reset`

The `reset` command will clear all locally cached data and configuration information, including the API token. This returns the workflow to a clean initial state.
