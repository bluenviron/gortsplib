package main

import (
	"fmt"
	"unsafe"
)

// #cgo pkg-config: libavcodec libavutil
// #include <libavcodec/avcodec.h>
import "C"

func frameData(frame *C.AVFrame) **C.uint8_t {
	return (**C.uint8_t)(unsafe.Pointer(&frame.data[0]))
}

func frameLineSize(frame *C.AVFrame) *C.int {
	return (*C.int)(unsafe.Pointer(&frame.linesize[0]))
}

func switchEndianness16(samples []byte) []byte {
	ls := len(samples)
	for i := 0; i < ls; i += 2 {
		samples[i], samples[i+1] = samples[i+1], samples[i]
	}
	return samples
}

// opusEncoder is a wrapper around FFmpeg's Opus encoder.
type opusEncoder struct {
	Width  int
	Height int
	FPS    int

	codecCtx         *C.AVCodecContext
	frame            *C.AVFrame
	pkt              *C.AVPacket
	samplesBuffer    []byte
	samplesBufferPTS int64
}

// initialize initializes a opusEncoder.
func (d *opusEncoder) initialize() error {
	codec := C.avcodec_find_encoder(C.AV_CODEC_ID_OPUS)
	if codec == nil {
		return fmt.Errorf("avcodec_find_encoder() failed")
	}

	d.codecCtx = C.avcodec_alloc_context3(codec)
	if d.codecCtx == nil {
		return fmt.Errorf("avcodec_alloc_context3() failed")
	}

	d.codecCtx.bit_rate = 64000
	d.codecCtx.sample_fmt = C.AV_SAMPLE_FMT_S16
	d.codecCtx.sample_rate = 48000
	d.codecCtx.channel_layout = C.AV_CH_LAYOUT_MONO
	d.codecCtx.channels = C.av_get_channel_layout_nb_channels(d.codecCtx.channel_layout)

	res := C.avcodec_open2(d.codecCtx, codec, nil)
	if res < 0 {
		C.avcodec_close(d.codecCtx)
		return fmt.Errorf("avcodec_open2() failed")
	}

	d.frame = C.av_frame_alloc()
	if d.frame == nil {
		C.avcodec_close(d.codecCtx)
		return fmt.Errorf("av_frame_alloc() failed")
	}

	d.frame.nb_samples = d.codecCtx.frame_size
	d.frame.format = (C.int)(d.codecCtx.sample_fmt)
	d.frame.channel_layout = d.codecCtx.channel_layout

	res = C.av_frame_get_buffer(d.frame, 0)
	if res < 0 {
		C.av_frame_free(&d.frame)
		C.avcodec_close(d.codecCtx)
		return fmt.Errorf("av_frame_get_buffer() failed")
	}

	d.pkt = C.av_packet_alloc()
	if d.pkt == nil {
		C.av_frame_free(&d.frame)
		C.avcodec_close(d.codecCtx)
		return fmt.Errorf("av_packet_alloc() failed")
	}

	return nil
}

// close closes the decoder.
func (d *opusEncoder) close() {
	C.av_packet_free(&d.pkt)
	C.av_frame_free(&d.frame)
	C.avcodec_close(d.codecCtx)
}

// encode encodes LPCM samples into Opus packets.
func (d *opusEncoder) encode(samples []byte) ([][]byte, int64, error) {
	// convert from big-endian to little-endian
	samples = switchEndianness16(samples)

	// put samples into an internal buffer
	d.samplesBuffer = append(d.samplesBuffer, samples...)

	// split buffer into AVFrames
	requiredSampleSize := (int)(d.codecCtx.frame_size) * 2
	frameCount := len(d.samplesBuffer) / requiredSampleSize
	if frameCount == 0 {
		return nil, 0, fmt.Errorf("sample buffer is not filled enough")
	}

	ret := make([][]byte, frameCount)
	var pts int64

	for i := 0; i < frameCount; i++ {
		samples = d.samplesBuffer[:requiredSampleSize]
		d.samplesBuffer = d.samplesBuffer[requiredSampleSize:]

		samplePTS := d.samplesBufferPTS
		d.samplesBufferPTS += int64(len(samples) / 2)

		// pass samples pointer to frame
		d.frame.data[0] = (*C.uint8_t)(&samples[0])

		// send frame to the encoder
		d.frame.pts = (C.int64_t)(samplePTS)
		res := C.avcodec_send_frame(d.codecCtx, d.frame)
		if res < 0 {
			return nil, 0, fmt.Errorf("avcodec_send_frame() failed")
		}

		// wait for result
		res = C.avcodec_receive_packet(d.codecCtx, d.pkt)
		if res < 0 {
			return nil, 0, fmt.Errorf("avcodec_receive_packet() failed")
		}

		// perform a deep copy of the data before unreferencing the packet
		data := C.GoBytes(unsafe.Pointer(d.pkt.data), d.pkt.size)

		if i == 0 {
			pts = (int64)(d.pkt.pts)
		}

		C.av_packet_unref(d.pkt)

		ret[i] = data
	}

	return ret, pts, nil
}
