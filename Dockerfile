FROM golang:1.20-alpine3.17 AS build

ENV GOPROXY=https://goproxy.cn,direct

RUN mkdir /hackernews-feed
COPY . /hackernews-feed
WORKDIR /hackernews-feed

RUN go build -o hackernews-feed .

FROM alpine:3.17
COPY --from=build /hackernews-feed/hackernews-feed /hackernews-feed/hackernews-feed

CMD ["/hackernews-feed/hackernews-feed"]