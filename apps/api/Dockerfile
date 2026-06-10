# syntax=docker/dockerfile:1

FROM ccr.ccs.tencentyun.com/k3s-platform/golang:1.22-alpine AS builder

ENV GOPROXY=https://goproxy.cn,direct
ENV GO111MODULE=on

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/forum-api ./cmd/server

FROM ccr.ccs.tencentyun.com/k3s-platform/distroless-static-debian12:nonroot

WORKDIR /

COPY --from=builder /out/forum-api /forum-api

EXPOSE 8080

USER nonroot:nonroot
ENTRYPOINT ["/forum-api"]
