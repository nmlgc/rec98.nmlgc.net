#include <emscripten.h>
#include "decoder.h"

#define EXPORT extern "C" EMSCRIPTEN_KEEPALIVE

struct mz_inflate_state {
	tinfl_decompressor m_decomp;
	mz_uint m_dict_ofs, m_dict_avail, m_first_call, m_has_flushed;
	int m_window_bits;
	mz_uint8 m_dict[TINFL_LZ_DICT_SIZE];
	tinfl_status m_last_status;
};

struct buffer_description {
	void *ptr;
	int size;
};

struct decoder_info {
	struct buffer_description palette;
	struct buffer_description frame;
	struct buffer_description zlib;
	zmbv_format_t format;
} decoder_info;

VideoDecoder decoder;

struct decoder_info *VideoDecoder::GetInfo(void)
{
	decoder_info.palette.ptr = palette;
	decoder_info.palette.size = sizeof(palette);
	decoder_info.frame.ptr = newframe;
	decoder_info.frame.size = bufsize;
	decoder_info.zlib.ptr = (struct mz_inflate_state *)zstream.state;
	decoder_info.zlib.size = sizeof(struct mz_inflate_state);
	decoder_info.format = format;
	return &decoder_info;
}

EXPORT
bool setup(int width, int height)
{
	return decoder.SetupDecompress(width, height);
}

EXPORT
struct decoder_info *get_info(void)
{
	return decoder.GetInfo();
}

EXPORT
bool decode_frame(void *frame, int size)
{
	return decoder.DecompressFrame(frame, size);
}
