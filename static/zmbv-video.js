const zmbvSupported = (() => {
	if(!("WebAssembly" in window) || !("Worker" in window)) {
		return false;
	}
	if((() => {
		const canvas = document.createElement("canvas");
		return !canvas.getContext("2d");
	})()) {
		return false;
	}
	return true;
})();

/** @type {(Promise<WebAssembly.Module>|undefined)} */
let decoderModulePromise;

class ZMBVVideoElement extends HTMLElement {
	/** @type {string} */
	url;

	/** @type {(string|undefined)} */
	poster;

	loop = false;

	/** @private */
	_width = 0;

	/** @private */
	_height = 0;

	/** @type {(Worker|undefined)} @private */
	worker;

	/** @type {(Promise|undefined)} @private */
	initPromise;

	/** @type {(HTMLImageElement|undefined)} @private */
	img;

	/** @type {(HTMLCanvasElement|undefined)} @private */
	canvas;

	/** @type {(CanvasRenderingContext2D|undefined)} @private */
	ctx;

	/** @type {(ImageData|null)} */
	imageData = null;

	/** @private */
	frameDurationMicrosec = 1;

	/** @private */
	frameCount = 0;

	/** @private */
	currentFrameStartTime = 0;

	/** @private */
	_currentFrame = 0;

	/** @private */
	_seeking = false;

	/** @private */
	_playing = false;

	/** @private */
	_frameRequested = false;

	/**
	 * @param {string} url
	 */
	constructor(url, poster = undefined) {
		super();
		this.url = url;
		this.poster = poster;
		if(this.poster) {
			const img = document.createElement("img");
			img.src = this.poster;
			img.style.imageRendering = "crisp-edges";
			img.style.imageRendering = "pixelated";
			img.style.maxHeight = "100%";
			img.onload = () => {
				if(!this.imageData) {
					this.img = img;
					this.appendChild(img);
				}
			};
		}
	}

	/**
	 * @private
	 */
	async init() {
		if(!decoderModulePromise) {
			const url = "/static/zmbv/zmbv.wasm";
			try {
				decoderModulePromise = WebAssembly.compileStreaming(fetch(url));
			} catch {
				decoderModulePromise = fetch(url)
					.then(resp => resp.arrayBuffer)
					.then(bytes => WebAssembly.compile(bytes));
			}
		}
		const worker = new Worker("/static/zmbv-worker.js");
		const promise = new Promise((resolve, reject) => {
			const onError = event => {
				reject(event.error);
			};
			const onMessage = event => {
				if(event.data.kind === "ready") {
					resolve(event.data);
				}
				worker.removeEventListener("message", onMessage);
				worker.removeEventListener("error", onError);
			};
			worker.onmessage = onMessage;
			worker.onerror = onError;
		});
		worker.postMessage({ kind: "url", url: this.url });
		decoderModulePromise.then(module => worker.postMessage({ kind: "module", module }));

		const { width, height, frameCount, frameDurationMicrosec } = await promise;
		worker.onmessage = this.onMessage.bind(this);
		this.worker = worker;
		this.worker.onerror = event => {
			console.error("Decoder worker error:", event.error);
		};

		this.canvas = document.createElement("canvas");
		this.width = width;
		this.height = height;
		this.canvas.style.imageRendering = "crisp-edges";
		this.canvas.style.imageRendering = "pixelated";
		this.ctx = this.canvas.getContext("2d", {
			alpha: false,
		});
		this.imageData = this.ctx.createImageData(width, height);
		if(this.img) {
			this.ctx.drawImage(this.img, 0, 0);
			this.img.remove();
			this.img = undefined;
		}
		this.appendChild(this.canvas);

		this.frameCount = frameCount;
		this.frameDurationMicrosec = frameDurationMicrosec;
		this.dispatchEvent(new Event("canplay"));
		this.currentFrame = 0;
	}

	/**
	 * @returns {Promise}
	 */
	load() {
		if(!this.initPromise) {
			this.initPromise = this.init();
		}
		return this.initPromise;
	}

	get width() {
		return this._width;
	}

	set width(newWidth) {
		if(this._width !== newWidth) {
			this._width = newWidth;
			this.style.minWidth = newWidth + "px";
		}
		if(this.img) {
			this.img.width = newWidth;
		}
		if(this.canvas && this.canvas.width !== newWidth) {
			this.canvas.width = newWidth;
		}
	}

	get height() {
		return this._height;
	}

	set height(newHeight) {
		if(this._height !== newHeight) {
			this._height = newHeight;
			this.style.minHeight = newHeight + "px";
		}
		if(this.img) {
			this.img.height = newHeight;
		}
		if(this.canvas && this.canvas.height !== newHeight) {
			this.canvas.height = newHeight;
		}
	}

	get currentFrame() {
		return this._currentFrame;
	}

	set currentFrame(newFrame) {
		if(newFrame < 0) newFrame = 0;
		if(newFrame >= this.frameCount) newFrame = this.frameCount - 1;
		this._currentFrame = newFrame;
		this._seeking = true;
		this.requestFrame();
	}

	get currentTime() {
		return secondsFrom(this._currentFrame, 1_000_000 / this.frameDurationMicrosec);
	}

	set currentTime(newTime) {
		const newFrame = frameFrom(newTime, 1_000_000 / this.frameDurationMicrosec);
		this.currentFrame = newFrame;
	}

	get duration() {
		return secondsFrom(this.frameCount, 1_000_000 / this.frameDurationMicrosec);
	}

	get playing() {
		return this._playing;
	}

	get paused() {
		return !this._playing;
	}

	get seeking() {
		return this._seeking;
	}

	/**
	 * @private
	 */
	requestFrame() {
		if(this._frameRequested) return;
		this._frameRequested = true;
		this.worker.postMessage({
			kind: "request",
			frame: this._currentFrame,
			width: this.imageData.width,
			height: this.imageData.height,
			output: this.imageData.data.buffer,
		}, [this.imageData.data.buffer]);
	}

	/**
	 * @private
	 * @param {MessageEvent} event
	 */
	onMessage(event) {
		if(event.data.kind === "frame") {
			const { width, height, output } = event.data;
			this.imageData = new ImageData(new Uint8ClampedArray(output), width, height);
			this._frameRequested = false;
			if(this._seeking) {
				this._seeking = false;
				this.ctx.putImageData(this.imageData, 0, 0);
				this.dispatchEvent(new Event("seeked"));
			}
		}
	}

	async play() {
		await this.load();
		if(!this._playing) {
			this.currentFrameStartTime = performance.now();
			this._playing = true;
			const tick = () => {
				if(!this._playing) return;
				let newFrame = this._currentFrame;
				let delta = performance.now() - this.currentFrameStartTime;
				while(delta >= this.frameDurationMicrosec / 1000) {
					newFrame++;
					delta -= this.frameDurationMicrosec / 1000;
				}
				this.currentFrameStartTime = performance.now() - delta;
				if(newFrame > this.frameCount - 1) {
					if(this.loop) {
						newFrame = 0;
					} else {
						this.pause();
					}
				}
				if(!this._frameRequested) {
					this.ctx.putImageData(this.imageData, 0, 0);
				}
				if(this._currentFrame !== newFrame) {
					this._currentFrame = newFrame;
					this.requestFrame();
				}
				window.requestAnimationFrame(tick);
			}
			tick();
			setTimeout(() => {
				this.dispatchEvent(new Event("play"));
			}, 0);
		}
	}

	pause() {
		if(this._playing) {
			this._playing = false;
			this.dispatchEvent(new Event("pause"));
		}
	}
}

window.customElements.define("rec98-zmbv-video", ZMBVVideoElement);
window["ZMBVVideoElement"] = ZMBVVideoElement;
