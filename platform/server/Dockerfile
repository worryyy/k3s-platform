# syntax=docker/dockerfile:1

FROM golang:1.26 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/platform-server \
    ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=build /out/platform-server /usr/local/bin/platform-server

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/platform-server"]