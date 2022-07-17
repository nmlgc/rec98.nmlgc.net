"use strict";

// Currency
// --------
const integerNumFmt = new Intl.NumberFormat(
	navigator.language || navigator.userLanguage,
	{ style: 'currency', currency: 'EUR', minimumFractionDigits: 0 }
)

const fractionNumFmt = new Intl.NumberFormat(
	navigator.language || navigator.userLanguage,
	{ style: 'currency', currency: 'EUR', minimumFractionDigits: 2 }
)

/**
 * @param {number} cents
 * @returns {string} Formatted currency string.
 */
function valueInCurrency(cents) {
	return ((cents % 100) ? fractionNumFmt : integerNumFmt).format(cents / 100);
}

/**
 * @param {number} cents
 */
function formatCurrency(cents) {
	document.write(valueInCurrency(cents))
}
// --------

// Enabling external content
// -------------------------
var externals = {};
var consent_ids = {};

function externalRegister(day, target, url) {
	const hostname = new URL(url).hostname;
	const target_id = `${day}-${target}`;
	const consent_id = `${day}-consent`;
	if(externals[hostname] === undefined) {
		externals[hostname] = {}
	}
	if(consent_ids[hostname] === undefined) {
		consent_ids[hostname] = []
	}
	if(document.getElementById(consent_id) === null) {
		document.write(`
			<a
				href="#${consent_id}"
				id="${consent_id}"
				onClick="externalEnable('${hostname}'); return false;"
			>(click to enable external content from <code>${hostname}</code>)
			</a>`
		);
		consent_ids[hostname].push(consent_id);
	}
	if(externals[hostname][target_id] !== undefined) {
		alert(
			'Target ID "#' + target_id + '" already registered as \"'
			+ externals[hostname][target_id] + '"'
		)
		return;
	}
	externals[hostname][target_id] = url;
}

function externalEnable(hostname) {
	for(let target_id in externals[hostname]) {
		document.getElementById(target_id).src = externals[hostname][target_id];
	}
	for(let consent_id of consent_ids[hostname]) {
		document.getElementById(consent_id).hidden = true;
	}
}
// -------------------------

// Multi-layer galleries
// ---------------------
/**
 * @param {HTMLButtonElement} button
 * @param {HTMLVideoElement[]} vids
 * @param {0 | 1} subvid_on_click
 */
function switchVideoButtonSet(button, vids, subvid_on_click) {
	button.textContent = `(Switch to ${
		subvid_on_click === 1 ? "fixed version" : "original"
	})`;
	button.onclick = () => {
		switchVideo(vids[1 - subvid_on_click], vids[subvid_on_click]);
		switchVideoButtonSet(button, vids, (1 - subvid_on_click));
	};
}

/**
 * @param {string} id DOM element that receives the button
 */
function switchVideoButton(id) {
	const vids = [
		document.getElementById(`${id}-0`), document.getElementById(`${id}-1`),
	];
	const button = document.createElement('button');
	switchVideoButtonSet(button, vids, 1);
	document.getElementById(id).prepend(button);
}

/**
 * @param {HTMLVideoElement} vidOld
 * @param {HTMLVideoElement} vidNew
 */
function switchVideo(vidOld, vidNew) {
	const paused = vidOld.paused;
	vidOld.pause();
	vidNew.currentTime = vidOld.currentTime;
	vidNew.onseeked = () => {
		vidNew.onseeked = null;
		vidOld.classList.remove('active');
		vidNew.classList.add('active');
		!paused && vidNew.play();
	}
}

/**
 * @param {string} id DOM element that receives the switch bar
 */
function switchMultiple(id, onSwitch = (elmOld, elmNew) => {}) {
	const bar = document.getElementById(id);
	const captionContainer = document.getElementById(`${id}-caption`);
	let first = true;
	let activeTuple = [null, null, ""]; // (button, controlled element, sub-ID)
	return {
		/**
		 * @param {string} text
		 * @param {string} subID
		 * @param {string?} caption
		 */
		add: (text, subID, active = false, caption = null) => {
			if(!captionContainer && caption) {
				alert(`Caption given, but no container defined (${caption})`);
				return;
			}
			const controlledElement = document.getElementById(`${id}-${subID}`);
			const newButton = document.createElement('button');
			newButton.innerHTML = text;
			if(active) {
				activeTuple = [newButton, controlledElement, subID];
				newButton.classList.add('active');
				caption && (captionContainer.innerHTML = caption);
			}
			newButton.onclick = () => {
				activeTuple[0].classList.remove('active');
				newButton.classList.add('active');
				caption && (captionContainer.innerHTML = caption);
				onSwitch(activeTuple[1], controlledElement);
				activeTuple = [newButton, controlledElement, subID];
			};
			!first
				? bar.appendChild(document.createTextNode("•"))
				: first = false;
			bar.appendChild(newButton);
			return controlledElement;
		},
		getActive: () => activeTuple,
	}
}

/**
 *
 * @param {HTMLVideoElement} vid
 * @param {number} time
 */
function videoSeekAndStop(vid, time) {
	vid.pause();
	vid.currentTime = time;
}

/**
 *
 * @param {HTMLVideoElement} vid
 * @param {number} fps
 * @param {-1 | +1} direction
 */
function videoFrameStep(vid, fps, direction) {
	videoSeekAndStop(vid, (vid.currentTime + (direction / fps)));
}

/**
  * @param {HTMLElement} containerID
  * @param {number} fps
  * @param {function} vidFunc Function that returns the <video> to operate on
  * @param {function | null} middleButton
  */
function videoAddControls(containerID, fps, vidFunc, middleButton = null) {
	const prev = document.createElement('button');
	const next = document.createElement('button');

	prev.textContent = `< Previous frame`;
	next.textContent = `Next frame >`;

	prev.onclick = () => videoFrameStep(vidFunc(), fps, -1);
	next.onclick = () => videoFrameStep(vidFunc(), fps, +1);

	const container = document.getElementById(containerID);
	container.appendChild(prev);
	if(middleButton) {
		container.appendChild(document.createTextNode("•"));
		container.appendChild(middleButton());
	}
	container.appendChild(document.createTextNode("•"));
	container.appendChild(next);
}

/**
 * @param {string} id DOM element that receives the switch bar
 * @param {function} onSwitch Function called when clicking a switch button
 */
function switchMultipleVideos(id, onSwitch = switchVideo) {
	const ret = switchMultiple(id, onSwitch);

	const vidFunc = () => ret.getActive()[1];

	return Object.assign({
		seek: (time) => vidFunc().currentTime = time,
		seekAndStop: (time) => videoSeekAndStop(vidFunc(), time),
		addControls: (containerID, fps, middleButton = null) => {
			videoAddControls(containerID, fps, vidFunc, middleButton);
		}
	}, ret);
}
// ---------------------
