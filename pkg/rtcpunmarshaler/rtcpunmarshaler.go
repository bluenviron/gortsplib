// Package rtcpunmarshaler contains a RTCP unmarshaler compatible with most RTSP implementations.
package rtcpunmarshaler

import (
	"encoding/binary"

	"github.com/pion/rtcp"
)

const headerLength = 4

func unmarshalAllowMissingSDESEnd(rawData []byte) ([]rtcp.Packet, bool) {
	var packets []rtcp.Packet

	for len(rawData) != 0 {
		var header rtcp.Header
		err := header.Unmarshal(rawData)
		if err != nil {
			return nil, false
		}

		bytesProcessed := int(header.Length+1) * 4
		if bytesProcessed > len(rawData) {
			return nil, false
		}

		inPacket := rawData[:bytesProcessed]
		rawData = rawData[bytesProcessed:]

		if header.Type != rtcp.TypeSourceDescription {
			var packet []rtcp.Packet
			packet, err = rtcp.Unmarshal(inPacket)
			if err != nil || len(packet) != 1 {
				return nil, false
			}

			packets = append(packets, packet[0])
			continue
		}

		packet, ok := unmarshalSourceDescriptionAllowMissingEnd(inPacket, header)
		if !ok {
			return nil, false
		}

		packets = append(packets, packet)
	}

	if len(packets) == 0 {
		return nil, false
	}

	return packets, true
}

func unmarshalSourceDescriptionAllowMissingEnd(rawPacket []byte, header rtcp.Header) (rtcp.Packet, bool) {
	if header.Count != 1 || len(rawPacket) < headerLength+4 {
		return nil, false
	}

	body := rawPacket[headerLength:]
	chunks := make([]rtcp.SourceDescriptionChunk, 0, header.Count)

	for range int(header.Count) {
		if len(body) < 4 {
			return nil, false
		}

		var chunk rtcp.SourceDescriptionChunk
		chunk.Source = binary.BigEndian.Uint32(body)
		body = body[4:]

		for {
			if len(body) == 0 {
				return nil, false
			}

			if body[0] == 0 {
				body = body[1:]
				for len(body) > 0 && body[0] == 0 {
					body = body[1:]
				}
				break
			}

			if len(body) < 2 {
				return nil, false
			}

			itemLen := 2 + int(body[1])
			if itemLen > len(body) {
				return nil, false
			}

			if itemLen == len(body) {
				var item rtcp.SourceDescriptionItem
				err := item.Unmarshal(body)
				if err != nil {
					return nil, false
				}

				chunk.Items = append(chunk.Items, item)
				body = body[:0]
				break
			}

			var item rtcp.SourceDescriptionItem
			err := item.Unmarshal(body[:itemLen])
			if err != nil {
				return nil, false
			}

			chunk.Items = append(chunk.Items, item)
			body = body[itemLen:]
		}

		chunks = append(chunks, chunk)
	}

	if len(body) != 0 {
		return nil, false
	}

	return &rtcp.SourceDescription{Chunks: chunks}, true
}

// Unmarshal is a wrapper around pion/rtcp.Unmarshal that tolerates some malformed packets.
func Unmarshal(rawData []byte) ([]rtcp.Packet, error) {
	packets, err := rtcp.Unmarshal(rawData)
	if err == nil {
		return packets, nil
	}

	packets, ok := unmarshalAllowMissingSDESEnd(rawData)
	if ok {
		return packets, nil
	}

	return nil, err
}
