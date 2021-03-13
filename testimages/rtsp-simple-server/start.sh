#!/bin/sh -e

echo "$@" > /rtsp-simple-server.yml

exec /rtsp-simple-server
