PROFILE ?= default
CRON ?= cron(0 0,8 * * ? *)

deploy-prod: build
	sls deploy --stage prod --profile $(PROFILE) --cron "$(CRON)"

deploy-stg: build
	sls deploy --stage stg --profile $(PROFILE) --cron "$(CRON)"

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/main main.go struct.go

run:
	ENV=local go run main.go struct.go
