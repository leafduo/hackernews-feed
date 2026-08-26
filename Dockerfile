FROM golang:1.27.0-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/hackernews-feed .

FROM alpine:3.24
COPY --from=build /out/hackernews-feed /usr/local/bin/hackernews-feed

ENTRYPOINT ["/usr/local/bin/hackernews-feed"]
