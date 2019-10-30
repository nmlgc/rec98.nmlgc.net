"use strict";

let mailto_support = "support@nmlgc.net";

function HTMLSupportMail() {
	return `<span class="emoji">📧</span>
<a href="mailto:` + mailto_support + `"><kbd>`+ mailto_support + `</kbd></a>`;
}

function isOneTime() {
	return true;
}

function cycle() {
	return isOneTime() ? "onetime" : "monthly";
}

function startTransaction() {
	document.getElementById("error").hidden = true;
	document.querySelector("html").classList.add("wait");
}

function thankyou() {
	return document.querySelector("form").submit();
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
		document.querySelector("html").classList.remove("wait");
	} else {
		thankyou();
	}
}

let params_shared = {
	application_context: {
		shipping_preference: 'NO_SHIPPING'
	}
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
	}
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

	button_container.innerHTML = "";
	if(isOneTime()) {
		paypal.Buttons(order).render(button_selector);
		amount.onchange = function() {
			amount.value = formatNumber(amount, 2);
		}
		amount.min = 1.00;
		amount.step = 0.01;
	} else {
	}
	amount.onchange();
}

onCycle();
