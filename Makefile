VERSION ?= `git describe --abbrev=0 --tags`

release:
	rm -f build/*
	env GOOS=linux GOARCH=amd64 go build
	zip build/terraform-index-$(VERSION)-linux-amd64.zip terraform-index
	rm terraform-index

	env GOOS=windows GOARCH=amd64 go build
	zip build/terraform-index-$(VERSION)-windows-amd64.zip terraform-index.exe
	rm terraform-index.exe

	env GOOS=darwin GOARCH=amd64 go build
	zip build/terraform-index-$(VERSION)-macos-amd64.zip terraform-index
	rm terraform-index

.PHONY: release