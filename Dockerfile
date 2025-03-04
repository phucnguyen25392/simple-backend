FROM arm64/golang:1.21
#FROM amd64/golang:1.21

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY *.go ./

RUN go build -v -o /app

CMD ["/app/backend"]
