FROM arm64v8/golang:1.21

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY *.go ./

RUN go build -v -o /app

CMD ["/app/backend"]
