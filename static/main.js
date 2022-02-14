"use strict";

// Currency
// --------
let numFmt = new Intl.NumberFormat(
	navigator.language || navigator.userLanguage,
	{style: 'currency', currency: 'EUR', minimumFractionDigits: 0 }
)

function formatCurrency(cents) {
	document.write(numFmt.format(cents / 100))
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
function switchVideoButton(id, subvid) {
	return `<button id="${id}-switch" onclick="switchVideo('${id}', ${subvid})"
		>(Switch to ${subvid === 1 ? "fixed version" : "original"})</button>`;
}

function switchVideoLink(id) {
	document.write(switchVideoButton(id, 1));
}

function switchVideo(id, subvid) {
	const vidOld = document.getElementById(`${id}-vid-${1 - subvid}`);
	const vidNew = document.getElementById(`${id}-vid-${subvid}`);
	const paused = vidOld.paused;
	vidOld.pause();
	vidNew.currentTime = vidOld.currentTime;
	vidNew.onseeked = () => {
		vidNew.onseeked = null;
		vidOld.classList.remove("active");
		vidNew.classList.add("active");
		!paused && vidNew.play();
		document.getElementById(`${id}-switch`).outerHTML = switchVideoButton(
			id, (1 - subvid)
		);
	}
}
// ---------------------
