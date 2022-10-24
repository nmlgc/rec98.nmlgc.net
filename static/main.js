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
 * @param {string} id DOM element that receives the switch bar
 * @param {function} onSwitch Function called when clicking a switch button
 */
function switchMultipleVideos(id, onSwitch = switchVideo) {
	const ret = switchMultiple(id, onSwitch);

	return Object.assign({
		addMiddleButton: (containerID, middleButton) => {
			videoAddMiddleButton(containerID, middleButton);
		}
	}, ret);
}
// ---------------------

// Web Component initialization workaround
// ---------------------------------------

// Placed as the last child of a web component, this dummy element ensures that
// we only initialize the parent element once all its child nodes are connected
// to the DOM.
class ReC98ParentInit extends HTMLElement {
	connectedCallback() {
		if(!this.parentElement || (this.parentElement.lastChild !== this) || !(
			(this.parentElement.tagName === 'REC98-VIDEO')
		)) {
			throw "Must be placed at the last child of a supported element.";
		}
		/** @type {ReC98Video} */
		(this.parentElement).init();
	}
};

window.customElements.define("rec98-parent-init", ReC98ParentInit);
// ---------------------------------------

/**
 * Translates equivalent KeyboardEvents into a virtual key.
 *
 * @typedef {' ' | '←' | '→' | '↑' | '↓' | '⏮' | '⏭' | null} VirtualKey
 * @param {KeyboardEvent} event
 * @returns {VirtualKey}
 */
function virtualKey(event) {
	switch(event.code) {
	case "Space":
		return ' ';
	case "ArrowLeft":
	case "KeyA":
	case "KeyH":
		return '←';
	case "ArrowRight":
	case "KeyD":
	case "KeyL":
		return '→';
	case "ArrowUp":
	case "KeyW":
	case "KeyK":
		return '↑';
	case "ArrowDown":
	case "KeyS":
	case "KeyJ":
		return '↓';
	case "Home":
		return '⏮';
	case "End":
		return '⏭';
	}
	return null;
}
