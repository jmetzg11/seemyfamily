FROM golang:1.26.1-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /web ./cmd/web

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

COPY --from=build /web /usr/local/bin/web

EXPOSE 4000

CMD ["web"]
