class ReC98ZMBVVideo extends HTMLElement {
	static workerURL = document.currentScript?.dataset.workerUrl;

	static intersectionObserver = new IntersectionObserver((entries, observer) => {
		for(const entry of entries) {
			if(entry.isIntersecting) {
				const videoElement = entry.target as ReC98ZMBVVideo;
				observer.unobserve(videoElement);
				videoElement.onVisibleInViewport();
			}
		}
	}, {
		rootMargin: "0px 0px 1000px 0px",
	});

	#frameCount = 0;
	#frameDurationMicrosec = 1;
	#currentFrameStartTime = 0;
	#currentFrame = 0;
	#canPlay = false;
	#seeking = false;
	#paused = true;
	loop = false;

	/** @type {string} */
	#src;

	/** @type {HTMLImageElement} */
	#posterImage;

	/** @type {string} */
	#posterSrc;

	/** @type {HTMLCanvasElement} */
	#canvas;

	/** @type {?CanvasRenderingContext2D} */
	#ctx;

	/** @type {?ImageData} */
	#imageData;

	/** @type {?Worker} */
	#worker;

	/** @type {?Promise} */
	#loadPromise;

	/** @type {?number} */
	#frameRequested;

	constructor() {
		super();

		this.#canvas = document.createElement("canvas");
		this.appendChild(this.#canvas);

		this.#posterImage = new Image();
		this.#posterImage.onload = () => {
			if(!this.#canPlay) {
				this.#initCtxIfNeeded();
				this.#ctx.drawImage(this.#posterImage, 0, 0);
			}
		};
	}

	disconnectedCallback() {
		ReC98ZMBVVideo.intersectionObserver.unobserve(this);
	}

	onVisibleInViewport() {
		if(this.#posterImage.src !== this.#posterSrc) {
			this.#posterImage.src = this.#posterSrc;
		}
	}

	#initCtxIfNeeded() {
		if(!this.#ctx) {
			this.#ctx = this.#canvas.getContext("2d", { alpha: false });
		}
	}

	get src() {
		return this.#src;
	}

	set src(src) {
		this.#src = src;
		this.#loadPromise = null;
		this.#frameRequested = null;
		this.#worker?.terminate();
		this.#worker = null;

		this.#imageData = null;
		if(this.#ctx) {
			if(this.#posterImage.naturalWidth > 0 && this.#posterImage.naturalHeight > 0) {
				this.#ctx.drawImage(this.#posterImage, 0, 0);
			} else {
				this.#ctx.clearRect(0, 0, this.#canvas.width, this.#canvas.height);
			}
		}

		this.#canPlay = false;
		this.#paused = true;
		this.#seeking = false;
		this.#currentFrame = 0;
		this.#frameDurationMicrosec = 1;
		this.#frameCount = 0;
	}

	/**
	 * @returns {?string}
	 */
	get poster() {
		return this.#posterSrc;
	}

	/**
	 * @param {string} url
	 */
	set poster(url) {
		this.#posterSrc = url;
		ReC98ZMBVVideo.intersectionObserver.observe(this);
	}

	get width() {
		return this.#canvas.width;
	}

	set width(newWidth) {
		if(this.#canvas.width !== newWidth) {
			this.#canvas.width = newWidth;
			if(this.#canvas.height) {
				this.#canvas.style.aspectRatio = `${this.#canvas.width} / ${this.#canvas.height}`;
			}
		}
	}

	get height() {
		return this.#canvas.height;
	}

	set height(newHeight) {
		if(this.#canvas.height !== newHeight) {
			this.#canvas.height = newHeight;
			if(this.#canvas.height) {
				this.#canvas.style.aspectRatio = `${this.#canvas.width} / ${this.#canvas.height}`;
			}
		}
	}

	get currentFrame() {
		return this.#currentFrame;
	}

	set currentFrame(newFrame) {
		if(this.#currentFrame === newFrame) {
			return;
		}
		if(newFrame < 0) newFrame = 0;
		if(newFrame >= this.#frameCount) newFrame = this.#frameCount - 1;
		if(this.#paused) {
			this.dispatchEvent(new Event("seeking"));
		}
		this.#currentFrame = newFrame;
		this.#sendRequest();
		this.dispatchEvent(new Event("timeupdate"));
	}

	#sendRequest() {
		if(this.#frameRequested === null) {
			this.#worker.postMessage({
				kind: "request",
				frame: this.#currentFrame,
				width: this.#imageData.width,
				height: this.#imageData.height,
				output: this.#imageData.data.buffer,
			}, [this.#imageData.data.buffer]);
		}
		this.#frameRequested = this.#currentFrame;
	}

	/**
	 * @param {MessageEvent} event
	 */
	#onMessage(event) {
		if(event.data.kind === "frame") {
			const { frame, width, height, output } = event.data;
			console.log("received %d", frame);
			this.#imageData = new ImageData(new Uint8ClampedArray(output), width, height);
			const frameRequested = this.#frameRequested;
			this.#frameRequested = null;
			if(frameRequested !== frame) {
				this.#ctx.putImageData(this.#imageData, 0, 0);
				this.#sendRequest();
			} else if(this.#paused) {
				this.#ctx.putImageData(this.#imageData, 0, 0);
				this.dispatchEvent(new Event("seeked"));
			}
		}
	}

	get currentTime() {
		return secondsFrom(this.#currentFrame, 1_000_000 / this.#frameDurationMicrosec);
	}

	set currentTime(newTime) {
		const newFrame = frameFrom(newTime, 1_000_000 / this.#frameDurationMicrosec);
		this.currentFrame = newFrame;
	}

	get duration() {
		return secondsFrom(this.#frameCount, 1_000_000 / this.#frameDurationMicrosec);
	}

	get paused() {
		return this.#paused;
	}

	get seeking() {
		return this.#seeking;
	}

	load() {
		if(!this.#loadPromise) {
			this.#loadPromise = (async () => {
				this.dispatchEvent(new Event("loadstart"));
				this.#initCtxIfNeeded();

				if(!ReC98ZMBVVideo.workerURL) {
					throw new Error('<script> tag missing data-worker-url attribute');
				}
				const worker = new Worker(ReC98ZMBVVideo.workerURL);
				const promise = new Promise((resolve, reject) => {
					worker.onmessage = (event) => {
						if(event.data.kind === "ready") {
							resolve(event.data);
						}
					};
					worker.onerror = reject;
				});
				worker.postMessage({ kind: "url", url: this.src });

				const { width, height, frameCount, frameDurationMicrosec } = await (promise as any);
				this.width = width;
				this.height = height;
				this.#imageData = this.#ctx.createImageData(width, height);
				this.#frameCount = frameCount;
				this.#frameDurationMicrosec = frameDurationMicrosec;

				worker.onmessage = this.#onMessage.bind(this);
				worker.onerror = null;
				this.#worker = worker;

				this.currentFrame = 0;
				this.#canPlay = true;
				this.#sendRequest();
				this.dispatchEvent(new Event("canplay"));
				this.dispatchEvent(new Event("canplaythrough"));
			})();
		}
		return this.#loadPromise;
	}

	async play() {
		if(!this.#canPlay) {
			await this.load();
		}
		if(this.#paused) {
			this.#paused = false;
			this.#currentFrameStartTime = performance.now() - 1_000_000 / this.#frameDurationMicrosec;
			const tick = () => {
				if(this.#paused) return;
				let newFrame = this.#currentFrame;
				let delta = performance.now() - this.#currentFrameStartTime;
				while(delta >= this.#frameDurationMicrosec / 1000) {
					newFrame++;
					delta -= this.#frameDurationMicrosec / 1000;
				}
				this.#currentFrameStartTime = performance.now() - delta;
				if(newFrame > this.#frameCount - 1) {
					if(this.loop) {
						newFrame = 0;
					} else {
						newFrame = this.#frameCount - 1;
						this.pause();
						return;
					}
				}
				console.log("intended  %d", newFrame);
				if(this.#frameRequested === null) {
					this.#ctx.putImageData(this.#imageData, 0, 0);
				}
				this.currentFrame = newFrame;
				window.requestAnimationFrame(tick);
			};
			tick();
			this.dispatchEvent(new Event("play"));
		}
	}

	pause() {
		if(!this.#paused) {
			this.#paused = true;
			this.dispatchEvent(new Event("pause"));
		}
	}
}

// NOTE(handlerug): I suppose esbuild has to be configured properly to remove this hack.
(window as any).ReC98ZMBVVideo = ReC98ZMBVVideo;

window.customElements.define("rec98-zmbv-video", ReC98ZMBVVideo);
