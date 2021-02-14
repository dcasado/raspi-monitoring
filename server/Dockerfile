FROM golang:1.14.1 AS build

WORKDIR /go/src/github.com/dcasado/raspi-monitoring-server/

COPY main.go .

RUN CGO_ENABLED=0 GOOS=linux && go build main.go


FROM debian:10.3-slim  

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates

RUN update-ca-certificates

WORKDIR /app

COPY --from=build /go/src/github.com/dcasado/raspi-monitoring-server/main .
COPY static ./static
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

EXPOSE 80

ENTRYPOINT ["./entrypoint.sh"]