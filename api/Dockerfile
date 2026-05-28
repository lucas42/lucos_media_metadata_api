FROM golang:1.26
ARG VERSION
ENV VERSION=$VERSION

WORKDIR /go/src/lucos_media_metadata_api

COPY go.* .
RUN go mod download

COPY predicateconfig/ ./predicateconfig/
COPY rdfgen/ ./rdfgen/
COPY api .
COPY scripts/ ./scripts/
RUN go install && go build -o $(go env GOPATH)/bin/migrate_album_tags ./scripts/migrate_album_tags/ && go build -o $(go env GOPATH)/bin/migrate_creative_work_tags ./scripts/migrate_creative_work_tags/ && go build -o $(go env GOPATH)/bin/migrate_offence_tags ./scripts/migrate_offence_tags/

CMD ["lucos_media_metadata_api"]