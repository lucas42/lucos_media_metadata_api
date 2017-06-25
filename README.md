# lucos_media_metadata_api
An API for managing media metadata.

## Requirments

* [Golang](https://golang.org/)
* github.com/mattn/go-sqlite3

## Installing
Run `go install`

## Running
Run `lucos_media_metadata_api`

(The install command should add this to your $GOBIN.  Make sure you've added this to your $PATH correctly to find the command)

Accepts the following environment variables:

* *PORT* The tcp port to listen on.  Defaults to 8080

## Testing
Run `go test`

[![CircleCI](https://circleci.com/gh/lucas42/lucos_media_metadata_api.svg?style=shield)](https://circleci.com/gh/lucas42/lucos_media_metadata_api)