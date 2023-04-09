function attributeAsString(element: Element, attribute: string): string {
	const ret = element.getAttribute(attribute);
	if(!ret) {
		throw `${attribute} not given.`;
	}
	return ret;
}

function attributeAsNumber(element: Element, attribute: string): number {
	return Number(attributeAsString(element, attribute));
}

/**
 * Raw seconds→frame conversion.
 */
function frameFrom(seconds: number, fps: number) {
	return Math.floor(seconds * fps);
}

/**
 * Raw frame→currentTime conversion. Avoids rounding errors by returning the
 * middle of a frame.
 */
function secondsFrom(frame: number, fps: number) {
	return ((frame + 0.5) / fps);
}

/**
 * Generates the CSS `width` for the timeline bar at a given frame.
 */
function timelineWidthAt(frame: number, frameCount: number) {
	return `${(frame / (frameCount - 1)) * 100}%`;
}

class ReC98VideoMarker extends HTMLElement {
	button = document.createElement("button");
	videoIndex = -1;
	frameCount = 0;

	init(
		player: ReC98Video,
		videoIndex: number,
		timelineWidth: number,
		fps: number,
		frameCount: number
	) {
		const frame = attributeAsNumber(this, "data-frame");
		const title = attributeAsString(this, "data-title");
		const alignment = attributeAsString(this, "data-alignment");
		this.frameCount = frameCount;
		this.videoIndex = videoIndex;

		const onclick = (() => {
			player.seekTo(secondsFrom(frame, fps));
			player.focus();
		})

		this.style.left = timelineWidthAt(frame, frameCount);
		this.setWidth(timelineWidth);
		this.button.style[alignment] = "0";
		this.button.innerHTML = title;
		this.onclick = onclick;
		this.button.onclick = onclick;

		this.appendChild(this.button);
	}

	setWidth(timelineWidth: number) {
		const width = (timelineWidth / this.frameCount);

		this.style.width = `${Math.max(width, 1)}px`;
		this.button.style.borderWidth = `${Math.min(width, 3)}px`;
	}
};

class ReC98Video extends HTMLElement {
	// Members
	// -------

	eTabSwitcher: (ReC98TabSwitcher | null) = null;

	eVideoWrap = document.createElement("div");
	eControls = document.createElement("div");
	ePlay = document.createElement("button");
	eTimeSecondsIcon = document.createElement("span");
	eHome = document.createElement("button");
	eRewind = document.createElement("button");
	eTimeSeconds = document.createElement("span");
	eFastForward = document.createElement("button");
	eEnd = document.createElement("button");
	eTimeFrameIcon = document.createElement("span");
	eFramePrevious = document.createElement("button");
	eTimeFrame = document.createElement("span");
	eFrameNext = document.createElement("button");
	eTimeline = document.createElement("div");
	eTimelineBorder = document.createElement("div");
	eTimelinePos = document.createElement("div");
	eDownload = document.createElement("a");
	eFullscreen = document.createElement("button");

	videos: HTMLCollectionOf<HTMLVideoElement>;
	videoShown: HTMLVideoElement;
	dynamicCaptions?: HTMLCollectionOf<HTMLDivElement>;
	frameCount = 0;
	fps = 1;
	scrubPossible = false;
	switchingVideos = false;
	timeIntervalID: (number | null) = null;
	// -------

	/**
	 * Raw currentTime→frame conversion.
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

		// Rewind if we're at the end of a non-looping video – otherwise,
		// playback would immediately pause again.
		if(!this.videoShown.loop && (this.frame() === (this.frameCount - 1))) {
			this.videoShown.currentTime = 0;
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

	/**
	 * Returns the new currentTime after a seek by the given number of delta
	 * frames.
	 */
	frameSeekTime(frameDelta: number) {
		if(!this.videoShown) {
			return 0;
		}
		const frameNew = (this.frame() + frameDelta);
		return (secondsFrom(frameNew, this.fps) + (
			(frameNew <                0) ? +this.videoShown.duration :
			(frameNew >= this.frameCount) ? -this.videoShown.duration : 0
		));
	}

	/**
	 * Seeks the video to the given position, waiting for a previous seek to
	 * complete if necessary.
	 */
	seekTo(seconds: number) {
		if(this.videoShown.seeking) {
			this.videoShown.onseeked = (() => {
				this.renderTimeFromVideo();
				this.videoShown.currentTime = seconds;
				this.videoShown.onseeked = (() => this.renderTimeFromVideo());
			});
		} else {
			this.videoShown.currentTime = seconds;
		}
	}

	seekBy(frameDelta: number) {
		this.videoShown.pause();
		this.seekTo(this.frameSeekTime(frameDelta));
	}

	seekFast(direction: (-1 | 1)) {
		this.seekBy(direction * (this.frameCount / 10));
	}

	scrub(event: PointerEvent) {
		this.focus();

		// Why is the border width included in [offsetX]?!?
		const fraction = ((event.offsetX + 1) / this.eTimeline.offsetWidth);

		let frame = frameFrom((fraction * this.videoShown.duration), this.fps);
		frame = Math.min(Math.max(frame, 0), (this.frameCount - 1));
		const seconds = secondsFrom(frame, this.fps);
		this.renderTime(seconds); // Immediate feedback
		this.pause();
		this.seekTo(seconds);
	}

	toggle(event: Event) {
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

	renderTime(seconds: number) {
		const frame = frameFrom(seconds, this.fps);
		this.eTimeSeconds.textContent = (
			Math.trunc(seconds).toString().padStart(2, "0") +
			":" +
			Math.trunc((seconds % 1) * 100).toString().padStart(2, "0")
		);
		this.eTimeFrame.textContent = frame.toString();
		this.eTimelinePos.style.width = timelineWidthAt(frame, this.frameCount);
	}

	renderTimeFromVideo() {
		this.renderTime(this.videoShown.currentTime);
	}

	markers() {
		return this.eTimeline.getElementsByTagName(
			"rec98-video-marker"
		) as HTMLCollectionOf<ReC98VideoMarker>;
	}

	resizeMarkers() {
		const rect = this.eTimeline.getBoundingClientRect();
		for(const marker of this.markers()) {
			marker.setWidth(rect.width);
		}
	}

	// Constant property initialization
	constructor() {
		super();
		this.eVideoWrap.className = "video-wrap"
		this.eControls.className = "controls";

		// Play/Pause button
		// -----------------
		this.ePlay.className = "large";
		// -----------------

		// Seeking buttons
		// ---------------

		// Focused buttons prevent the arrow keys from working as intended, so
		// we always focus the main ReC98Video element instead.
		const preventFocus = (() => this.focus());

		this.eHome.textContent = "⏮"
		this.eHome.title = "First frame (Home)"
		this.eHome.onfocus = preventFocus;
		this.eEnd.textContent = "⏭"
		this.eEnd.title = "Last frame (End)"
		this.eEnd.onfocus = preventFocus;

		this.eRewind.textContent = "⏪";
		this.eRewind.title = "Rewind (Ctrl-←️ / Ctrl-A / Ctrl-H)";
		this.eRewind.onfocus = preventFocus;
		this.eFastForward.textContent = "⏩";
		this.eFastForward.title = "Fast forward (Ctrl-→️ / Ctrl-D / Ctrl-L)";
		this.eFastForward.onfocus = preventFocus;

		this.eFramePrevious.textContent = "⏴";
		this.eFramePrevious.title = "Previous frame (←️ / A / H)";
		this.eFramePrevious.onfocus = preventFocus;
		this.eFrameNext.textContent = "⏵";
		this.eFrameNext.title = "Next frame (→️ / D / L)";
		this.eFrameNext.onfocus = preventFocus;

		this.eHome.className = "seconds";
		this.eRewind.className = "seconds";
		this.eFastForward.className = "seconds";
		this.eEnd.className = "seconds";
		this.eFramePrevious.className = "frame previous";
		this.eFrameNext.className = "frame next";
		// ---------------

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

		// Timeline
		// --------
		this.eTimeline.className = "timeline";
		this.eTimelineBorder.className = "border";
		this.eTimelinePos.className = "pos";
		// --------

		// Download
		// --------
		this.eDownload.textContent = "⍗";
		this.eDownload.title = "Lossless source file";
		this.eDownload.className = "large";
		// --------

		// Fullscreen
		// ----------
		this.eFullscreen.textContent = "⛶";
		this.eFullscreen.title = "Toggle fullscreen (F)";
		this.eFullscreen.className = "large";
		this.eFullscreen.onclick = (() => {
			if(!this.parentElement) {
				return;
			}
			if(!this.parentElement.onfullscreenchange) {
				this.parentElement.onfullscreenchange = (() => {
					if(!document.fullscreenElement && "orientation" in screen) {
						screen.orientation.unlock();
					}
				})
			}
			if(!document.fullscreenElement && !document['webkitFullscreenElement']) {
				(this['webkitRequestFullscreen']
					? this.parentElement['webkitRequestFullscreen']()
					: this.parentElement.requestFullscreen()
				);
				if("orientation" in screen) {
					screen.orientation.lock((this.offsetWidth > this.offsetHeight)
						? "landscape"
						: "portrait"
					).catch(() => {});
				}
				this.focus();
			} else {
				(document['webkitExitFullscreen']
					? document['webkitExitFullscreen']()
					: document.exitFullscreen()
				);
			}
		});
		// ----------
	}

	/**
	 * @returns `true` if the playing video was changed
	 */
	showVideo(index: number) {
		const videoPrev = this.videoShown;
		const videoNew = this.videos[index];
		if((videoPrev === videoNew) || this.switchingVideos) {
			return false;
		}

		const seekedFunc = (() => this.renderTimeFromVideo());

		this.fps = attributeAsNumber(videoNew, "data-fps");
		this.frameCount = attributeAsNumber(videoNew, "data-frame-count");
		this.videoShown = this.videos[index];

		videoNew.oncanplay = (() => {
			this.scrubPossible = true;
			videoNew.oncanplay = null;
		})
		videoNew.onclick = ((event) => this.toggle(event));
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
			// frame numbers when seeking a paused video, or whenever a
			// non-looping video finished playing.
			// As a workaround, we perform a round-trip conversion from
			// `currentTime` to frames and back, which gets us back onto our
			// discrete seek grid in any case. This might lead to a noticeable
			// jump back by one frame, but is unfortunately the best compromise
			// in view of what we're given here.
			const frame = Math.min(
				frameFrom(videoNew.currentTime, this.fps), (this.frameCount - 1)
			);
			videoNew.currentTime = secondsFrom(frame, this.fps);
		});

		if(videoPrev && this.eTabSwitcher) {
			// If a user switches videos fast enough, they could easily hit an
			// ongoing seek where an otherwise playing video might still be
			// paused. This would cause the new video to stay paused when it
			// shouldn't be. Blocking a switch in this case works well enough,
			// and probably won't be noticeable at the switching speeds where
			// this becomes an issue.
			this.switchingVideos = true;

			// Pause the old video, but save the previous playing state to
			// decide whether to unpause the new video.
			const videoPrevPaused = videoPrev.paused;
			videoPrev.pause();
			videoNew.onseeked = (() => {
				videoNew.classList.add("active");
				seekedFunc();
				videoPrev.classList.remove("active");
				if(!videoPrevPaused) {
					videoNew.play();
				}
				this.switchingVideos = false;
				videoNew.onseeked = seekedFunc;
			});
		} else {
			videoNew.onseeked = seekedFunc;
		}

		if(videoPrev) {
			// Stop any further events from mutating [this.videoShown] based on
			// changes to the previous video.
			videoPrev.onplay = null;
			videoPrev.onpause = null;
			videoPrev.onseeked = null;

			this.seekTo(videoPrev.currentTime);
		}

		for(const marker of this.markers()) {
			marker.hidden = (marker.videoIndex !== index);
		}
		this.eDownload.href = attributeAsString(videoNew, "data-lossless");
		if(this.dynamicCaptions) {
			for(let i = 0; i < this.dynamicCaptions.length; i++) {
				this.dynamicCaptions[i].style.visibility = (
					(i == index) ? "visible" : "hidden"
				);
			}
		}
		return true;
	}

	/**
	 * DOM initialization. Called from the separate ReC98VideoInit component
	 * because child components aren't visible during connectedCallback() yet…
	 */
	init() {
		this.tabIndex = -1;

		this.appendChild(this.eVideoWrap);

		this.eTimelineBorder.appendChild(this.eTimelinePos);
		this.eTimeline.appendChild(this.eTimelineBorder);
		this.eControls.appendChild(this.ePlay);
		this.eControls.appendChild(this.eTimeSecondsIcon);
		this.eControls.appendChild(this.eHome);
		this.eControls.appendChild(this.eRewind);
		this.eControls.appendChild(this.eTimeSeconds);
		this.eControls.appendChild(this.eFastForward);
		this.eControls.appendChild(this.eEnd);
		this.eControls.appendChild(this.eTimeFrameIcon);
		this.eControls.appendChild(this.eFramePrevious);
		this.eControls.appendChild(this.eTimeFrame);
		this.eControls.appendChild(this.eFrameNext);
		this.eControls.appendChild(this.eTimeline);
		this.eControls.appendChild(this.eDownload);
		this.eControls.appendChild(this.eFullscreen);
		this.appendChild(this.eControls);

		// Event handlers
		// --------------
		let scrubActive = false;
		let scrubWasPaused = false;

		this.onkeydown = ((event) => {
			if(document.activeElement !== this) {
				return;
			}
			if(this.eTabSwitcher?.keydownHandler(event)) {
				return;
			}
			switch(virtualKey(event)) {
			case ' ':
				return this.ePlay.onclick?.(event);
			case '⏮':
				event.preventDefault();
				return this.eHome.onclick?.(event);
			case '⏭':
				event.preventDefault();
				return this.eEnd.onclick?.(event);
			case '←':
				return ((event.ctrlKey)
					? this.eRewind.onclick?.(event)
					: this.eFramePrevious.onclick?.(event)
				);
			case '→':
				return ((event.ctrlKey)
					? this.eFastForward.onclick?.(event)
					: this.eFrameNext.onclick?.(event)
				);
			case '⛶':
				return this.eFullscreen.onclick?.(event);
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
		this.eHome.onclick = (() => this.seekTo(0));
		this.eRewind.onclick = (() => this.seekFast(-1));
		this.eFastForward.onclick = (() => this.seekFast(+1));
		this.eEnd.onclick = (() =>
			this.seekTo(secondsFrom((this.frameCount - 1), this.fps))
		);
		this.eFramePrevious.onclick = (() => this.seekBy(-1));
		this.eFrameNext.onclick = (() => this.seekBy(+1));
		this.eTimelineBorder.onpointermove = ((event) => (
			scrubActive && this.scrubPossible && this.scrub(event)
		));
		this.eTimelineBorder.onpointerdown = ((event) => {
			if(event.button !== 0) {
				return;
			}
			scrubActive = true;
			scrubWasPaused = (this.videoShown?.paused ?? true);
			this.eTimelineBorder.setPointerCapture(event.pointerId);
			this.eTimelineBorder.onpointermove?.(event);
		});
		this.eTimelineBorder.onpointerup = ((event) => {
			if(event.button !== 0) {
				return;
			}
			scrubActive = false;
			!scrubWasPaused && this.play();
		});
		// --------------

		// Reparent videos
		// ---------------
		this.videos = this.getElementsByTagName("video");
		for(let i = 0; i < this.videos.length; i++) {
			this.eVideoWrap.appendChild(this.videos[0]);
		}
		// ---------------

		let lastChild = null;
		let requested = null;

		if(this.videos.length >= 2) {
			this.eTabSwitcher = new ReC98TabSwitcher((i) => {
				this.focus();
				return this.showVideo(i);
			});
			this.prepend(this.eTabSwitcher);
			this.classList.add("with-switcher");
			this.eDownload.title = "Lossless source file of current tab";
		}

		const timelineWidth = this.eTimeline.getBoundingClientRect().width;

		for(let i = 0; i < this.videos.length; i++) {
			const video = this.videos[i];
			video.onclick = ((event) => this.toggle(event));

			// The backend code still enables controls just in case the user
			// runs with disabled JavaScript, so we're separately disabling
			// them here as we're about to replace them with our own.
			video.controls = false;

			// Setup markers. Note that we mutate [markers] by reparenting its
			// elements; a `for…of` loop would therefore skip every second
			// marker.
			const fps = attributeAsNumber(video, "data-fps");
			const frameCount = attributeAsNumber(video, "data-frame-count");
			const markers = video.getElementsByTagName(
				"rec98-video-marker"
			) as HTMLCollectionOf<ReC98VideoMarker>;
			while(markers[0]) {
				markers[0].init(this, i, timelineWidth, fps, frameCount);
				this.eTimeline.appendChild(markers[0]);
				this.classList.add("with-markers");
			}

			if(video.classList.contains("active")) {
				requested = i;
			}
			this.eTabSwitcher?.add(
				attributeAsString(video, "data-title"), (i === requested)
			);
			lastChild = i;
		}
		if(lastChild === null) {
			throw "No <video> child element found.";
		}

		// Dynamic captions
		// ----------------
		const figcaption = this.parentElement?.querySelector(
			"figcaption.dynamic"
		);
		this.dynamicCaptions = figcaption?.getElementsByTagName("div");
		if(this.dynamicCaptions) {
			if(this.dynamicCaptions.length != this.videos.length) {
				throw `Expected ${this.videos.length} dynamic captions, but got ${this.dynamicCaptions.length}.`;
			}
		}
		// ----------------

		this.showVideo(requested ?? lastChild);
		this.pause();

		// Bind resizeMarkers() so that it gets called with the right
		// context, and overwrite the old property so that we have a
		// function reference we can unregister from events later on.
		this.resizeMarkers = this.resizeMarkers.bind(this);
		document.addEventListener("fullscreenchange", this.resizeMarkers);
		document.addEventListener("webkitfullscreenchange", this.resizeMarkers);
		// Some browsers (*cough* Firefox) refuse to layout the timeline at the
		// above call to getBoundingClientRect() and just return 0. Just gotta
		// defer setting the correct marker widths in that case…
		if(timelineWidth === 0) {
			window.addEventListener("DOMContentLoaded", this.resizeMarkers);
		}
	}

	disconnectedCallback() {
		document.removeEventListener("fullscreenchange", this.resizeMarkers);
		document.removeEventListener("webkitfullscreenchange", this.resizeMarkers);
		window.removeEventListener("DOMContentLoaded", this.resizeMarkers);
	}
};

window.customElements.define("rec98-video", ReC98Video);
window.customElements.define("rec98-video-marker", ReC98VideoMarker);
