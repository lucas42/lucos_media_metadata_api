FROM golang:1.27
ARG VERSION
ENV VERSION=$VERSION

WORKDIR /go/src/lucos_media_metadata_api

COPY go.* .
RUN go mod download

COPY predicateconfig/ ./predicateconfig/
COPY rdfgen/ ./rdfgen/
COPY api .
RUN go install

CMD ["lucos_media_metadata_api"]