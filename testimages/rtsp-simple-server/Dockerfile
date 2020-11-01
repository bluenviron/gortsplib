############################
FROM aler9/rtsp-simple-server:latest AS server
############################
FROM alpine:3.12

COPY --from=server /rtsp-simple-server /

COPY start.sh /
RUN chmod +x /start.sh

ENTRYPOINT [ "/start.sh" ]
