package main

import (
	"fmt"
	"image"
	"runtime"
	"unsafe"
)

// #cgo pkg-config: libavcodec libavutil libswscale
// #include <libavcodec/avcodec.h>
// #include <libavutil/imgutils.h>
// #include <libswscale/swscale.h>
import "C"

func frameData(frame *C.AVFrame) **C.uint8_t {
	return (**C.uint8_t)(unsafe.Pointer(&frame.data[0]))
}

func frameLineSize(frame *C.AVFrame) *C.int {
	return (*C.int)(unsafe.Pointer(&frame.linesize[0]))
}

// vp8Decoder is a wrapper around FFmpeg's VP8 decoder.
type vp8Decoder struct {
	codecCtx     *C.AVCodecContext
	yuv420Frame  *C.AVFrame
	rgbaFrame    *C.AVFrame
	rgbaFramePtr []uint8
	swsCtx       *C.struct_SwsContext
}

// initialize initializes a vp8Decoder.
func (d *vp8Decoder) initialize() error {
	codec := C.avcodec_find_decoder(C.AV_CODEC_ID_VP8)
	if codec == nil {
		return fmt.Errorf("avcodec_find_decoder() failed")
	}

	d.codecCtx = C.avcodec_alloc_context3(codec)
	if d.codecCtx == nil {
		return fmt.Errorf("avcodec_alloc_context3() failed")
	}

	res := C.avcodec_open2(d.codecCtx, codec, nil)
	if res < 0 {
		C.avcodec_close(d.codecCtx)
		return fmt.Errorf("avcodec_open2() failed")
	}

	d.yuv420Frame = C.av_frame_alloc()
	if d.yuv420Frame == nil {
		C.avcodec_close(d.codecCtx)
		return fmt.Errorf("av_frame_alloc() failed")
	}

	return nil
}

// close closes the decoder.
func (d *vp8Decoder) close() {
	if d.swsCtx != nil {
		C.sws_freeContext(d.swsCtx)
	}

	if d.rgbaFrame != nil {
		C.av_frame_free(&d.rgbaFrame)
	}

	C.av_frame_free(&d.yuv420Frame)
	C.avcodec_close(d.codecCtx)
}

func (d *vp8Decoder) reinitDynamicStuff() error {
	if d.swsCtx != nil {
		C.sws_freeContext(d.swsCtx)
	}

	if d.rgbaFrame != nil {
		C.av_frame_free(&d.rgbaFrame)
	}

	d.rgbaFrame = C.av_frame_alloc()
	if d.rgbaFrame == nil {
		return fmt.Errorf("av_frame_alloc() failed")
	}

	d.rgbaFrame.format = C.AV_PIX_FMT_RGBA
	d.rgbaFrame.width = d.yuv420Frame.width
	d.rgbaFrame.height = d.yuv420Frame.height
	d.rgbaFrame.color_range = C.AVCOL_RANGE_JPEG

	res := C.av_frame_get_buffer(d.rgbaFrame, 1)
	if res < 0 {
		return fmt.Errorf("av_frame_get_buffer() failed")
	}

	d.swsCtx = C.sws_getContext(d.yuv420Frame.width, d.yuv420Frame.height, int32(d.yuv420Frame.format),
		d.rgbaFrame.width, d.rgbaFrame.height, (int32)(d.rgbaFrame.format), C.SWS_BILINEAR, nil, nil, nil)
	if d.swsCtx == nil {
		return fmt.Errorf("sws_getContext() failed")
	}

	rgbaFrameSize := C.av_image_get_buffer_size((int32)(d.rgbaFrame.format), d.rgbaFrame.width, d.rgbaFrame.height, 1)
	d.rgbaFramePtr = (*[1 << 30]uint8)(unsafe.Pointer(d.rgbaFrame.data[0]))[:rgbaFrameSize:rgbaFrameSize]
	return nil
}

// decode decodes a RGBA image from VP8.
func (d *vp8Decoder) decode(au []byte) (*image.RGBA, error) {
	// send access unit to decoder
	var pkt C.AVPacket
	ptr := &au[0]
	var p runtime.Pinner
	p.Pin(ptr)
	pkt.data = (*C.uint8_t)(ptr)
	pkt.size = (C.int)(len(au))
	res := C.avcodec_send_packet(d.codecCtx, &pkt)
	p.Unpin()
	if res < 0 {
		return nil, nil
	}

	// receive frame if available
	res = C.avcodec_receive_frame(d.codecCtx, d.yuv420Frame)
	if res < 0 {
		return nil, nil
	}

	// if frame size has changed, allocate needed objects
	if d.rgbaFrame == nil || d.rgbaFrame.width != d.yuv420Frame.width || d.rgbaFrame.height != d.yuv420Frame.height {
		err := d.reinitDynamicStuff()
		if err != nil {
			return nil, err
		}
	}

	// convert color space from YUV420 to RGBA
	res = C.sws_scale(d.swsCtx, frameData(d.yuv420Frame), frameLineSize(d.yuv420Frame),
		0, d.yuv420Frame.height, frameData(d.rgbaFrame), frameLineSize(d.rgbaFrame))
	if res < 0 {
		return nil, fmt.Errorf("sws_scale() failed")
	}

	// embed frame into an image.RGBA
	return &image.RGBA{
		Pix:    d.rgbaFramePtr,
		Stride: 4 * (int)(d.rgbaFrame.width),
		Rect: image.Rectangle{
			Max: image.Point{(int)(d.rgbaFrame.width), (int)(d.rgbaFrame.height)},
		},
	}, nil
}
