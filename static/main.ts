"use strict";

// Currency
// --------
const integerNumFmt = new Intl.NumberFormat(
	navigator.language || navigator['userLanguage'],
	{ style: 'currency', currency: 'EUR', minimumFractionDigits: 0 }
)

const fractionNumFmt = new Intl.NumberFormat(
	navigator.language || navigator['userLanguage'],
	{ style: 'currency', currency: 'EUR', minimumFractionDigits: 2 }
)

/**
 * @returns Formatted currency string.
 */
function valueInCurrency(cents: number) {
	return ((cents % 100) ? fractionNumFmt : integerNumFmt).format(cents / 100);
}

function formatCurrency(cents: number) {
	document.write(valueInCurrency(cents))
}
// --------

// Enabling external content
// -------------------------
var externals = {};
var consent_ids = {};

function externalRegister(day: string, target: string, url: string) {
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

function externalEnable(hostname: string) {
	for(let target_id in externals[hostname]) {
		(document.getElementById(target_id) as HTMLVideoElement).src = (
			externals[hostname][target_id]
		);
	}
	for(let consent_id of consent_ids[hostname]) {
		document.getElementById(consent_id)!.hidden = true;
	}
}
// -------------------------

// Web Component initialization workaround
// ---------------------------------------

// Placed as the last child of a web component, this dummy element ensures that
// we only initialize the parent element once all its child nodes are connected
// to the DOM.
class ReC98ParentInit extends HTMLElement {
	connectedCallback() {
		if(!this.parentElement || (this.parentElement.lastChild !== this) || !(
			(this.parentElement.tagName === 'REC98-VIDEO') ||
			(this.parentElement.tagName === 'REC98-IMAGE-SWITCHER')
		)) {
			throw "Must be placed at the last child of a supported element.";
		}
		(this.parentElement as (ReC98Video | ReC98ImageSwitcher)).init();

		// Let's not make the grid more complicated than it needs to be.
		this.parentElement.removeChild(this);
	}
};

window.customElements.define("rec98-parent-init", ReC98ParentInit);
// ---------------------------------------

type VirtualKey = (' ' | '←' | '→' | '↑' | '↓' | '⏮' | '⏭' | '⛶' | null);

/**
 * Translates equivalent KeyboardEvents into a virtual key.
 */
function virtualKey(event: KeyboardEvent): VirtualKey {
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
	case "KeyF":
		return '⛶'
	}
	return null;
}

// Fetching and error handling
// ---------------------------

type FetchSaneResult = (Response | TypeError);

async function fetchSane(
	input: (RequestInfo | URL), init?: RequestInit
): Promise<FetchSaneResult> {
	try {
		const ret = await fetch(input, init);
		return ret;
	} catch(e) {
		return e as TypeError;
	}
}

/**
 * Updates the `#error` element with a text generated from the given Fetch
 * error.
 */
async function fetchSetError(
	response: FetchSaneResult, extra = "", please_report = false
) {
	const error = document.getElementById("error");
	if(!error) {
		throw "Missing an #error element on the page.";
	}
	error.innerHTML = "🐞 Something went wrong: ";
	if(!("ok" in response)) {
		error.innerHTML += `<code>${response.name}: ${response.message}</code>`;
	} else {
		error.innerHTML += `<code>${await response.text()}</code>`;
	}
	error.innerHTML += `<br>${extra} `;
	if(please_report) {
		error.innerHTML += "Please contact me so that I can figure out the issue.";
	}
	error.hidden = false;
}
// ---------------------------
