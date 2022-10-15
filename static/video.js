/**
 * @param {Element} element
 * @param {string} attribute
 * @returns {string}
 */
function attributeAsString(element, attribute) {
	const ret = element.getAttribute(attribute);
	if(!ret) {
		throw `${attribute} not given.`;
	}
	return ret;
}

/**
 * @param {Element} element
 * @param {string} attribute
 * @returns {number}
 */
function attributeAsNumber(element, attribute) {
	return Number(attributeAsString(element, attribute));
}

/**
 * Raw seconds→frame conversion.
 *
 * @param {number} seconds
 * @param {number} fps
 * @returns {number}
 */
function frameFrom(seconds, fps) {
	return Math.floor(seconds * fps);
}

/**
 * Raw frame→currentTime conversion. Avoids rounding errors by returning the
 * middle of a frame.
 *
 * @param {number} frame
 * @param {number} fps
 * @returns {number}
 */
function secondsFrom(frame, fps) {
	return ((frame + 0.5) / fps);
}

class ReC98Video extends HTMLElement {
	// Members
	// -------
	eControls = document.createElement("div");
	ePlay = document.createElement("button");
	eTimeSecondsIcon = document.createElement("span");
	eTimeSeconds = document.createElement("span");
	eTimeFrameIcon = document.createElement("span");
	eTimeFrame = document.createElement("span");

	/** @type {HTMLCollectionOf<HTMLVideoElement>} */
	videos;

	/** @type {HTMLVideoElement} */
	videoShown;

	fps = 1;

	/** @type {number | null} */
	timeIntervalID = null;
	// -------

	/**
	 * Raw currentTime→frame conversion.
	 *
	 * @returns {number}
	 */
	 frame() {
		const seconds = ((this.videoShown) ? this.videoShown.currentTime : 0);
		return frameFrom(seconds, this.fps);
	}

	onPlay() {
		this.ePlay.textContent = "⏸";
		this.ePlay.title = "Pause (Space)";
		this.timeIntervalID = setInterval(
			(() => this.renderTimeFromVideo()), (1000 / this.fps)
		);
	}

	play() {
		// Prevent a second call from the `onplay` handler.
		if(this.timeIntervalID) {
			return;
		}

		// https://developer.chrome.com/blog/play-request-was-interrupted/
		const playPromise = this.videoShown.play();
		if(playPromise !== undefined) {
			playPromise.then(() => this.onPlay());
		} else {
			this.onPlay();
		}
	}

	pause() {
		this.videoShown?.pause();
		this.ePlay.textContent = "▶";
		this.ePlay.title = "Play (Space)";
		if(this.timeIntervalID) {
			clearInterval(this.timeIntervalID);
			this.timeIntervalID = null;
		}
	}

	/** @param {Event} event */
	toggle(event) {
		if(!this.videoShown) {
			return;
		}
		event.preventDefault();

		// This function might be called twice in very quick succession. As
		// `<video>.play()` is asynchronous in modern browsers, but immediately
		// sets `<video>.paused` to `false`, a second call checking for that
		// flag would call `<video>.pause()` while `<video>.play()` is still
		// running, leading to an infinite loop of "play() was interrupted by
		// pause()" exceptions. [this.timeIntervalID] is only set after the
		// promise resolved, and is therefore a more reliable indicator of the
		// current playing state.
		if(!this.timeIntervalID) {
			this.play();
		} else {
			this.pause();
		}
		this.focus();
	}

	/** @param {number} seconds */
	renderTime(seconds) {
		const frame = frameFrom(seconds, this.fps);
		this.eTimeSeconds.textContent = (
			Math.trunc(seconds).toString().padStart(2, "0") +
			":" +
			Math.trunc((seconds % 1) * 100).toString().padStart(2, "0")
		);
		this.eTimeFrame.textContent = frame.toString();
	}

	renderTimeFromVideo() {
		this.renderTime(this.videoShown.currentTime);
	}

	// Constant property initialization
	constructor() {
		super();
		this.eControls.className = "controls";

		// Play/Pause button
		// -----------------
		this.ePlay.className = "large";
		// -----------------

		// Time
		// ----
		this.eTimeSecondsIcon.textContent = "⌚";
		this.eTimeSecondsIcon.title = "Seconds";
		this.eTimeSeconds.title = "Seconds";
		this.eTimeFrameIcon.textContent = "🎞️";
		this.eTimeFrameIcon.title = "Frame";
		this.eTimeFrame.title = "Frame";

		this.eTimeSecondsIcon.className = "seconds icon";
		this.eTimeSeconds.className = "seconds time";
		this.eTimeFrameIcon.className = "frame icon";
		this.eTimeFrame.className = "frame time";
		this.renderTime(0);
		// ----
	}

	/**
	 * @param {number} index
	 */
	showVideo(index) {
		const videoNew = this.videos[index];
		const seekedFunc = (() => this.renderTimeFromVideo());

		this.fps = attributeAsNumber(videoNew, "data-fps");
		this.videoShown = videoNew;

		videoNew.onclick = ((event) => this.toggle(event));
		videoNew.onseeked = seekedFunc;
		videoNew.onplay = (() => this.play());
		videoNew.onpause = (() => {
			this.pause();

			// Since `currentTime` is continuous, every (1 / [fps]) time span
			// within the video can always correspond to at least two discrete
			// frames. The actual frame for a specific `currentTime` value is
			// therefore defined by the browser's rounding algorithm, and will
			// certainly not match the one used in frame(), which defines the
			// number shown in [eTimeFrame]. As a result, the frame number is
			// very likely to be incorrect half of the time.
			// It doesn't matter during playback, where the frame number is
			// constantly updating anyway, but manifests itself in inconsistent
			// frame numbers when seeking a paused video.
			// As a workaround, we perform a round-trip conversion from
			// `currentTime` to frames and back, which gets us back onto our
			// discrete seek grid in any case. This might lead to a noticeable
			// jump back by one frame, but is unfortunately the best compromise
			// in view of what we're given here.
			const frame = frameFrom(videoNew.currentTime, this.fps);
			videoNew.currentTime = secondsFrom(frame, this.fps);
		});
	}

	/**
	 * DOM initialization. Called from the separate ReC98VideoInit component
	 * because child components aren't visible during connectedCallback() yet…
	 */
	init() {
		this.tabIndex = -1;

		this.eControls.appendChild(this.ePlay);
		this.eControls.appendChild(this.eTimeSecondsIcon);
		this.eControls.appendChild(this.eTimeSeconds);
		this.eControls.appendChild(this.eTimeFrameIcon);
		this.eControls.appendChild(this.eTimeFrame);
		this.appendChild(this.eControls);

		// Event handlers
		// --------------
		this.onkeydown = ((event) => {
			if(document.activeElement !== this) {
				return;
			}
			switch(virtualKey(event)) {
			case ' ':
				return this.ePlay.onclick?.(event);
			}
		});

		// Preloading the video is required for seeking to work before the
		// video has been play()ed the first time.
		this.onpointerenter = (() => {
			for(const video of this.videos) {
				video.load();
				video.preload = "auto";
			}
			this.onpointerenter = null;
		});

		this.ePlay.onclick = ((event) => this.toggle(event));
		// --------------

		this.videos = this.getElementsByTagName("video");
		let lastChild = null;
		let requested = null;
		for(let i = 0; i < this.videos.length; i++) {
			const video = this.videos[i];
			video.onclick = ((event) => this.toggle(event));

			// The backend code still enables controls just in case the user
			// runs with disabled JavaScript, so we're separately disabling
			// them here as we're about to replace them with our own.
			video.controls = false;

			if(video.classList.contains("active")) {
				requested = i;
			}
			lastChild = i;
		}
		if(lastChild === null) {
			throw "No <video> child element found.";
		}

		this.showVideo(requested ?? lastChild);
		this.pause();
	}
};

window.customElements.define("rec98-video", ReC98Video);
