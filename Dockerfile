ARG BUILD_FROM=alpine:3.22
FROM golang:1.27-alpine3.22 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /huebridge ./cmd/huebridge

FROM ${BUILD_FROM}
COPY --from=build /huebridge /usr/bin/huebridge
COPY run.sh /run.sh
RUN chmod a+x /run.sh
CMD ["/run.sh"]
