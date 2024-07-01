FROM golang

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . ./

EXPOSE 3000

RUN  go build -o ./binary-app ./main/main.go

CMD ["/app/binary-app"]