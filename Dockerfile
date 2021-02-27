FROM golang:1

WORKDIR /go/src/lucos_media_metadata_api

ENV GO111MODULE=auto

RUN go get github.com/mattn/go-sqlite3
RUN go get github.com/jmoiron/sqlx

COPY *.go ./

RUN go install

ENV PORT=3002
EXPOSE $PORT

CMD ["lucos_media_metadata_api"]