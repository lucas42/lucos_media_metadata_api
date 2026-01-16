FROM golang:1.26rc2

WORKDIR /go/src/lucos_media_metadata_api

COPY go.* .
RUN go mod download

COPY rdfgen/ ./rdfgen/
COPY api .
RUN go install

CMD ["lucos_media_metadata_api"]