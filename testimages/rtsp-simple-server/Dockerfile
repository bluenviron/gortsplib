####################################
FROM amd64/golang:1.15-alpine3.12 AS server

RUN apk --no-cache add git

RUN git clone -b v0.14.2 https://github.com/aler9/rtsp-simple-server \
    && cd rtsp-simple-server \
    && CGO_ENABLED=0 \
    go build -o /rtsp-simple-server .

############################
FROM alpine:3.12

COPY --from=server /rtsp-simple-server /

COPY start.sh /
RUN chmod +x /start.sh

ENTRYPOINT [ "/start.sh" ]
