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
