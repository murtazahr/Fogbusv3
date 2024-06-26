FROM golang:1.17-alpine

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN go build -o /Fogbusv3 ./cmd/node

EXPOSE 8080

ENTRYPOINT ["/Fogbusv3"]
CMD ["--type", "fog", "--listen", "/ip4/0.0.0.0/tcp/8080", "--fabric-config", "/app/config.yaml"]