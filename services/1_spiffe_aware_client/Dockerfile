FROM golang:1.15-alpine

RUN apk add git &&  mkdir -p /spiffe-aware-client /root/go/bin /root/go/src /root/go/pkg

COPY ./spiffe-aware-client.go /spiffe-aware-client/spiffe-aware-client.go

WORKDIR /spiffe-aware-client

ENV GOPATH /root/go

ENV GOBIN /usr/bin

RUN go get ./... &&  go install spiffe-aware-client.go
