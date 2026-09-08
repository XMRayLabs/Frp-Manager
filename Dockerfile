FROM alpine:latest

ARG ARCH

RUN apk add --no-cache bash ca-certificates curl sqlite && update-ca-certificates

ENV TZ=UTC0

WORKDIR /app
COPY ./frp-manager-${ARCH} /app/frp-manager

RUN mkdir -p /data

# web port
EXPOSE 9000

# rpc port
EXPOSE 9001

ENV DB_DSN=/data/data.db?_pragma=journal_mode(WAL)

ENTRYPOINT [ "/app/frp-manager" ]

CMD [ "master" ]
