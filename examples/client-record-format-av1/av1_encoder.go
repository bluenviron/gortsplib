package main

import (
	"fmt"
	"image"
	"log"
	"unsafe"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/av1"
)

// #cgo pkg-config: libavcodec libavutil libswscale
// #include <libavcodec/avcodec.h>
// #include <libswscale/swscale.h>
// #include <libavutil/opt.h>
import "C"

func frameData(frame *C.AVFrame) **C.uint8_t {
	return (**C.uint8_t)(unsafe.Pointer(&frame.data[0]))
}

func frameLineSize(frame *C.AVFrame) *C.int {
	return (*C.int)(unsafe.Pointer(&frame.linesize[0]))
}

// av1Encoder is a wrapper around FFmpeg's AV1 encoder.
type av1Encoder struct {
	Width  int
	Height int
	FPS    int

	codecCtx    *C.AVCodecContext
	rgbaFrame   *C.AVFrame
	yuv420Frame *C.AVFrame
	swsCtx      *C.struct_SwsContext
	pkt         *C.AVPacket
}

// initialize initializes a av1Encoder.
func (d *av1Encoder) initialize() error {
	// prefer svtav1 over libaom-av1
	var codec *C.AVCodec
	for _, lib := range []string{"libsvtav1", "libaom-av1"} {
		str := C.CString(lib)
		defer C.free(unsafe.Pointer(str))
		codec = C.avcodec_find_encoder_by_name(str)
		if codec != nil {
			if lib == "libaom-av1" {
				log.Println("WARNING: using libaom-av1 - encoding will be very slow. Compile FFmpeg against libsvtav1 to speed things up.")
			}
			break
		}
	}
	if codec == nil {
		return fmt.Errorf("avcodec_find_encoder_by_name() failed")
	}

	d.codecCtx = C.avcodec_alloc_context3(codec)
	if d.codecCtx == nil {
		return fmt.Errorf("avcodec_alloc_context3() failed")
	}

	key := C.CString("preset")
	defer C.free(unsafe.Pointer(key))
	val := C.CString("8")
	defer C.free(unsafe.Pointer(val))
	C.av_opt_set(d.codecCtx.priv_data, key, val, 0)

	d.codecCtx.pix_fmt = C.AV_PIX_FMT_YUV420P
	d.codecCtx.width = (C.int)(d.Width)
	d.codecCtx.height = (C.int)(d.Height)
	d.codecCtx.time_base.num = 1
	d.codecCtx.time_base.den = (C.int)(d.FPS)
	d.codecCtx.gop_size = 10
	d.codecCtx.max_b_frames = 0
	d.codecCtx.bit_rate = 600000

	res := C.avcodec_open2(d.codecCtx, codec, nil)
	if res < 0 {
		C.avcodec_close(d.codecCtx)
		return fmt.Errorf("avcodec_open2() failed")
	}

	d.rgbaFrame = C.av_frame_alloc()
	if d.rgbaFrame == nil {
		C.avcodec_close(d.codecCtx)
		return fmt.Errorf("av_frame_alloc() failed")
	}

	d.rgbaFrame.format = C.AV_PIX_FMT_RGBA
	d.rgbaFrame.width = d.codecCtx.width
	d.rgbaFrame.height = d.codecCtx.height

	res = C.av_frame_get_buffer(d.rgbaFrame, 0)
	if res < 0 {
		C.av_frame_free(&d.rgbaFrame)
		C.avcodec_close(d.codecCtx)
		return fmt.Errorf("av_frame_get_buffer() failed")
	}

	d.yuv420Frame = C.av_frame_alloc()
	if d.rgbaFrame == nil {
		C.av_frame_free(&d.rgbaFrame)
		C.avcodec_close(d.codecCtx)
		return fmt.Errorf("av_frame_alloc() failed")
	}

	d.yuv420Frame.format = C.AV_PIX_FMT_YUV420P
	d.yuv420Frame.width = d.codecCtx.width
	d.yuv420Frame.height = d.codecCtx.height

	res = C.av_frame_get_buffer(d.yuv420Frame, 0)
	if res < 0 {
		C.av_frame_free(&d.yuv420Frame)
		C.av_frame_free(&d.rgbaFrame)
		C.avcodec_close(d.codecCtx)
		return fmt.Errorf("av_frame_get_buffer() failed")
	}

	d.swsCtx = C.sws_getContext(d.rgbaFrame.width, d.rgbaFrame.height, (int32)(d.rgbaFrame.format),
		d.yuv420Frame.width, d.yuv420Frame.height, (int32)(d.yuv420Frame.format), C.SWS_BILINEAR, nil, nil, nil)
	if d.swsCtx == nil {
		C.av_frame_free(&d.yuv420Frame)
		C.av_frame_free(&d.rgbaFrame)
		C.avcodec_close(d.codecCtx)
		return fmt.Errorf("sws_getContext() failed")
	}

	d.pkt = C.av_packet_alloc()
	if d.pkt == nil {
		C.av_frame_free(&d.yuv420Frame)
		C.av_frame_free(&d.rgbaFrame)
		C.avcodec_close(d.codecCtx)
		return fmt.Errorf("av_packet_alloc() failed")
	}

	return nil
}

// close closes the decoder.
func (d *av1Encoder) close() {
	C.av_packet_free(&d.pkt)
	C.sws_freeContext(d.swsCtx)
	C.av_frame_free(&d.yuv420Frame)
	C.av_frame_free(&d.rgbaFrame)
	C.avcodec_close(d.codecCtx)
}

// encode encodes a RGBA image into AV1.
func (d *av1Encoder) encode(img *image.RGBA, pts int64) ([][]byte, int64, error) {
	// pass image pointer to frame
	d.rgbaFrame.data[0] = (*C.uint8_t)(&img.Pix[0])

	// convert color space from RGBA to YUV420
	res := C.sws_scale(d.swsCtx, frameData(d.rgbaFrame), frameLineSize(d.rgbaFrame),
		0, d.rgbaFrame.height, frameData(d.yuv420Frame), frameLineSize(d.yuv420Frame))
	if res < 0 {
		return nil, 0, fmt.Errorf("sws_scale() failed")
	}

	// send frame to the encoder
	d.yuv420Frame.pts = (C.int64_t)(pts)
	res = C.avcodec_send_frame(d.codecCtx, d.yuv420Frame)
	if res < 0 {
		return nil, 0, fmt.Errorf("avcodec_send_frame() failed")
	}

	// wait for result
	res = C.avcodec_receive_packet(d.codecCtx, d.pkt)
	if res == -C.EAGAIN {
		return nil, 0, nil
	}
	if res < 0 {
		return nil, 0, fmt.Errorf("avcodec_receive_packet() failed")
	}

	// perform a deep copy of the data before unreferencing the packet
	data := C.GoBytes(unsafe.Pointer(d.pkt.data), d.pkt.size)
	pts = (int64)(d.pkt.pts)
	C.av_packet_unref(d.pkt)

	// decompress
	var bs av1.Bitstream
	err := bs.Unmarshal(data)
	if err != nil {
		return nil, 0, err
	}

	return bs, pts, nil
}
