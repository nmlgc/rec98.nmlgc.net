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

class ReC98Video extends HTMLElement {
	// Members
	// -------
	eControls = document.createElement("div");
	ePlay = document.createElement("button");
	eTimeSecondsIcon = document.createElement("span");
	eTimeSeconds = document.createElement("span");

	/** @type {HTMLCollectionOf<HTMLVideoElement>} */
	videos;

	/** @type {HTMLVideoElement} */
	videoShown;

	fps = 1;

	/** @type {number | null} */
	timeIntervalID = null;
	// -------

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
		this.eTimeSeconds.textContent = (
			Math.trunc(seconds).toString().padStart(2, "0") +
			":" +
			Math.trunc((seconds % 1) * 100).toString().padStart(2, "0")
		);
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

		this.eTimeSecondsIcon.className = "seconds icon";
		this.eTimeSeconds.className = "seconds time";
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

		// For compatibility with code that accesses the <video> directly
		videoNew.onplay = (() => this.play());
		videoNew.onpause = (() => this.pause());
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
