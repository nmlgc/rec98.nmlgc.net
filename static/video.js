class ReC98Video extends HTMLElement {
	// Members
	// -------
	eControls = document.createElement("div");
	ePlay = document.createElement("button");

	/** @type {HTMLCollectionOf<HTMLVideoElement>} */
	videos;

	/** @type {HTMLVideoElement} */
	videoShown;
	// -------

	play() {
		this.videoShown.play();
		this.ePlay.textContent = "⏸";
		this.ePlay.title = "Pause (Space)";
	}

	pause() {
		this.videoShown?.pause();
		this.ePlay.textContent = "▶";
		this.ePlay.title = "Play (Space)";
	}

	/** @param {Event} event */
	toggle(event) {
		if(!this.videoShown) {
			return;
		}
		event.preventDefault();
		if(this.videoShown.paused) {
			this.play();
		} else {
			this.pause();
		}
		this.focus();
	}

	// Constant property initialization
	constructor() {
		super();
		this.eControls.className = "controls";

		// Play/Pause button
		// -----------------
		this.ePlay.className = "large";
		// -----------------
	}

	/**
	 * @param {number} index
	 */
	showVideo(index) {
		const videoNew = this.videos[index];

		this.videoShown = videoNew;

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
