# syntax=docker/dockerfile:1

FROM golang:alpine AS build

ARG VERSION=v1.0.0
ARG BUILD=0

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION} -X main.build=${BUILD} -X main.buildTime=${BUILD}" \
    -o /out/2panel .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /out/2panel /usr/local/bin/2panel
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/2panel"]
CMD ["-data", "/data"]