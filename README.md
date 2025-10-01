# lucos_media_metadata_api
An API for managing media metadata.


## Dependencies

* docker
* docker-compose

## Build-time Dependencies

* [Golang](https://golang.org/)
* github.com/mattn/go-sqlite3
* github.com/jmoiron/sqlx

## Running
`nice -19 docker-compose up -d --no-build`

## Building
The build is configured to run in Dockerhub when a commit is pushed to the `main` branch in github.

## Running locally

* Install the build-time dependencies (see above)
* Run `go install`
* Run `lucos_media_metadata_api`

(The install command should add this to your $GOBIN.  Make sure you've added this to your $PATH correctly to find the command)

Accepts the following environment variables:

* *PORT* The tcp port to listen on.  Defaults to 8080

## Testing
Run `go test ./...`

[![CircleCI](https://circleci.com/gh/lucas42/lucos_media_metadata_api.svg?style=shield)](https://circleci.com/gh/lucas42/lucos_media_metadata_api)

For code coverage, run tests with:
`go test ./... -coverprofile=coverage.out`
Then, to view coverage report in browser, run:
`go tool cover -html=coverage.out`


## Backing Up
Copy the file from the docker host at /var/lib/docker/volumes/lucos_media_metadata_api_db/_data/media.sqlite
For example:
cp /var/lib/docker/volumes/lucos_media_metadata_api_db/_data/media.sqlite manual-backups/media-`date +%F`.sqlite