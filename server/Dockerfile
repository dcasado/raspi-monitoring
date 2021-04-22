FROM golang:1.15 AS build

WORKDIR /go/src/github.com/dcasado/raspi-monitoring-server/

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates

RUN go get github.com/mattn/go-sqlite3

COPY main.go .

RUN CGO_ENABLED=1 GOOS=linux && go build main.go


FROM debian:10.9-slim  

ENV DB_FILE_PATH=/app/data/db.sqlite3
ENV USER monitoring
ENV GROUP monitoring
ENV GID 1010
ENV UID 1010

RUN groupadd -g ${GID} ${GROUP} && useradd -m -u ${UID} -g ${GROUP} ${USER}

USER ${USER}

WORKDIR /home/${USER}
RUN mkdir data

COPY --from=build --chown=${USER}:${GROUP} /go/src/github.com/dcasado/raspi-monitoring-server/main .
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY static ./static
COPY --chown=${USER}:${GROUP} entrypoint.sh .
RUN chmod u+x entrypoint.sh

EXPOSE 80

ENTRYPOINT ["./entrypoint.sh"]