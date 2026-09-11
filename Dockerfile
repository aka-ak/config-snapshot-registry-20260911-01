FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . ./
RUN go test ./... && go build -o /out/snapshot-registry ./cmd/server

FROM alpine:3.20
RUN mkdir -p /data
COPY --from=build /out/snapshot-registry /usr/local/bin/snapshot-registry
ENV STATE_FILE=/data/state.json
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/snapshot-registry"]
