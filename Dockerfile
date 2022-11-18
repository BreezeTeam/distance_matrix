FROM golang:1.18 AS build
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GOPROXY="https://goproxy.cn,direct"

COPY $pwd/.. /go/src/project
WORKDIR /go/src/project

RUN go build -o matrix .

FROM alpine:latest AS runtime
COPY --from=build /go/src/project/matrix ./
COPY --from=build /go/src/project/etc/matrix.yaml ./
EXPOSE 8888/tcp
CMD ["./matrix","-f","matrix.yaml"]
