FROM alpine:3.22

RUN apk add --no-cache \
    gstreamer-tools \
    gst-plugins-bad \
    gst-plugins-good  \
    gst-rtsp-server \
    gst-libav

COPY emptyvideo.mkv /

COPY start.sh /
RUN chmod +x /start.sh

ENTRYPOINT [ "/start.sh" ]
