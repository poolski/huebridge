ARG BUILD_FROM=golang:1.27-bookworm
FROM golang:1.27-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /huebridge ./cmd/huebridge

FROM ${BUILD_FROM}
COPY --from=build /huebridge /usr/bin/huebridge
COPY run.sh /run.sh
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod a+x /run.sh /docker-entrypoint.sh

# defaultBridgePort and defaultAdminPort in cmd/huebridge/main.go — both
# configurable at runtime via HUEBRIDGE_API_PORT/HUEBRIDGE_ADMIN_PORT.
EXPOSE 8299 8300

CMD ["/docker-entrypoint.sh"]
