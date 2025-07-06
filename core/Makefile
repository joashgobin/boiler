production_host_ip = "<ip>"
project_name = "<appName>"
username = "<user>"

## help: list all options
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

## deploy: upload updated version of app to VPS
.PHONY: deploy
deploy: build
	@echo "deploying to VPS"
	@echo "transferring static files"
	rm -rf static/gen
	ssh ${username}@${production_host_ip} "mkdir -p ~/${project_name}"
	rsync -rP ./static ${username}@${production_host_ip}:~/${project_name}/
	rsync -rP ./remote ${username}@${production_host_ip}:~/${project_name}/
	rsync -P ./Makefile ${username}@${production_host_ip}:~/${project_name}/
	@echo "transferring binary"
	rsync -P ./bin/linux_amd64/web ${username}@${production_host_ip}:~/${project_name}/
	ssh -t ${username}@${production_host_ip} "cd ~/${project_name}/ && sudo systemctl link remote/${project_name}.service && sudo systemctl daemon-reload && sudo systemctl enable ${project_name}.service && sudo systemctl restart ${project_name}.service && sudo cp -r ~/${project_name}/static/ /usr/local/share/fiber/"

## build: build local copy
.PHONY: build
build:
	@echo "building app locally"
	@go build -ldflags='-s' -o=./bin/web .
	@GOOS=linux GOARCH=amd64 go build -ldflags='-s' -o=./bin/linux_amd64/web .

## tidy: tidy module dependencies and format all .go files
.PHONY: tidy
tidy:
	@echo 'Tidying module dependencies...'
	go mod tidy
	@echo 'Verifying and vendoring module dependencies...'
	go mod verify
	go mod vendor
	@echo 'Formatting .go files...'
	go fmt ./...

## audit: run quality control checks
.PHONY: audit
audit:
	@echo 'Checking module dependencies...'
	go mod tidy -diff
	go mod verify
	@echo 'Vetting code...'
	go vet ./...
	go tool staticcheck ./...
	@echo 'Running tests...'
	go test -race -vet=off ./...

.PHONY: up
up:
	cat remote/create_app_database.sql | sudo mysql
