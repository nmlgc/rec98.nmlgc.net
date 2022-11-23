#include <emscripten.h>
#include "decoder.h"

#define EXPORT extern "C" EMSCRIPTEN_KEEPALIVE

VideoCodec codec;

EXPORT
bool setup(int width, int height)
{
	return codec.SetupDecompress(width, height);
}

EXPORT
bool decode_frame(void *frame, int size)
{
	return codec.DecompressFrame(frame, size);
}
