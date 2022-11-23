#pragma once

#include "zmbv.h"

struct decoder_info;

class VideoDecoder : public VideoCodec
{
public:
	struct decoder_info *GetInfo(void);
};
