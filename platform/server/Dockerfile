FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/platform-server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/platform-server /usr/local/bin/platform-server
COPY configs/service-catalog.yaml /app/configs/service-catalog.yaml
USER nonroot:nonroot
ENTRYPOINT ["platform-server"]
