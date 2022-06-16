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
	let first = true;
	let activeTuple = [null, null, ""]; // (button, controlled element, sub-ID)
	return {
		/**
		 * @param {string} text
		 * @param {string} subID
		 */
		add: (text, subID, active = false) => {
			const controlledElement = document.getElementById(`${id}-${subID}`);
			const newButton = document.createElement('button');
			newButton.innerHTML = text;
			if(active) {
				activeTuple = [newButton, controlledElement, subID];
				newButton.classList.add('active');
			}
			newButton.onclick = () => {
				activeTuple[0].classList.remove('active');
				newButton.classList.add('active');
				onSwitch(activeTuple[1], controlledElement);
				activeTuple = [newButton, controlledElement, subID];
			};
			!first
				? bar.appendChild(document.createTextNode("•"))
				: first = false;
			bar.appendChild(newButton);
		},
		getActive: () => activeTuple,
	}
}

/**
 * @param {string} id DOM element that receives the switch bar
 */
function switchMultipleVideos(id) {
	const ret = switchMultiple(id, switchVideo);

	const frameStep = (fps, direction) => {
		const vid = ret.getActive()[1];
		vid.pause();
		vid.currentTime += (direction / fps);
	};
	const seekAndStop = (time) => {
		const vid = ret.getActive()[1];
		vid.pause();
		vid.currentTime = time;
	};
	/**
	 * @param {HTMLElement} containerID
	 * @param {number} fps
	 * @param {function | null} middleButton
	 */
	const addControls = (containerID, fps, middleButton = null) => {
		const prev = document.createElement('button');
		const next = document.createElement('button');

		prev.textContent = `< Previous frame`;
		next.textContent = `Next frame >`;

		prev.onclick = () => frameStep(fps, -1);
		next.onclick = () => frameStep(fps, +1);

		const container = document.getElementById(containerID);
		container.appendChild(prev);
		if(middleButton) {
			container.appendChild(document.createTextNode("•"));
			container.appendChild(middleButton());
		}
		container.appendChild(document.createTextNode("•"));
		container.appendChild(next);
	}
	return Object.assign({ frameStep, seekAndStop, addControls }, ret);
}
// ---------------------
