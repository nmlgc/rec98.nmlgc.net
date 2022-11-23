const MAX_VECTOR = 16;
const ZMBV_FORMAT_8BPP = 0x04;
const ZMBV_FORMAT_15BPP = 0x05;
const ZMBV_FORMAT_16BPP = 0x06;
const ZMBV_FORMAT_32BPP = 0x08;

/**
 * @param {number} integer
 * @returns {string}
 */
function parseFourCC(integer) {
	return String.fromCharCode(integer >> 24, (integer >> 16) & 0xff, (integer >> 8) & 0xff, integer & 0xff);
}

/**
 * @param {DataView} view
 * @returns {object}
 */
function parseList(view) {
	const list = [];
	let offset = 0;
	while(offset < view.byteLength) {
		const newView = new DataView(view.buffer, view.byteOffset + offset, view.byteLength - offset);
		const chunk = parseChunk(newView);
		offset += chunk.size + 8;
		offset += offset % 2;
		list.push(chunk);
	}
	return list;
}

/**
 * @param {DataView} view
 * @returns {object}
 */
function parseChunk(view) {
	const type = parseFourCC(view.getUint32(0));
	const size = view.getUint32(4, true);
	let tag, body, children;
	if(type === "LIST" || type === "RIFF") {
		tag = parseFourCC(view.getUint32(8));
		body = new DataView(view.buffer, view.byteOffset + 12, size - 4);
		children = parseList(body);
	} else {
		body = new DataView(view.buffer, view.byteOffset + 8, size);
	}
	return { type, size, tag, body, children };
}

/**
 * @param {DataView} view
 * @returns {object}
 */
function parseAvih(view) {
	return {
		microSecPerFrame: view.getUint32(0, true),
		width: view.getUint32(32, true),
		height: view.getUint32(36, true),
	};
}

/**
 * @param {DataView} view
 * @returns {object}
 */
function parseStrh(view) {
	return {
		type: parseFourCC(view.getUint32(0)),
		handler: parseFourCC(view.getUint32(4)),
	};
}

let decoderModuleResolve, decoderModuleReject;
/** @type {Promise<WebAssembly.Module>} */
const decoderModulePromise = new Promise((resolve, reject) => {
	decoderModuleResolve = resolve;
	decoderModuleReject = reject;
});

class ZMBVDecoder {
	/** @type {string} */
	url;

	/** @type {(Promise|undefined)} */
	initPromise;

	/** @type {WebAssembly.Instance} */
	decoder;

	/** @type {WebAssembly.Memory} */
	memory;

	/** @type {Array<Uint8Array>} */
	frames;
	framePtr = 0;

	loaded = false;
	width = 0;
	height = 0;
	frameCount = 0;
	frameDurationMicrosec = 1;
	currentFrame = 0;

	/**
	 * @param {string} url
	 */
	constructor(url) {
		this.url = url;
	}

	async init() {
		const videoBuffer = await fetch(this.url).then((resp) => resp.arrayBuffer());
		this.memory = new WebAssembly.Memory({ initial: 256, maximum: 256 });
		this.decoder = await WebAssembly.instantiate(await decoderModulePromise, {
			env: { memory: this.memory },
			wasi_snapshot_preview1: {
				fd_close: () => {},
				proc_exit: () => {},
				fd_write: () => {},
				environ_sizes_get: () => {},
				environ_get: () => {},
				fd_seek: () => {},
			},
		});
		this.decoder.exports._initialize();

		const avi = parseChunk(new DataView(videoBuffer));
		if(avi.type !== "RIFF" || avi.tag !== "AVI ") {
			throw new Error("Invalid FourCC values");
		}
		const hdrl = avi.children.find((child) => child.tag === "hdrl");
		const header = parseAvih(hdrl.children.find((child) => child.type === "avih").body);
		const streams = hdrl.children
			.filter((child) => child.tag === "strl")
			.map((strl) => parseStrh(strl.children.find((child) => child.type === "strh").body));
		const videoStreamID = streams.findIndex(
			(stream) => stream.type === "vids" && stream.handler.toLowerCase() === "zmbv"
		);

		const neededChunkTypePrefix = videoStreamID.toString().padStart(2, "0");
		this.frames = avi.children
			.find((child) => child.tag === "movi")
			.children.map((chunk) => (chunk.tag === "rec " ? chunk.children : chunk))
			.flat()
			.filter((chunk) => chunk.type.startsWith(neededChunkTypePrefix))
			.map((chunk) => new Uint8Array(chunk.body.buffer, chunk.body.byteOffset, chunk.body.byteLength));

		const frameDataSize = this.frames.reduce((acc, cur) => Math.max(acc, cur.length), 0);
		this.framePtr = this.decoder.exports.malloc(frameDataSize);
		if(!this.decoder.exports.setup(header.width, header.height)) {
			throw new Error("Decoder setup error");
		}

		this.width = header.width;
		this.height = header.height;
		this.frameCount = this.frames.length;
		this.frameDurationMicrosec = header.microSecPerFrame;
		this.currentFrame = -1;
		this.loaded = true;
	}

	/**
	 * @param {number} newFrame
	 * @param {ArrayBuffer} output
	 */
	presentFrame(newFrame, outputBuffer) {
		if(!this.loaded) return;
		if(newFrame < 0) newFrame = 0;
		if(newFrame >= this.frameCount) newFrame = this.frameCount - 1;
		if(newFrame == 0) {
			this.currentFrame = -1;
		} else if(this.currentFrame > newFrame) {
			this.currentFrame = -1;
		}

		while(this.currentFrame < newFrame) {
			const frame = this.frames[++this.currentFrame];
			new Uint8Array(this.memory.buffer, this.framePtr, frame.length).set(frame);
			if(!this.decoder.exports.decode_frame(this.framePtr, frame.length)) {
				throw new Error("Decoder error");
			}
		}

		const infoPtr = this.decoder.exports.get_info();
		const info = new Uint32Array(this.memory.buffer, infoPtr, 7);
		const palette = new Uint8Array(this.memory.buffer, info[0], info[1]);
		const frame = new Uint8Array(this.memory.buffer, info[2], info[3]);
		const format = info[6];
		const output = new Uint8ClampedArray(outputBuffer);
		let pixelSize;
		switch(format) {
			case ZMBV_FORMAT_8BPP:
				pixelSize = 1;
				break;
			case ZMBV_FORMAT_15BPP:
			case ZMBV_FORMAT_16BPP:
				pixelSize = 2;
				break;
			case ZMBV_FORMAT_32BPP:
				pixelSize = 4;
		}
		let w = 0;
		for(let row = 0; row < this.height; row++) {
			const rowStart = pixelSize * (MAX_VECTOR + (row + MAX_VECTOR) * (this.width + MAX_VECTOR * 2));
			switch(format) {
				case ZMBV_FORMAT_8BPP:
					for(let col = 0; col < this.width; col++) {
						const c = frame[rowStart + col];
						output[w++] = palette[c * 4 + 0];
						output[w++] = palette[c * 4 + 1];
						output[w++] = palette[c * 4 + 2];
						output[w++] = 255;
					}
					break;
				case ZMBV_FORMAT_15BPP:
					for(let col = 0; col < this.width; col++) {
						const c = frame[rowStart + col * 2];
						output[w++] = ((c & 0x7c00) * 0x21) >>> 12;
						output[w++] = ((c & 0x03e0) * 0x21) >>> 7;
						output[w++] = ((c & 0x001f) * 0x21) >>> 2;
						output[w++] = 255;
					}
					break;
				case ZMBV_FORMAT_16BPP:
					for(let col = 0; col < this.width; col++) {
						const c = frame[rowStart + col * 2];
						output[w++] = ((c & 0xf800) * 0x21) >>> 13;
						output[w++] = ((c & 0x07e0) * 0x41) >>> 9;
						output[w++] = ((c & 0x001f) * 0x21) >>> 2;
						output[w++] = 255;
					}
					break;
				case ZMBV_FORMAT_32BPP:
					for(let col = 0; col < this.width; col++) {
						output[w++] = frame[rowStart + col * 4 + 2];
						output[w++] = frame[rowStart + col * 4 + 1];
						output[w++] = frame[rowStart + col * 4 + 0];
						output[w++] = 255;
					}
					break;
				default:
					break;
			}
		}
	}
}

/** @type {(ZMBVDecoder|undefined)} */
let decoder;

async function initialize(url) {
	const newDecoder = new ZMBVDecoder(url);
	await newDecoder.init();
	decoder = newDecoder;
	postMessage({
		kind: "ready",
		width: decoder.width,
		height: decoder.height,
		frameCount: decoder.frameCount,
		frameDurationMicrosec: decoder.frameDurationMicrosec,
	});
}

function requestFrame({ frame, width, height, output }) {
	if(!decoder) return;
	decoder.presentFrame(frame, output);
	postMessage({ kind: "frame", width, height, output }, undefined, [output]);
}

onmessage = (ev) => {
	switch(ev.data.kind) {
		case "url":
			return initialize(ev.data.url);
		case "module":
			return decoderModuleResolve(ev.data.module);
		case "request":
			return requestFrame(ev.data);
	}
};
