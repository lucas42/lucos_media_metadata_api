FROM golang:1.24

WORKDIR /go/src/lucos_media_metadata_api

COPY go.* .
RUN go mod download

COPY src .
RUN go install

ENV PORT=3002
EXPOSE $PORT

CMD ["lucos_media_metadata_api"]