"use strict";

let mailto_support = "support@nmlgc.net";
const form = document.querySelector("form");

function HTMLSupportMail() {
	return `
<a href="mailto:` + mailto_support + `"><kbd>` + mailto_support + `</kbd></a>`;
}

function isOneTime() {
	return document.getElementById("onetime").checked;
}

function cycle() {
	return isOneTime() ? "onetime" : "monthly";
}

function validateForm(data, actions) {
	for (const el of form.querySelectorAll("[required]")) {
		if (!el.reportValidity()) {
			actions.reject();
			return false;
		}
	}
	return true;
}

function startTransaction() {
	document.getElementById("error").hidden = true;
	document.querySelector("html").classList.add("wait");
}

function endTransaction() {
	document.querySelector("html").classList.remove("wait");
}

function thankyou() {
	return form.submit();
}

async function sendIncoming(orderID, amount) {
	let response = await fetch('/api/transaction-incoming', {
		method: 'post',
		headers: {
			'content-type': 'application/json'
		},
		body: JSON.stringify({
			PayPalID: orderID,
			CustName: document.getElementById("cust-name").value,
			CustURL: document.getElementById("cust-url").value,
			Metric: document.getElementById("metric").value,
			Goal: document.getElementById("goal").value,
			Cycle: cycle(),
			Cents: amount * 100,
		})
	});
	if(!response.ok) {
		let error = document.getElementById("error");
		error.innerHTML =
			"Something went wrong: " + await response.text() + "<br>" +
			"I should have received your order though, and will confirm it " +
			"as soon as I see it.";
		error.hidden = false;
		endTransaction();
	} else {
		thankyou();
	}
}

let params_shared = {
	application_context: {
		shipping_preference: 'NO_SHIPPING'
	}
};

let subscription = {
	createSubscription: function(data, actions) {
		startTransaction();
		let p = {
			purchase_units: [{
				amount: {
					value: document.getElementById("amount").value
				}
			}]
		};
		return actions.subscription.create(Object.assign({
			'plan_id': 'P-9AN20019EU9300502LW47CUI',
			'quantity': document.getElementById("amount").value
		}, params_shared));
	},
	onApprove: async function(data, actions) {
		// For some reason, PayPal's /v2/checkout/orders/ API doesn't return
		// the subscription amount, so for now, let's just send it ourselves…
		// At least the server bails out if the order ID doesn't exist, so… 🤷
		await sendIncoming(data.orderID, document.getElementById("amount").value);
	},
	onCancel: endTransaction,
	onClick: validateForm,
};

let order = {
	createOrder: function(data, actions) {
		startTransaction();
		return actions.order.create(Object.assign({
			purchase_units: [{
				amount: {
					value: document.getElementById("amount").value
				}
			}]
		}, params_shared));
	},
	onApprove: async function(data, actions) {
		await actions.order.capture();
		await sendIncoming(data.orderID, 0);
	},
	onCancel: endTransaction,
	onClick: validateForm,
};

function formatNumber(obj, digits) {
	obj.value = Math.max(obj.value, obj.min);
	obj.value = Math.min(obj.value, obj.max);
	return parseFloat(obj.value).toFixed(digits);
}

function onCycle() {
	let button_id = 'paypal-button-container';
	let button_selector = '#' + button_id;
	let button_container = document.getElementById(button_id);

	let amount = document.getElementById("amount");
	let push_amount = document.getElementById("push_amount");
	let push_noun = document.getElementById("push_noun");

	const updatePushes = function(amount) {
		const price = (push_amount.dataset.price / 100);
		push_amount.innerHTML = (Math.round((amount / price) * 100) / 100);
		push_noun.innerHTML = ((amount == price) ? " push" : " pushes");
	}

	button_container.innerHTML = "";
	if(isOneTime()) {
		paypal.Buttons(order).render(button_selector);
		amount.onchange = function() {
			amount.value = formatNumber(amount, 2);
			updatePushes(amount.value);
		}
		amount.min = 1.00;
		amount.step = 0.01;
	} else {
		paypal.Buttons(subscription).render(button_selector);
		amount.onchange = function() {
			amount.value = formatNumber(amount, 0);
			updatePushes(amount.value);
		}
		amount.min = 1;
		amount.step = 1;
	}
	amount.onchange();
}

onCycle();
