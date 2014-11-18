default: workflow

.PHONY: link
link: workflow
	@if [ -z `find -L $(ALFRED_WORKFLOWS) -samefile build -maxdepth 1` ]; then ln -s `pwd`/build $(ALFRED_WORKFLOWS)/user.workflow.`uuidgen`; fi

.PHONY: unlink
unlink:
	@find -L $(ALFRED_WORKFLOWS) -samefile build -maxdepth 1 -exec rm {} \;

.PHONY: workflow
workflow: build/alfred-toggl README.md LICENSE.txt resources/*
	@cp resources/* build
	@cp LICENSE.txt build
	@cp README.md build

.PHONY: package
package: build/alfred-toggl.alfredworkflow

build/alfred-toggl.alfredworkflow: build/alfred-toggl README.md LICENSE.txt resources/*
	@cd build && zip -r alfred-toggl.alfredworkflow .

build/alfred-toggl: *.go
	@mkdir -p build
	@go build -ldflags="-w" -o $@ $^

.PHONY: clean
clean:
	@rm -rf build
