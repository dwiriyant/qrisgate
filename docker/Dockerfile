FROM golang:1.26-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/qrisgate-api ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /bin/qrisgate-api /usr/local/bin/qrisgate-api
COPY db/migrations /db/migrations
WORKDIR /
EXPOSE 8080 9090
ENTRYPOINT ["qrisgate-api"]
