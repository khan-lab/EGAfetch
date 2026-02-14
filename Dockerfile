FROM golang:1.24-alpine AS builder

ARG VERSION=dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 go build -ldflags "-X main.version=${VERSION}" -o /egafetch ./cmd/egafetch

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
COPY --from=builder /egafetch /usr/local/bin/egafetch

ENTRYPOINT ["egafetch"]
