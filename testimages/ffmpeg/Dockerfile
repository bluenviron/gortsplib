FROM alpine:3.14

RUN apk add --no-cache \
    ffmpeg

COPY emptyvideo.mkv /

COPY start.sh /
RUN chmod +x /start.sh

ENTRYPOINT [ "/start.sh" ]
