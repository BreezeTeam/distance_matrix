FROM golang:1.26 AS build
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOPROXY=https://goproxy.cn,direct
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o matrix matrix.go

FROM alpine:3.19
WORKDIR /app
COPY --from=build /app/matrix .
COPY --from=build /app/etc ./etc
EXPOSE 8888
CMD ["./matrix", "-f", "etc/matrix.docker.yaml"]
