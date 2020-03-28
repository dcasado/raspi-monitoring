FROM golang:1.14.1 AS build

WORKDIR /go/src/github.com/dcasado/raspi-monitoring-client/

COPY main.go .

RUN CGO_ENABLED=0 GOOS=linux && go build main.go


FROM debian:10.3-slim  

WORKDIR /app

COPY --from=build /go/src/github.com/dcasado/raspi-monitoring-client/main .

VOLUME [ "/sys/class/thermal/thermal_zone0/temp" ]

ENTRYPOINT ["./main"]
