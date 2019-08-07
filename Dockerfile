ARG NAME=cloudfront-broker
ARG VERSION=0.1
ARG OSB_VERSION=2.13

FROM golang:1.12-alpine as builder

RUN apk update && \
    apk add git \
        bash

ARG NAME
ARG VERSION
ARG OSB_VERSION

ENV NAME=${NAME}
ENV VERSION=${VERSION}
ENV OSB_VERSION=${OSB_VERSION}

ENV GO111MODULE=on

WORKDIR /go/src/github.com/akkeris/${NAME}

RUN go get -u golang.org/x/lint/golint

COPY . .
COPY .git .

RUN go get

RUN	go mod tidy && \
    golint ${NAME}.go && \
	golint pkg/...

RUN GIT_COMMIT=$(git rev-parse HEAD) && \
    GO_VERSION=$(go version | sed 's/^go version go\(\([0-9]*\.[0-9]*\)*\).*$/\1/') && \
    BUILT=$(date +"%F-%I:%M:%S%z") && \
    env && \
    go build -i -ldflags "-X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT} -X main.GoVersion=${GO_VERSION} -X main.Built=${BUILT} -X main.OSBVersion=${OSB_VERSION}" -o ${NAME} ${NAME}.go

FROM alpine:3.9

ARG NAME
ENV NAME=${NAME}

RUN apk update && \
    apk add \
        openssl \
        bash \
        binutils \
        ca-certificates

WORKDIR /app

COPY --from=builder /go/src/github.com/akkeris/${NAME}/${NAME} /app/${NAME}
COPY start.sh .
COPY start-tasks.sh .

CMD /app/${NAME} -insecure -logtostderr=1 -stderrthreshold 0

