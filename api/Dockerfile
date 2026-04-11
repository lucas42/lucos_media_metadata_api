FROM golang:1.26

WORKDIR /go/src/lucos_media_metadata_api

COPY go.* .
RUN go mod download

COPY rdfgen/ ./rdfgen/
COPY api .
COPY scripts/ ./scripts/
RUN go install && go build -o $(go env GOPATH)/bin/migrate_album_tags ./scripts/migrate_album_tags/

CMD ["lucos_media_metadata_api"]