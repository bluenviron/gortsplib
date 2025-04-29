# RTSP-over-HTTP Tunneling Example

This example demonstrates how to connect to a RTSP server using RTSP-over-HTTP tunneling according to the Apple standard.

## How RTSP-over-HTTP Works

RTSP-over-HTTP tunneling is a mechanism to allow RTSP traffic to pass through firewalls and web proxies that only allow HTTP traffic. The Apple standard uses two HTTP connections:

1. A long-running HTTP GET connection for receiving data from the server
2. A long-running HTTP POST connection for sending data to the server

The RTSP protocol messages and RTP/RTCP packets are base64-encoded and sent over these HTTP connections.

Both HTTP and HTTPS are supported, with the latter providing encrypted communications. When using HTTPS, the client's TLS configuration can be customized using the `TLSConfig` field.

## Key Implementation Details

- The HTTP tunneling uses a session cookie to match the GET and POST connections
- RTSP commands and media data are interleaved and base64-encoded
- The POST connection uses chunked transfer encoding to maintain a long-running HTTP request
- Raw TCP sockets are used for better control over the HTTP protocol
- Base64 decoding is handled carefully to manage partial data chunks
- The client transparently handles the tunneling, allowing the same API to be used regardless of the transport

## Usage

```
go run main.go <rtsp url>
```

Example:
```
go run main.go rtsp://example.com:554/stream
```

## Requirements

- A RTSP server that supports RTSP-over-HTTP tunneling
- Go 1.16 or later

## Testing

You can test this implementation with various RTSP servers that support HTTP tunneling, including:
- Wowza Streaming Engine
- Apple Darwin Streaming Server
- Some IP cameras that support ONVIF standards

## Implementation Note

This implementation uses raw TCP sockets instead of Go's HTTP client to avoid buffering issues with long-running HTTP connections. This allows for:

1. Direct control over the HTTP protocol
2. Efficient handling of base64-encoded data
3. Proper management of partially received data between reads
4. Reliable chunked transfer encoding for continuous data streaming