FROM golang:1.23.4-alpine as build

WORKDIR /app

COPY go.mod go.sum main.go .

RUN go mod download && \
    go build -o ./serverless-discord-bot

FROM scratch

COPY --from=build /app/serverless-discord-bot /app/serverless-discord-bot
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

CMD ["/app/serverless-discord-bot"]