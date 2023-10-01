import { Unzlib } from "fflate";

/*
 * Utilities
 */

function assert(condition: boolean, message?: string): asserts condition {
	if(!condition) {
		throw new Error(message ?? "Assertion failed");
	}
}

function assertNonNull<T>(value: T, message?: string): asserts value is NonNullable<T> {
	assert(value !== undefined && value !== null, message);
}

/*
 * Constants
 */

// ZMBV version implemented by DOSBox
const DBZV_VERSION_HIGH = 0;
const DBZV_VERSION_LOW  = 1;

// Compression algorithms
const COMPRESSION_NONE = 0;
const COMPRESSION_ZLIB = 1;

// Max distance from zero of a motion vector
const MAX_VECTOR = 16;

// Frame tag bit mask
const TAG_KEYFRAME      = 0x01;
const TAG_DELTA_PALETTE = 0x02;

// Frame data layout
const FORMAT_NONE  = 0x00;
const FORMAT_1BPP  = 0x01;
const FORMAT_2BPP  = 0x02;
const FORMAT_4BPP  = 0x03;
const FORMAT_8BPP  = 0x04;
const FORMAT_15BPP = 0x05;
const FORMAT_16BPP = 0x06;
const FORMAT_24BPP = 0x07;
const FORMAT_32BPP = 0x08;

/*
 * ZMBV decoding
 */

// From fflate
type InflateState = {
	// lmap
	l?: Uint16Array;
	// dmap
	d?: Uint16Array;
	// lbits
	m?: number;
	// dbits
	n?: number;
	// final
	f?: number;
	// pos
	p?: number;
	// byte
	b?: number;
	// lstchk
	i?: boolean;
};

// From fflate
type PrivateInflate = {
	s: InflateState;
	o: Uint8Array;
	p: Uint8Array;
	d: boolean;
};

/**
 * Stores information about the layout of a motion block as stored in the
 * decoder frame buffer.
 */
interface BlockInfo {
	/**
	 * Index of the first pixel of the block in the buffer.
	 */
	offset: number;

	/**
	 * Width of the block in pixels.
	 */
	dx: number;

	/**
	 * Height of the block in pixels.
	 */
	dy: number;
}

class ZMBVDecoder {
	private decompressor: Unzlib;
	private decompressedData = new Uint8Array();
	private compression = COMPRESSION_NONE;
	private pitch = 0;
	private blockWidth = 0;
	private blockHeight = 0;
	private blockInfo: BlockInfo[] = [];
	private oldFrame = new Uint32Array();
	private newFrame = new Uint32Array();
	private pixelSize = 4;
	private currentFrame = -1;
	private lastDecodedFrame = -1;
	private frameCache: { [key: number]: {
		frame: Uint32Array;
		state: PrivateInflate["s"];
		output: PrivateInflate["o"];
	} } = {};
	private frameCachingInterval = 0;

	constructor(private width: number, private height: number, private frames: Uint8Array[]) {
		this.decompressor = new Unzlib();
		this.decompressor.ondata = (data) => {
			// This callback is executed after every call to push()
			this.decompressedData = data;
		};
	}

	/**
	 * Initializes the frame buffers, generates block information, and enables
	 * frame caching if necessary.
	 */
	private setup() {
		// Buffers are padded by MAX_VECTOR from every side. This lets us handle
		// motion vectors going out of bounds and getting naturally zeroed out.
		// On the following diagram, the dots are the actual image, and the
		// zeros are the MAX_VECTOR padding:
		//
		// +----------------------------+
		// |0000000000000000000000000000|
		// |0000000000000000000000000000|
		// |000......................000|
		// |000......................000|
		// |000......................000|
		// |000......................000|
		// |000......................000|
		// |0000000000000000000000000000|
		// |0000000000000000000000000000|
		// +----------------------------+

		this.pitch = this.width + 2 * MAX_VECTOR;
		const frameSize = this.pitch * (this.height + 2 * MAX_VECTOR);
		this.oldFrame = new Uint32Array(frameSize);
		this.newFrame = new Uint32Array(frameSize);

		// Populate block information.
		this.blockInfo = [];
		for(let y = 0; y < this.height; y += this.blockHeight) {
			for(let x = 0; x < this.width; x += this.blockWidth) {
				this.blockInfo.push({
					offset: (y + MAX_VECTOR) * this.pitch + (x + MAX_VECTOR),
					dx: Math.min(this.width - x, this.blockWidth),
					dy: Math.min(this.height - y, this.blockHeight),
				});
			}
		}

		// If the interval between keyframes is too big (or there are not enough
		// keyframes), enable frame caching.
		let interval = 0, intervalSum = 0, keyframeCount = 0;
		for(const frame of this.frames.slice(1)) {
			if((frame[0] & TAG_KEYFRAME) !== 0) {
				intervalSum += interval;
				keyframeCount++;
				interval = 0;
			} else {
				interval++;
			}
		}
		const keyframeInterval = intervalSum === 0 ? Infinity : Math.floor(intervalSum / keyframeCount);
		if(keyframeInterval >= 40) {
			this.frameCachingInterval = 20;
		}
	}

	decode(array: Uint8Array) {
		const isKeyFrame = array[0] & TAG_KEYFRAME;
		let frameData: Uint8Array;

		if(isKeyFrame) {
			const highVersion = array[1];
			const lowVersion = array[2];
			const compression = array[3];
			const format = array[4];
			const blockWidth = array[5];
			const blockHeight = array[6];
			frameData = array.subarray(7);

			assert(highVersion === DBZV_VERSION_HIGH && lowVersion === DBZV_VERSION_LOW,
				`Unsupported ZMBV codec version ${highVersion}.${lowVersion}: ` +
				`the only supported version is ${DBZV_VERSION_HIGH}.${DBZV_VERSION_LOW}`);
			assert(compression === COMPRESSION_NONE || compression === COMPRESSION_ZLIB,
				`Unsupported compression type ${compression}: ` +
				`only no compression (0) and zlib (1) are supported`);
			assert(format === FORMAT_32BPP,
				"Unsupported frame format: only 32 BPP is supported");
			assert(blockWidth !== 0 && blockHeight !== 0,
				"Block width/height must be greater than zero");

			this.compression = compression;

			if(
				blockWidth !== this.blockWidth ||
				blockHeight !== this.blockHeight
			) {
				this.blockWidth = blockWidth;
				this.blockHeight = blockHeight;
				this.setup();
			}

			// Reset the inflate state by calling the constructor again (yes).
			// Save ondata to restore it after the call.
			const ondata = this.decompressor.ondata;
			Unzlib.call(this.decompressor);
			this.decompressor.ondata = ondata;
		} else {
			frameData = array.subarray(1);
		}

		if(this.compression === COMPRESSION_ZLIB) {
			this.decompressor.push(frameData);
			frameData = this.decompressedData!;
		}

		let frameData32 = new Uint32Array(frameData.buffer, frameData.byteOffset);
		if(isKeyFrame) {
			for(let y = 0; y < this.height; y++) {
				for(let x = 0; x < this.width; x++) {
					this.newFrame[(y + MAX_VECTOR) * this.pitch + MAX_VECTOR + x] = frameData32[y * this.width + x];
				}
			}
		} else {
			[this.newFrame, this.oldFrame] = [this.oldFrame, this.newFrame];
			frameData32 = frameData32.subarray(Math.ceil(this.blockInfo.length * 2 / 4));
			let blockChangesOffset = 0;
			const blocks = new Int8Array(frameData.buffer, frameData.byteOffset);
			for(let i = 0; i < this.blockInfo.length; i++) {
				const blockInfo = this.blockInfo[i];
				const delta = blocks[i * 2] & 1;
				const vx = blocks[i * 2] >> 1;
				const vy = blocks[i * 2 + 1] >> 1;
				if(delta) {
					for(let y = 0; y < blockInfo.dy; y++) {
						for(let x = 0; x < blockInfo.dx; x++) {
							this.newFrame[blockInfo.offset + y * this.pitch + x] =
								this.oldFrame[blockInfo.offset + (y + vy) * this.pitch + x + vx] ^
								frameData32[blockChangesOffset++];
						}
					}
				} else {
					for(let y = 0; y < blockInfo.dy; y++) {
						for(let x = 0; x < blockInfo.dx; x++) {
							this.newFrame[blockInfo.offset + y * this.pitch + x] =
								this.oldFrame[blockInfo.offset + (y + vy) * this.pitch + x + vx];
						}
					}
				}
			}
		}
	}

	output(output: Uint8ClampedArray) {
		for(let y = 0; y < this.height; y++) {
			for(let x = 0; x < this.width; x++) {
				const pixel = this.newFrame[(y + MAX_VECTOR) * this.pitch + MAX_VECTOR + x];
				// BGRX → RGBA
				output[(y * this.width + x) * this.pixelSize + 0] = (pixel >> 16) & 0xFF;
				output[(y * this.width + x) * this.pixelSize + 1] = (pixel >> 8) & 0xFF;
				output[(y * this.width + x) * this.pixelSize + 2] = (pixel >> 0) & 0xFF;
				output[(y * this.width + x) * this.pixelSize + 3] = 0xFF;
			}
		}
	}

	presentFrame(newFrame: number, outputBuffer: ArrayBuffer) {
		// Clamp the new frame number.
		if(newFrame < 0) newFrame = 0;
		if(newFrame >= this.frames.length) newFrame = this.frames.length - 1;

		// If we"re ahead of the new frame…
		if(this.currentFrame > newFrame) {
			// Position ourselves one frame before the new one (because we have
			// to decode it in order to display it!)
			this.currentFrame = newFrame - 1;

			while(
				// We hit the beginning
				this.currentFrame >= 0 &&
				// We hit a keyframe
				(this.frames[this.currentFrame + 1][0] & TAG_KEYFRAME) === 0 &&
				// We hit a cached frame
				!this.frameCache[this.currentFrame]
			) {
				// Rewind.
				this.currentFrame--;
			}
		} else {
			// If we"re behind, look for a cached frame before the new frame
			// that also comes after the current frame.
			for(let i = newFrame; i > this.currentFrame; i--) {
				if(this.frameCache[i]) {
					this.currentFrame = i;
					break;
				}
			}
		}

		if(this.currentFrame !== this.lastDecodedFrame && this.frameCache[this.currentFrame]) {
			const { frame, state, output } = this.frameCache[this.currentFrame];
			this.newFrame = new Uint32Array(frame);
			(this.decompressor as unknown as PrivateInflate).s = { ...state };
			(this.decompressor as unknown as PrivateInflate).o = new Uint8Array(output);
		}

		while(this.currentFrame < newFrame) {
			if(
				// Caching is enabled
				this.frameCachingInterval > 0 &&
				// The caching interval passed
				(this.currentFrame % this.frameCachingInterval) === 0 &&
				// The frame has not yet been cached
				!this.frameCache[this.currentFrame] &&
				// The frame is not a keyframe (otherwise it"s useless)
				(this.frames[this.currentFrame][0] & TAG_KEYFRAME) === 0
			) {
				// Copy some of the relevant state to the cache.
				this.frameCache[this.currentFrame] = {
					frame: new Uint32Array(this.newFrame),
					state: { ...(this.decompressor as unknown as PrivateInflate).s },
					output: new Uint8Array((this.decompressor as unknown as PrivateInflate).o),
				};
			}

			const frame = this.frames[++this.currentFrame];
			this.decode(frame);
			this.lastDecodedFrame = this.currentFrame;
		}

		const output = new Uint8ClampedArray(outputBuffer);
		this.output(output);
	}
}

let decoder: ZMBVDecoder | undefined;

/*
 * AVI parsing
 */

type FourCC = number;

type Rect = {
	left: number;
	top: number;
	right: number;
	bottom: number;
};

type RIFFChunk = RIFFList;

interface RIFFList {
	fcc: "LIST";
	listType: string;
	childChunks: RIFFChunk[];
}

interface AVIHeader {
	mainHeader: AVIMainHeader;
	streamList: AVIStream[];
}

interface AVIMainHeader {
	microSecPerFrame: number;
	maxBytesPerSec: number;
	paddingGranularity: number;
	flags: number;
	totalFrames: number;
	initialFrames: number;
	streams: number;
	suggestedBufferSize: number;
	width: number;
	height: number;
}

interface AVIStream {
	index: number;
	header: AVIStreamHeader;
	format: AVIVideoStreamFormat | AVIUnknownStreamFormat;
}

interface AVIStreamHeader {
	type: FourCC;
	handler: FourCC;
	flags: number;
	priority: number;
	language: number;
	initialFrames: number;
	scale: number;
	rate: number;
	start: number;
	length: number;
	suggestedBufferSize: number;
	quality: number;
	sampleSize: number;
	frame: Rect;
};

interface AVIStreamFormat {
	raw: Uint8Array;
}

interface AVIUnknownStreamFormat extends AVIStreamFormat {
	formatType: "unknown";
}

interface AVIVideoStreamFormat extends AVIStreamFormat {
	formatType: "video";
	width: number;
	height: number;
	planes: number;
	bitCount: number;
	compression: FourCC;
	sizeImage: number;
	xPelsPerMeter: number;
	yPelsPerMeter: number;
	clrUsed: number;
	clrImportant: number;
	// XXX: a color table may follow this structure
	// https://learn.microsoft.com/en-us/windows/win32/api/wingdi/ns-wingdi-bitmapinfoheader
}

function fccToStr(fcc: FourCC) {
	return String.fromCharCode(
		fcc & 0xFF,
		(fcc >> 8) & 0xFF,
		(fcc >> 16) & 0xFF,
		(fcc >> 24) & 0xFF,
	);
}

function leUint16(array: Uint8Array, offset: number) {
	return (
		array[offset] |
		(array[offset + 1] << 8)
	) >>> 0;
}

function leSint32(array: Uint8Array, offset: number) {
	return (
		array[offset] |
		(array[offset + 1] << 8) |
		(array[offset + 2] << 16) |
		(array[offset + 3] << 24)
	);
}

function leUint32(array: Uint8Array, offset: number) {
	return leSint32(array, offset) >>> 0;
}

function* parseRIFFList(array: Uint8Array) {
	assert(fccToStr(leUint32(array, 0)) === "LIST");
	const size = leUint32(array, 4);
	for(let start = 12; start - 12 + 8 < size;) {
		const childSize = leUint32(array, start + 4);
		yield new Uint8Array(array.buffer, array.byteOffset + start, childSize + 8);
		start += 8 + childSize + childSize % 2;
	}
}

function parseAVIHeader(array: Uint8Array): AVIHeader {
	let mainHeader: AVIMainHeader | undefined;
	const streamList: AVIStream[] = [];
	assert(fccToStr(leUint32(array, 8)) === "hdrl");
	for(const chunk of parseRIFFList(array)) {
		switch(fccToStr(leUint32(chunk, 0))) {
		case "avih":
			mainHeader = {
				microSecPerFrame:    leUint32(chunk, 8),
				maxBytesPerSec:      leUint32(chunk, 12),
				paddingGranularity:  leUint32(chunk, 16),
				flags:               leUint32(chunk, 20),
				totalFrames:         leUint32(chunk, 24),
				initialFrames:       leUint32(chunk, 28),
				streams:             leUint32(chunk, 32),
				suggestedBufferSize: leUint32(chunk, 36),
				width:               leUint32(chunk, 40),
				height:              leUint32(chunk, 44),
			};
			break;
		case "LIST":
			if(fccToStr(leUint32(chunk, 8)) !== "strl") {
				continue;
			}
			let streamHeader: AVIStream["header"] | undefined;
			let streamFormat: AVIStream["format"] | undefined;
			for(const streamChunk of parseRIFFList(chunk)) {
				switch(fccToStr(leUint32(streamChunk, 0))) {
				case "strh":
					streamHeader = {
						type:                leUint32(streamChunk, 8),
						handler:             leUint32(streamChunk, 12),
						flags:               leUint32(streamChunk, 16),
						priority:            leUint16(streamChunk, 20),
						language:            leUint16(streamChunk, 22),
						initialFrames:       leUint32(streamChunk, 24),
						scale:               leUint32(streamChunk, 28),
						rate:                leUint32(streamChunk, 32),
						start:               leUint32(streamChunk, 36),
						length:              leUint32(streamChunk, 40),
						suggestedBufferSize: leUint32(streamChunk, 44),
						quality:             leUint32(streamChunk, 48),
						sampleSize:          leUint32(streamChunk, 52),
						frame: {
							left:   leSint32(streamChunk, 56),
							top:    leSint32(streamChunk, 60),
							right:  leSint32(streamChunk, 64),
							bottom: leSint32(streamChunk, 68),
						},
					};
					break;
				case "strf":
					if(streamHeader && fccToStr(streamHeader.type) === "vids") {
						streamFormat = {
							formatType:    "video",
							raw:           streamChunk.subarray(8),
							width:         leSint32(streamChunk, 12),
							height:        leSint32(streamChunk, 16),
							planes:        leUint16(streamChunk, 20),
							bitCount:      leUint16(streamChunk, 22),
							compression:   leUint32(streamChunk, 24),
							sizeImage:     leUint32(streamChunk, 28),
							xPelsPerMeter: leSint32(streamChunk, 32),
							yPelsPerMeter: leSint32(streamChunk, 36),
							clrUsed:       leUint32(streamChunk, 40),
							clrImportant:  leUint32(streamChunk, 44),
						}
					} else {
						streamFormat = {
							formatType: "unknown",
							raw: streamChunk.subarray(8),
						};
					}
				}
			}
			assertNonNull(streamHeader);
			assertNonNull(streamFormat);
			streamList.push({
				index: streamList.length,
				header: streamHeader,
				format: streamFormat,
			});
			break;
		}
	}
	assertNonNull(mainHeader);
	return { mainHeader, streamList };
}

async function initialize(url: string) {
	const videoResp = await fetch(url);
	const videoBuffer = await videoResp.arrayBuffer();
	let array = new Uint8Array(videoBuffer);
	assert(fccToStr(leUint32(array, 0)) === "RIFF");
	assert(fccToStr(leUint32(array, 8)) === "AVI ");

	array = array.subarray(12);
	const header = parseAVIHeader(array);
	const { width, height, microSecPerFrame } = header.mainHeader;
	const stream = header.streamList.find(stream => fccToStr(stream.header.handler).toLowerCase() === "zmbv");
	assertNonNull(stream);

	while(!(fccToStr(leUint32(array, 0)) === "LIST" && fccToStr(leUint32(array, 8)) === "movi")) {
		const size = leUint32(array, 4);
		array = array.subarray(8 + size + size % 2);
	}
	array = array.subarray(0, leUint32(array, 4) + 8);

	function* extractFrames(streamIdx: number, array: Uint8Array) {
		for(const dataChunk of parseRIFFList(array)) {
			const fcc = fccToStr(leUint32(dataChunk, 0));
			if(fcc === "LIST" && fccToStr(leUint32(dataChunk, 8)) === "rec ") {
				yield* extractFrames(streamIdx, dataChunk);
			}
			if(Number.parseInt(fcc.slice(0, 2)) === streamIdx) {
				yield dataChunk.subarray(8);
			}
		}
	}
	const frames = [...extractFrames(stream.index, array)];

	decoder = new ZMBVDecoder(width, height, frames);

	postMessage({
		kind: "ready",
		width: width,
		height: height,
		frameCount: frames.length,
		frameDurationMicrosec: microSecPerFrame,
	});
}

function requestFrame({ frame, width, height, output }: { frame: number; width: number; height: number; output: ArrayBuffer; }) {
	if(!decoder) return;
	decoder.presentFrame(frame, output);
	postMessage({ kind: "frame", frame, width, height, output }, {
		transfer: [output],
	});
}

addEventListener("message", (ev) => {
	switch(ev.data.kind) {
	case "url":
		return initialize(ev.data.url);
	case "request":
		return requestFrame(ev.data);
	}
});
