FROM golang:1.14.1 AS build

WORKDIR /go/src/github.com/dcasado/raspi-monitoring-server/

RUN go get github.com/mattn/go-sqlite3

COPY main.go .

RUN CGO_ENABLED=1 GOOS=linux && go build main.go


FROM debian:10.3-slim  

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates

RUN update-ca-certificates

ENV DB_FILE_PATH=/app/data/db.sqlite3

WORKDIR /app
RUN mkdir data

COPY --from=build /go/src/github.com/dcasado/raspi-monitoring-server/main .
COPY static ./static
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

EXPOSE 80

ENTRYPOINT ["./entrypoint.sh"]