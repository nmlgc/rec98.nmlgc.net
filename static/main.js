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
function enableExternal(id_consent, id_target, callback) {
	callback(document.getElementById(id_target));
	document.getElementById(id_consent).hidden = true;
}
// -------------------------
