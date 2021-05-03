FROM golang:1.15 AS build

WORKDIR /go/src/github.com/dcasado/raspi-monitoring-client/

COPY main.go .

RUN CGO_ENABLED=0 GOOS=linux && go build main.go


FROM debian:10.9-slim  

WORKDIR /app

COPY --from=build /go/src/github.com/dcasado/raspi-monitoring-client/main .

ENTRYPOINT ["./main"]
