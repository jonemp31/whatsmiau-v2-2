FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install gcc and SQLite dev libraries
RUN apk add build-base sqlite-dev gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Enable CGO
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -a -o /app/whatsmiau .

FROM alpine:latest

RUN apk update && apk add --no-cache ffmpeg mailcap vips-dev

WORKDIR /app

COPY --from=builder /app/whatsmiau /app/whatsmiau
COPY logo.png .
COPY env.example .env

EXPOSE 8080

CMD [ "/app/whatsmiau" ]