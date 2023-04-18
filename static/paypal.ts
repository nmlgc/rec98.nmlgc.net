"use strict";

let mailto_support = "support@nmlgc.net";

const cust_name = document.getElementById("cust_name") as HTMLSelectElement;
const cust_url = document.getElementById("cust_url") as HTMLSelectElement;

const form = document.querySelector("form")!;
const amount = document.getElementById("amount") as HTMLInputElement;
const discount = document.getElementById("discount") as HTMLSelectElement;
const discount_breakdown = document.getElementById("discount_breakdown")!;
const discount_sponsor = document.getElementById("discount_sponsor")!;
const roundup_amount = document.getElementById("roundup_amount")!;
const roundup_pushes = document.getElementById(	"roundup_pushes")!;
const roundup_noun = document.getElementById("roundup_noun")!;

const error = document.getElementById("error")!;

function HTMLSupportMail() {
	return `
<a href="mailto:` + mailto_support + `"><kbd>` + mailto_support + `</kbd></a>`;
}

/**
 * Must match the implementation in db_discount_offers.go!
 *
 * @param capRemainingBeforeAmount In €.
 * @param amount In €.
 * @param pushprice In €.
 * @param discountFraction Fraction of a push covered by the sponsor.
 * @returns Round-up € funded by the sponsor, limited to the cap.
 */
function discountRoundupValue(
	capRemainingBeforeAmount: number,
	amount: number,
	pushprice: number,
	discountFraction: number
) {
	const pushprice_discounted = (pushprice * (1 - discountFraction));
	const roundup_value = (pushprice - pushprice_discounted);
	return Math.min(
		((amount / pushprice_discounted) * roundup_value),
		(capRemainingBeforeAmount - amount)
	);
}

function isOneTime() {
	return onetime.checked;
}

function cycle() {
	return isOneTime() ? "onetime" : "monthly";
}

function validateForm(data, actions) {
	for (const el of form.querySelectorAll("input[required]")) {
		if (!(el as HTMLInputElement).reportValidity()) {
			actions.reject();
			return false;
		}
	}
	return true;
}

function startTransaction() {
	error.hidden = true;
	document.querySelector("html")!.classList.add("wait");
}

function endTransaction() {
	document.querySelector("html")!.classList.remove("wait");
}

function thankyou() {
	return form.submit();
}

async function sendIncoming(
	orderID: string, amount: number, discountID: number
) {
	let response = await fetch('/api/transaction-incoming', {
		method: 'post',
		headers: {
			'content-type': 'application/json'
		},
		body: JSON.stringify({
			ProviderSession: orderID,
			CustName: cust_name.value,
			CustURL: cust_url.value,
			Metric: metric.value,
			Goal: goal.value,
			Micro: micro.checked,
			Cycle: cycle(),
			Discount: discountID,
			Cents: amount * 100,
		})
	});
	if(!response.ok) {
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
		return actions.subscription.create(Object.assign({
			'plan_id': 'P-9AN20019EU9300502LW47CUI',
			'quantity': amount.value
		}, params_shared));
	},
	onApprove: async function(data, actions) {
		// For some reason, PayPal's /v2/checkout/orders/ API doesn't return
		// the subscription amount, so for now, let's just send it ourselves…
		// At least the server bails out if the order ID doesn't exist, so… 🤷
		await sendIncoming(data.orderID, Number(amount.value), 0);
	},
	onCancel: endTransaction,
	onClick: validateForm,
};

let order = {
	createOrder: function(data, actions) {
		startTransaction();
		return actions.order.create(Object.assign({
			purchase_units: [{ amount: { value: amount.value } }]
		}, params_shared));
	},
	onApprove: async function(data, actions) {
		await actions.order.capture();
		await sendIncoming(
			data.orderID, 0, (discount) ? discount.selectedIndex : 0
		);
	},
	onCancel: endTransaction,
	onClick: validateForm,
};

function clampNumber(obj) {
	obj.value = Math.max(obj.value, obj.min);
	obj.value = Math.min(obj.value, obj.max);
	return obj
}

function formatNumber(obj, digits) {
	return parseFloat(obj.value).toFixed(digits);
}

function onCycle() {
	let button_id = 'paypal-button-container';
	let button_selector = '#' + button_id;
	let button_container = document.getElementById(button_id)!;

	let push_amount = document.getElementById("push_amount")!;
	let push_noun = document.getElementById("push_noun")!;

	const pushprice = (Number(push_amount.dataset.price) / 100);

	const updatePushAmount = function(
		target_amount: HTMLElement, target_noun: HTMLElement, money: number
	) {
		target_amount.innerHTML = (
			(Math.round((money / pushprice) * 100) / 100).toString()
		);
		target_noun.innerHTML = ((money == pushprice) ? " push" : " pushes");
	}

	button_container.innerHTML = "";
	if(isOneTime()) {
		paypal.Buttons(order).render(button_selector);
		amount.onchange = function() {
			const value_from_customer = clampNumber(amount);
			updatePushAmount(push_amount, push_noun, Number(amount.value));
			amount.value = formatNumber(value_from_customer, 2);

			if(!discount) {
				return;
			}

			const discount_offer = discount.options[discount.selectedIndex];
			const discount_fraction = Number(discount_offer.dataset.fraction);
			if(discount_offer.index === 0) {
				discount_breakdown.hidden = true;
			} else {
				discount_breakdown.hidden = false;
				discount_sponsor.innerHTML = discount_offer.dataset.name!;
				const roundup_value = discountRoundupValue(
					Number(amount.max),
					Number(amount.value),
					pushprice,
					discount_fraction
				)
				updatePushAmount(roundup_pushes, roundup_noun, roundup_value);
				roundup_amount.innerHTML = valueInCurrency(roundup_value * 100);
			}
		}
		amount.min = "1.00";
		amount.step = "0.01";

		if(discount) {
			discount.disabled = false;
			discount_breakdown.hidden = false;
		}
	} else {
		paypal.Buttons(subscription).render(button_selector);
		amount.onchange = function() {
			amount.value = formatNumber(amount, 0);
			updatePushAmount(push_amount, push_noun, Number(amount.value));
		}
		amount.min = "1";
		amount.step = "1";

		if(discount) {
			discount.disabled = true;
			discount.selectedIndex = 0;
			discount_breakdown.hidden = true;
		}
	}
	amount.onchange();
}

onCycle();
