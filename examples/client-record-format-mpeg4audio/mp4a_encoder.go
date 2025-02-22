package main

import (
	"fmt"
	"runtime"
	"unsafe"
)

// #cgo pkg-config: libavcodec libavutil libswresample
// #include <libavcodec/avcodec.h>
// #include <libswresample/swresample.h>
// #include <libavutil/opt.h>
// #include <libavutil/channel_layout.h>
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

func littleEndianToFloat(swrCtx *C.struct_SwrContext, samples []byte) ([]byte, error) {
	sampleCount := len(samples) / 2
	outSize := len(samples) * 2
	outSamples := make([]byte, outSize)

	var p runtime.Pinner
	p.Pin(&outSamples[0])
	p.Pin(&samples[0])
	defer p.Unpin()

	outBufs := (*C.uint8_t)(&outSamples[0])
	inBufs := (*C.uint8_t)(&samples[0])

	res := C.swr_convert(swrCtx, &outBufs, (C.int)(sampleCount), &inBufs, (C.int)(sampleCount))
	if res < 0 {
		return nil, fmt.Errorf("swr_convert() failed")
	}

	return outSamples, nil
}

// mp4aEncoder is a wrapper around FFmpeg's MPEG-4 Audio encoder.
type mp4aEncoder struct {
	Width  int
	Height int
	FPS    int

	codecCtx         *C.AVCodecContext
	frame            *C.AVFrame
	swrCtx           *C.struct_SwrContext
	pkt              *C.AVPacket
	samplesBuffer    []byte
	samplesBufferPTS int64
}

// initialize initializes a mp4aEncoder.
func (d *mp4aEncoder) initialize() error {
	codec := C.avcodec_find_encoder(C.AV_CODEC_ID_AAC)
	if codec == nil {
		return fmt.Errorf("avcodec_find_encoder() failed")
	}

	d.codecCtx = C.avcodec_alloc_context3(codec)
	if d.codecCtx == nil {
		return fmt.Errorf("avcodec_alloc_context3() failed")
	}

	d.codecCtx.bit_rate = 64000
	d.codecCtx.sample_fmt = C.AV_SAMPLE_FMT_FLT
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

	d.swrCtx = C.swr_alloc()
	if d.swrCtx == nil {
		C.av_frame_free(&d.frame)
		C.avcodec_close(d.codecCtx)
		return fmt.Errorf("swr_alloc() failed")
	}

	cstr := C.CString("out_channel_layout")
	defer C.free(unsafe.Pointer(cstr))
	C.av_opt_set_channel_layout(unsafe.Pointer(d.swrCtx), cstr, (C.int64_t)(d.codecCtx.channel_layout), 0)

	cstr = C.CString("out_sample_fmt")
	defer C.free(unsafe.Pointer(cstr))
	C.av_opt_set_int(unsafe.Pointer(d.swrCtx), cstr, C.AV_SAMPLE_FMT_FLTP, 0)

	cstr = C.CString("out_sample_rate")
	defer C.free(unsafe.Pointer(cstr))
	C.av_opt_set_int(unsafe.Pointer(d.swrCtx), cstr, 48000, 0)

	cstr = C.CString("in_channel_layout")
	defer C.free(unsafe.Pointer(cstr))
	C.av_opt_set_channel_layout(unsafe.Pointer(d.swrCtx), cstr, (C.int64_t)(d.codecCtx.channel_layout), 0)

	cstr = C.CString("in_sample_fmt")
	defer C.free(unsafe.Pointer(cstr))
	C.av_opt_set_int(unsafe.Pointer(d.swrCtx), cstr, C.AV_SAMPLE_FMT_S16, 0)

	cstr = C.CString("in_sample_rate")
	defer C.free(unsafe.Pointer(cstr))
	C.av_opt_set_int(unsafe.Pointer(d.swrCtx), cstr, 48000, 0)

	res = C.swr_init(d.swrCtx)
	if res < 0 {
		C.swr_free(&d.swrCtx)
		C.av_frame_free(&d.frame)
		C.avcodec_close(d.codecCtx)
		return fmt.Errorf("swr_init() failed")
	}

	d.pkt = C.av_packet_alloc()
	if d.pkt == nil {
		C.swr_free(&d.swrCtx)
		C.av_frame_free(&d.frame)
		C.avcodec_close(d.codecCtx)
		return fmt.Errorf("av_packet_alloc() failed")
	}

	return nil
}

// close closes the decoder.
func (d *mp4aEncoder) close() {
	C.av_packet_free(&d.pkt)
	C.swr_free(&d.swrCtx)
	C.av_frame_free(&d.frame)
	C.avcodec_close(d.codecCtx)
}

// encode encodes LPCM samples into Opus packets.
func (d *mp4aEncoder) encode(samples []byte) ([][]byte, int64, error) {
	// convert from big-endian to little-endian
	samples = switchEndianness16(samples)

	// convert from little-endian to float
	samples, err := littleEndianToFloat(d.swrCtx, samples)
	if err != nil {
		return nil, 0, err
	}

	// put samples into an internal buffer
	d.samplesBuffer = append(d.samplesBuffer, samples...)

	// split buffer into AVFrames
	requiredSampleSize := (int)(d.codecCtx.frame_size) * 4
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
		d.samplesBufferPTS += int64(len(samples) / 4)

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
		if res == -C.EAGAIN {
			return nil, 0, nil
		}
		if res < 0 {
			fmt.Println(res)
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
