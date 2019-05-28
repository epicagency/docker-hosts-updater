FROM golang:alpine as build

ENV GOPATH /go
ENV GOBIN=$GOPATH/bin\
    PATH=$PATH:$GOBIN

WORKDIR /usr/local/app

RUN apk add --no-cache git gcc g++ make --virtual .build-deps

COPY main.go main.go

RUN go get \
    && go build main.go \
    && cp main /usr/local/bin/grachev-dhu

FROM alpine:3.7

COPY --from=build /usr/local/bin/grachev-dhu /usr/local/bin/grachev-dhu

ENTRYPOINT ["grachev-dhu"]
