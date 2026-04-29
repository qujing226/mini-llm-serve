FROM golang:1.26 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/mini-llm-server ./cmd/server

FROM alpine:latest

WORKDIR /app
COPY --from=build /out/mini-llm-server /app/mini-llm-server
EXPOSE 8800 8801
ENTRYPOINT ["/app/mini-llm-server"]
