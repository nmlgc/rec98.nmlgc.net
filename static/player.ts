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
 * Generates the trackbar fraction for the timeline bar at a given frame.
 */
function timelineFractionAt(frame: number, frameCount: number) {
	return (frame / (frameCount - 1));
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
			player.seekToDiscrete(secondsFrom(frame, fps));
			player.focus();
		})

		this.style.left = `${timelineFractionAt(frame, frameCount) * 100}%`;
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

/**
 * Creates a function that maps a linear volume factor onto a more realistic
 * logarithmic scale with the given decibel range. Implements the scaled and
 * shifted variant described at
 * https://www.dr-lex.be/info-stuff/volumecontrols.html
 * to ensure that the returned curve goes through (0, 0) and (1, 1) – *not* the
 * linear roll-off variant that most of this page focuses on.
 */
const LinearToLog = (logDBRange: number) => {
	const factor = (10 ** (logDBRange / 20));
	const inv = (1 / factor);
	const power = Math.log(1 + factor);
	return ((linear: number) => ((Math.exp(power * linear) * inv) - inv));
};

// 40 dB maps a linear 0.5 to 0.1.
const linearToLog = LinearToLog(40);

abstract class ReC98Player extends HTMLElement {
	/** Should call `super.renderCustomTimeBase()`. */
	abstract renderCustomTime(seconds: number): void;

	/**
	 * @returns The given timeline fraction converted to seconds, or `null` if
	 * scrubbing is not possible yet.
	 */
	abstract secondsFromFraction(fraction: number): number | null;

	abstract currentTime(): number;
	abstract looping(): boolean;

	/**
	 * Seeks the element to the given position in seconds, as accurately as
	 * possible. Directly called when switching between elements.
	 */
	abstract seekToContinuous(seconds: number): void;

	/**
	 * Seeks the video to the closest discrete frame. Called during interactive
	 * seeking.
	 */
	abstract seekToDiscrete(seconds: number): void;

	/**
	 * @returns Object that can be `muted`, or `null` if muting doesn't make
	 * sense for the subclass.
	 */
	abstract muter(): { muted: boolean } | null;

	/**
	 * @returns Linear volume between 0.0 and 1.0.
	 */
	abstract volume(): number;

	abstract setVolume(linear: number): void;

	/** Should call `super.playbackStartBase(refreshRate)`. */
	abstract playbackStart(): void;

	/**
	 * Should call `super.playbackStopSubclass()`.
	 * @returns Previous playing state of the shown element.
	 **/
	abstract playbackStopSubclass(): boolean;

	/** Should call `super.uiPlayBase()`. */
	abstract uiPlay(): void;

	/**
	 * Should `return super.showBase(fps, losslessURL)` on success.
	 * @returns `true` if the played element was changed.
	 */
	abstract show(index: number): boolean;

	/**
	 * Called after connecting all common controls to the DOM.
	 * @returns Element collection, and whether to enable audio controls.
	 */
	abstract initSubclass(): [HTMLCollection, boolean];

	abstract preload(): void;

	// Members
	// -------

	eTabSwitcher: (ReC98TabSwitcher | null) = null;
	eTimeline: ReC98Trackbar;
	eVolumeBar: ReC98Trackbar;

	eElementWrap = document.createElement("div");
	eControls = document.createElement("div");
	ePlay = document.createElement("button");
	eTimeSecondsIcon = document.createElement("span");
	eHome = document.createElement("button");
	eRewind = document.createElement("button");
	eTimeSeconds = document.createElement("span");
	eFastForward = document.createElement("button");
	eEnd = document.createElement("button");
	eDownload = document.createElement("a");
	eVolume = document.createElement("button");
	eVolumeSymbol = document.createElement("span");
	eFullscreen = document.createElement("button");
	ePopups = document.createElement("div");

	duration: number | null = null;
	fps = 1;
	timeIntervalID: (number | null) = null;
	// -------

	playbackStartBase(refreshRate: number) {
		if(!this.timeIntervalID) {
			this.timeIntervalID = setInterval(
				(() => this.renderCurrentTime()), refreshRate,
			);
		}
	}

	playbackStop() {
		let intervalStopped = false;
		if(this.timeIntervalID) {
			intervalStopped = true;
			clearInterval(this.timeIntervalID);
			this.timeIntervalID = null;
		}
		let subclassStopped = this.playbackStopSubclass();
		return (intervalStopped || subclassStopped);
	}

	uiPlayBase() {
		this.ePlay.textContent = "⏸";
		this.ePlay.title = "Pause (Space)";
	}

	uiPause() {
		this.playbackStop();
		this.ePlay.textContent = "▶";
		this.ePlay.title = "Play (Space)";
	}

	seekBy(secondsDelta: number) {
		if(!this.duration) {
			return false;
		}
		let seconds = (this.currentTime() + secondsDelta);
		if(this.looping()) {
			// Preserve as much precision as possible.
			while(seconds < 0) {
				seconds = (this.duration + seconds);
			}
			if(seconds >= this.duration) {
				seconds %= this.duration;
			}
		} else {
			seconds = Math.min(Math.max(seconds, 0), this.duration);
		}
		this.playbackStop();
		return this.seekToDiscrete(seconds);
	}

	scrub(fraction: number) {
		const seconds = this.secondsFromFraction(fraction);
		if(!seconds) {
			return;
		}
		this.focus();
		this.renderCustomTime(seconds); // Immediate feedback
		this.seekToDiscrete(seconds);
	}

	toggle() {
		// This function might be called twice in very quick succession. As
		// `<video>.play()` is asynchronous in modern browsers, but immediately
		// sets `<video>.paused` to `false`, a second call checking for that
		// flag would call `<video>.pause()` while `<video>.play()` is still
		// running, leading to an infinite loop of "play() was interrupted by
		// pause()" exceptions. [this.timeIntervalID] is only set in
		// playbackStartBase(), and is therefore a more reliable indicator of
		// the current playing state.
		if(!this.timeIntervalID) {
			this.uiPlay();
		} else {
			this.uiPause();
		}
		this.focus();
		return true;
	}

	renderCustomTimeBase(seconds: number, timelineFraction: number) {
		this.eTimeSeconds.textContent = (
			Math.trunc(seconds).toString().padStart(2, "0") +
			":" +
			Math.trunc((seconds % 1) * 100).toString().padStart(2, "0")
		);
		this.eTimeline.setFraction(timelineFraction);
	}

	renderCurrentTime() {
		this.renderCustomTime(this.currentTime());
	}

	showPopup(text: string) {
		const ePopup = document.createElement("div");
		ePopup.className = "popup";
		ePopup.innerHTML = text;
		this.ePopups.appendChild(ePopup);
	}

	/**
	 * Should only be called from overrides of this method; call `handleKey()`
	 * to handle events.
	 */
	handleKeySwitch(key: VirtualKey) {
		switch(key) {
		case ' ':
			return this.toggle();
		case '⏮':
			// Should always seek to 00:00, even if the discrete frame 0 lies
			// on a slightly different timestamp.
			return this.seekToContinuous(0);
		case '⏭':
			return (this.duration
				? this.seekToDiscrete(this.duration - (1 / this.fps))
				: false
			);
		case '←':
			return this.seekBy(-(1 / this.fps));
		case '→':
			return this.seekBy(+(1 / this.fps));
		case '⏪':
			return (this.duration ? this.seekBy(-(this.duration / 10)) : false);
		case '⏩':
			return (this.duration ? this.seekBy(+(this.duration / 10)) : false);
		case '⛶':
			if(!this.onfullscreenchange) {
				this.onfullscreenchange = (() => {
					if(!document.fullscreenElement && "orientation" in screen) {
						screen.orientation.unlock();
					}
				});
			}
			if(
				!document.fullscreenElement &&
				!document['webkitFullscreenElement']
			) {
				(this['webkitRequestFullscreen']
					? this['webkitRequestFullscreen']()
					: this.requestFullscreen()
				);
				if("orientation" in screen) {
					const or = ((this.offsetWidth > this.offsetHeight)
						? "landscape"
						: "portrait"
					);
					screen.orientation.lock(or).catch(() => {});
				}
				this.focus();
			} else {
				(document['webkitExitFullscreen']
					? document['webkitExitFullscreen']()
					: document.exitFullscreen()
				);
			}
			return true;
		}
		return false;
	}

	handleKey(key: VirtualKey, event: Event) {
		if(this.handleKeySwitch(key)) {
			event.preventDefault();
		}
	}

	// Constant property initialization
	constructor() {
		super();
		this.eElementWrap.className = "wrap"
		this.eControls.className = "controls";
		this.ePopups.className = "popups";

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

		this.eHome.className = "seconds";
		this.eRewind.className = "seconds";
		this.eFastForward.className = "seconds";
		this.eEnd.className = "seconds";
		// ---------------

		// Time
		// ----
		this.eTimeSecondsIcon.textContent = "⌚";
		this.eTimeSecondsIcon.title = "Seconds";
		this.eTimeSeconds.title = "Seconds";

		this.eTimeSecondsIcon.className = "seconds icon";
		this.eTimeSeconds.className = "seconds time";
		// ----

		// Timeline
		// --------
		let wasPlayingBeforeScrub = false;

		this.eTimeline = new ReC98Trackbar({
			orientation: "horizontal",
			onMove: ((fraction) => this.scrub(fraction)),
			onStart: (() => wasPlayingBeforeScrub = this.playbackStop()),
			onStop: (() => (wasPlayingBeforeScrub && this.playbackStart())),
		})
		this.eTimeline.classList.add("timeline");
		// --------

		// Download
		// --------
		this.eDownload.textContent = "⍗";
		this.eDownload.title = "Lossless source file";
		this.eDownload.className = "large";
		// --------

		// Volume
		// ------
		this.eVolumeSymbol.textContent = "🔊";
		this.eVolume.title = "Volume";
		this.eVolume.className = "volume large";
		this.eVolume.onfocus = preventFocus;
		// ------

		// Fullscreen
		// ----------
		this.eFullscreen.textContent = "⛶";
		this.eFullscreen.title = "Toggle fullscreen (F)";
		this.eFullscreen.className = "large";
		this.eFullscreen.onclick = ((event) => this.handleKey('⛶', event));
		// ----------
	}

	showBase(fps: number, losslessURL: string) {
		this.fps = fps;
		this.eDownload.href = losslessURL;
		return true;
	}

	/**
	 * DOM initialization. Called from the separate ReC98VideoInit component
	 * because child components aren't visible during connectedCallback() yet…
	 */
	init() {
		this.tabIndex = -1;
		this.renderCustomTime(0);

		this.appendChild(this.eElementWrap);
		this.eElementWrap.appendChild(this.ePopups);

		this.eControls.appendChild(this.ePlay);
		this.eControls.appendChild(this.eTimeSecondsIcon);
		this.eControls.appendChild(this.eHome);
		this.eControls.appendChild(this.eRewind);
		this.eControls.appendChild(this.eTimeSeconds);
		this.eControls.appendChild(this.eFastForward);
		this.eControls.appendChild(this.eEnd);
		this.eControls.appendChild(this.eTimeline);
		this.eControls.appendChild(this.eDownload);
		this.eControls.appendChild(this.eFullscreen);
		this.appendChild(this.eControls);

		// Event handlers
		// --------------
		this.onkeydown = ((event) => {
			if(document.activeElement !== this) {
				return;
			}
			if(this.eTabSwitcher?.keydownHandler(event)) {
				return;
			}
			this.handleKey(virtualKey(event), event);
		});

		// Preloading the video is required for seeking to work before the
		// video has been play()ed the first time.
		this.onpointerenter = (() => {
			this.preload();
			this.onpointerenter = null;
		});

		this.ePlay.onclick = ((event) => this.handleKey(' ', event));
		this.eHome.onclick = ((event) => this.handleKey('⏮', event));
		this.eRewind.onclick = ((event) => this.handleKey('⏪', event));
		this.eFastForward.onclick = ((event) => this.handleKey('⏩', event));
		this.eEnd.onclick = ((event) => this.handleKey('⏭', event));
		// --------------

		const [elements, withAudio] = this.initSubclass();

		let lastChild: (number | null) = null;
		let requested: (number | null) = null;

		if(elements.length >= 2) {
			this.eTabSwitcher = new ReC98TabSwitcher((i) => {
				this.focus();
				return this.show(i);
			});
			this.prepend(this.eTabSwitcher);
			this.classList.add("with-switcher");
			this.eDownload.title = "Lossless source file of current tab";
		}

		for(let i = 0; i < elements.length; i++) {
			const element = elements[i];
			if(element.hasAttribute("data-active")) {
				requested = i;
			}
			this.eTabSwitcher?.add(
				attributeAsString(element, "data-title"), (i === requested)
			);
			lastChild = i;
		}
		if(lastChild === null) {
			throw "No child element found.";
		}

		this.show(requested ?? lastChild);
		this.uiPause();

		if(withAudio) {
			this.classList.add("with-audio");
			const muter = this.muter();
			const toggleBar = ((active: boolean) => (active
				? this.eVolumeBar.classList.add("active")
				: this.eVolumeBar.classList.remove("active")
			));
			const updateSymbol = (() => {
				const volume = this.volume();
				const muter = this.muter();
				this.eVolumeSymbol.textContent = (
					(muter ? muter.muted : (volume === 0.0)) ? "🔇" :
					(volume < 0.1) ? "🔉" :
					"🔊"
				);
			});

			this.eVolumeBar = new ReC98Trackbar({
				orientation: "vertical",
				onMove: ((fraction) => {
					this.setVolume(
						Math.min(Math.max(linearToLog(fraction), 0.0), 1.0)
					);
					this.eVolumeBar.setFraction(fraction);
					updateSymbol();
				}),
			});
			if(muter) {
				this.eVolumeSymbol.onclick = (() => {
					const muter = this.muter();
					muter!.muted = !muter?.muted;
					toggleBar(!muter!.muted);
					updateSymbol();
				});
				this.eVolume.onpointerenter = (() => {
					!(this.muter()?.muted) && toggleBar(true);
				});
			} else {
				this.eVolume.onpointerenter = (() => toggleBar(true));
			}
			this.eVolume.onpointerleave = (() => toggleBar(false));

			this.eVolume.appendChild(this.eVolumeSymbol);
			this.eVolume.appendChild(this.eVolumeBar);
			this.eControls.insertBefore(this.eVolume, this.eFullscreen);

			// Start with the volume bar shown to draw attention to the fact
			// that this video has audio
			toggleBar(true);
			this.eVolumeBar.props.onMove(0.5);
		}
	}
};

/** `<video>` content with support for frame-based seeking. */
class ReC98Video extends ReC98Player {
	eTimeFrameIcon = document.createElement("span");
	eFramePrevious = document.createElement("button");
	eTimeFrame = document.createElement("span");
	eFrameNext = document.createElement("button");

	videos: HTMLCollectionOf<HTMLVideoElement>;
	videoShown: HTMLVideoElement;
	switchingVideos = false;
	scrubPossible = false;
	frameCount = 0;

	renderCustomTime(seconds: number) {
		const frame = frameFrom(seconds, this.fps);
		const fraction = timelineFractionAt(frame, this.frameCount);
		this.eTimeFrame.textContent = frame.toString();
		super.renderCustomTimeBase(seconds, fraction);
	}

	secondsFromFraction(fraction: number) {
		if(!this.scrubPossible) {
			return null;
		}
		let frame = frameFrom((fraction * this.videoShown.duration), this.fps);
		frame = Math.min(Math.max(frame, 0), (this.frameCount - 1));
		return secondsFrom(frame, this.fps);
	}

	/**
	 * Raw currentTime→frame conversion.
	 */
	frame() {
		const seconds = ((this.videoShown) ? this.videoShown.currentTime : 0);
		return frameFrom(seconds, this.fps);
	}

	currentTime() {
		return this.videoShown.currentTime;
	}

	looping() {
		return this.videoShown.loop;
	}

	seekToContinuous(seconds: number) {
		if(this.videoShown.seeking) {
			this.videoShown.onseeked = (() => {
				this.renderCurrentTime();
				this.videoShown.currentTime = seconds;
				this.videoShown.onseeked = (() => this.renderCurrentTime());
			});
		} else {
			this.videoShown.currentTime = seconds;
		}
		return true;
	}

	seekToDiscrete(seconds: number) {
		// Since `currentTime` is continuous, every (1 / [fps]) time span within
		// the video can always correspond to at least two discrete frames. The
		// actual frame for a specific `currentTime` value is therefore defined
		// by the browser's rounding algorithm, and will certainly not match the
		// one used in frame(), which defines the number shown in [eTimeFrame].
		// As a result, the frame number is very likely to be incorrect half of
		// the time.
		// It doesn't matter during playback, where the frame number is
		// constantly updating anyway, but manifests itself in inconsistent
		// frame numbers when seeking a paused video, or whenever a non-looping
		// video finished playing.
		// As a workaround, we perform a round-trip conversion from seconds to
		// frames and back, which gets us back onto our discrete seek grid in
		// any case. This might lead to a noticeable jump back by one frame, but
		// is unfortunately the best compromise in view of what we're given
		// here.
		const frame = Math.min(
			frameFrom(seconds, this.fps), (this.frameCount - 1)
		);
		return this.seekToContinuous(secondsFrom(frame, this.fps));
	}

	muter() {
		return this.videoShown;
	}

	volume(): number {
		return this.videoShown.volume;
	}

	setVolume(linear: number) {
		this.videoShown.volume = linear;
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

	onPlay() {
		// Rewind if we're at the end of a non-looping video – otherwise,
		// playback would immediately pause again.
		if(!this.videoShown.loop && (this.frame() === (this.frameCount - 1))) {
			this.videoShown.currentTime = 0;
		}
		super.playbackStartBase(1000 / Math.max(this.fps, 10));
		super.uiPlayBase();
	}

	playbackStart() {
		this.videoShown.play();
	}

	playbackStopSubclass() {
		if(this.videoShown) {
			let ret = !this.videoShown.paused;
			this.videoShown.pause();
			return ret;
		}
		return false;
	}

	uiPlay() {
		this.playbackStart();
	}

	show(index: number) {
		const videoPrev = this.videoShown;
		const videoNew = this.videos[index];
		if((videoPrev === videoNew) || this.switchingVideos) {
			return false;
		}

		const seekedFunc = (() => this.renderCurrentTime());

		this.frameCount = attributeAsNumber(videoNew, "data-frame-count");
		this.videoShown = this.videos[index];
		this.duration = this.videoShown.duration;

		videoNew.oncanplay = (() => {
			// This can still be NaN above.
			this.duration = this.videoShown.duration;

			this.scrubPossible = true;
			videoNew.oncanplay = null;
		});
		videoNew.onplay = (() => this.onPlay());
		videoNew.onpause = (() => {
			this.uiPause();

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

			// Calling the raw pause() function on the <video> element is fine
			// here: its sole purpose is to save processing power for an
			// invisible video, so it makes sense to avoid the DOM manipulation
			// done in `this.uiPause()`.
			videoPrev.pause();

			videoNew.onseeked = (() => {
				videoNew.hidden = false;
				videoNew.volume = videoPrev.volume;
				videoNew.muted = videoPrev.muted;
				seekedFunc();
				videoPrev.hidden = true;
				if(!videoPrevPaused) {
					this.uiPlay();
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

			this.seekToContinuous(videoPrev.currentTime);
		}
		for(const marker of this.markers()) {
			marker.hidden = (marker.videoIndex !== index);
		}
		return super.showBase(
			attributeAsNumber(videoNew, "data-fps"),
			attributeAsString(videoNew, "data-lossless")
		);
	}

	preload() {
		for(const video of this.videos) {
			video.load();
			video.preload = "auto";
		}
	}

	constructor() {
		super();

		// Seeking buttons
		// ---------------

		const preventFocus = (() => this.focus());

		this.eFramePrevious.textContent = "⏴";
		this.eFramePrevious.title = "Previous frame (←️ / A / H)";
		this.eFramePrevious.onfocus = preventFocus;
		this.eFrameNext.textContent = "⏵";
		this.eFrameNext.title = "Next frame (→️ / D / L)";
		this.eFrameNext.onfocus = preventFocus;

		this.eFramePrevious.className = "frame previous";
		this.eFrameNext.className = "frame next";
		// ---------------

		// Time
		// ----
		this.eTimeFrameIcon.textContent = "🎞️";
		this.eTimeFrameIcon.title = "Frame";
		this.eTimeFrame.title = "Frame";

		this.eTimeFrameIcon.className = "frame icon";
		this.eTimeFrame.className = "frame time";
		// ----
	}

	initSubclass(): [HTMLCollection, boolean] {
		this.eFramePrevious.onclick = ((event) => this.handleKey('←', event));
		this.eFrameNext.onclick = ((event) => this.handleKey('→', event));

		this.eControls.appendChild(this.eTimeFrameIcon);
		this.eControls.appendChild(this.eFramePrevious);
		this.eControls.appendChild(this.eTimeFrame);
		this.eControls.appendChild(this.eFrameNext);

		const timelineWidth = this.eTimeline.getBoundingClientRect().width;

		// Reparent
		this.videos = this.getElementsByTagName("video");
		for(let i = 0; i < this.videos.length; i++) {
			this.eElementWrap.appendChild(this.videos[0]);
		}

		let withAudio = false;
		for(let i = 0; i < this.videos.length; i++) {
			const video = this.videos[i];
			video.onclick = ((event) => this.handleKey(' ', event));
			withAudio ||= video.hasAttribute("data-audio");

			// The backend code still enables controls just in case the user
			// runs with disabled JavaScript, so we're separately disabling
			// them here as we're about to replace them with our own.
			video.controls = false;
			if((this.videos.length !== 1) && !("active" in video.dataset)) {
				video.hidden = true;
			}

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
			}
		}

		// Bind resizeMarkers() so that it gets called with the right context,
		// and overwrite the old property so that we have a function reference
		// we can unregister from events later on.
		this.resizeMarkers = this.resizeMarkers.bind(this);
		document.addEventListener("fullscreenchange", this.resizeMarkers);
		document.addEventListener("webkitfullscreenchange", this.resizeMarkers);

		// Some browsers (*cough* Firefox) refuse to layout the timeline at the
		// above call to getBoundingClientRect() and just return 0. Just gotta
		// defer setting the correct marker widths in that case…
		if(timelineWidth === 0) {
			window.addEventListener("DOMContentLoaded", this.resizeMarkers);
		}
		return [this.videos, withAudio];
	}

	disconnectedCallback() {
		document.removeEventListener("fullscreenchange", this.resizeMarkers);
		document.removeEventListener("webkitfullscreenchange", this.resizeMarkers);
		window.removeEventListener("DOMContentLoaded", this.resizeMarkers);
	}
};

class ReC98Audio extends ReC98Player {
	eWaveform = document.createElement("img");
	eCursor = document.createElement("img");

	static ctx: AudioContext | null = null;
	static refreshRate: number | null = null;

	audios: HTMLCollectionOf<HTMLAudioElement>;
	audioBuffers: Array<AudioBuffer | Promise<void> | undefined>;
	audioSource: AudioBufferSourceNode | undefined;
	gain: GainNode | null = null;

	startTime = 0;
	seekTime: number | null = 0;
	indexShown: number;
	volume_: number;

	renderCustomTime(seconds: number) {
		const fraction = seconds / (this.duration ?? 1);
		this.eCursor.style.clipPath = `rect(0% ${fraction * 100}% 100% 0%)`;
		super.renderCustomTimeBase(seconds, fraction);
	}

	secondsFromFraction(fraction: number) {
		if(!this.duration) {
			return null;
		}
		return (Math.min(Math.max(fraction, 0), 1) * this.duration);
	}

	playing() {
		return (ReC98Audio.ctx?.state === "running");
	}

	currentTime() {
		if(!this.duration || !ReC98Audio.ctx) {
			return 0;
		} else if(this.seekTime != null) {
			return this.seekTime;
		}
		return ((ReC98Audio.ctx.currentTime - this.startTime) % this.duration);
	}

	looping() {
		return this.audios[this.indexShown].loop;
	}

	seekToContinuous(seconds: number) {
		const wasPlaying = this.playing();
		this.seekTime = (seconds % this.duration!);
		this.renderCustomTime(this.seekTime);
		wasPlaying && this.playbackStart();
	}

	seekToDiscrete(seconds: number) {
		return this.seekToContinuous(seconds);
	}

	muter() {
		return null;
	}

	volume(): number {
		return this.volume_;
	}

	setVolume(linear: number) {
		this.volume_ = linear;
		if(this.gain) {
			this.gain.gain.value = this.volume_;
		}
	}

	playbackStart() {
		const buf = this.audioBuffers[this.indexShown];
		if(!ReC98Audio.ctx || !this.gain || !buf || !("duration" in buf)) {
			return;
		}
		if(this.audioSource) {
			this.audioSource.stop();
		}

		const seconds = this.currentTime();
		this.gain.gain.value = this.volume_;
		this.audioSource = ReC98Audio.ctx.createBufferSource();
		this.audioSource.buffer = buf;
		this.audioSource.loop = this.looping();
		this.audioSource.connect(this.gain);
		this.startTime = (ReC98Audio.ctx.currentTime - seconds);
		this.audioSource.start(0, seconds);
		ReC98Audio.ctx.resume();

		this.seekTime = null;
		super.playbackStartBase(ReC98Audio.refreshRate ?? (1 / 10));
	}

	playbackStopSubclass() {
		let ret = this.playing();
		if(ReC98Audio.ctx && this.audioSource) {
			ReC98Audio.ctx.suspend();
			this.audioSource.stop();
		}
		return ret;
	}

	uiPlay() {
		const buf = this.audioBuffers[this.indexShown];
		if(buf && ("then" in buf)) {
			this.classList.add("wait");
			buf.then(() => {
				this.classList.remove("wait");
				this.uiPlay();
			});
			return;
		}
		this.playbackStart();
		super.uiPlayBase();
	}

	show(index: number) {
		const audioNew = this.audios[index];
		const waveNew = attributeAsString(audioNew, "data-waveform");
		if(this.eWaveform.src === waveNew) {
			return false;
		}
		this.indexShown = index;

		const buf = this.audioBuffers[index];
		if(buf && ("duration" in buf)) {
			this.duration = buf.duration;
		}

		if(this.eWaveform.src) {
			this.seekToContinuous(this.currentTime());
		}
		this.eWaveform.src = waveNew;
		this.eCursor.src = waveNew;
		return super.showBase(this.fps, audioNew.src);
	}

	preload() {
		if(!ReC98Audio.ctx) {
			ReC98Audio.ctx = new AudioContext();
			ReC98Audio.ctx.suspend();
		}
		this.gain = ReC98Audio.ctx.createGain();
		this.gain.connect(ReC98Audio.ctx.destination);
		for(let i = 0; i < this.audios.length; i++) {
			const audio = this.audios[i];
			this.audioBuffers[i] = fetchSane(audio.src!).then((response) => {
				if(!response || !("ok" in response)) {
					throw response.message;
				}
				return response.arrayBuffer();
			}).then((buffer) =>
				buffer ? ReC98Audio.ctx?.decodeAudioData(buffer) : undefined
			).then((buffer) => {
				this.audioBuffers[i] = buffer;
				this.duration = buffer?.duration ?? null;
			}).catch((reason) => {
				this.showPopup(
					`⚠️ Error loading <code>${audio.src}</code>: ${reason}`
				);
			})
			if(!ReC98Audio.refreshRate) {
				requestAnimationFrame((t1) => requestAnimationFrame((t2) => {
					ReC98Audio.refreshRate = (t2 - t1);
					this.fps = (1000 / ReC98Audio.refreshRate);
				}));
			} else {
				this.fps = (1000 / ReC98Audio.refreshRate);
			}
		}
	}

	constructor() {
		super();
		this.eCursor.className = "cursor";
	}

	initSubclass(): [HTMLCollection, boolean] {
		this.eWaveform.onclick = ((event) => this.handleKey(' ', event));
		this.eCursor.onclick = ((event) => this.handleKey(' ', event));

		this.eElementWrap.appendChild(this.eWaveform);
		this.eElementWrap.appendChild(this.eCursor);

		this.audios = this.getElementsByTagName("audio");
		this.audioBuffers = Array(this.audios.length);
		for(const audio of this.audios) {
			audio.hidden = true;
		}
		return [this.audios, true];
	}
};

window.customElements.define("rec98-video", ReC98Video);
window.customElements.define("rec98-video-marker", ReC98VideoMarker);
window.customElements.define("rec98-audio", ReC98Audio);
