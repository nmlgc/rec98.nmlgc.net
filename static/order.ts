"use strict";

let mailto_support = "support@nmlgc.net";

const form = document.querySelector("form")!;

const cust_name = document.getElementById("cust_name") as HTMLSelectElement;
const cust_url = document.getElementById("cust_url") as HTMLSelectElement;
const metric = document.getElementById("metric") as HTMLSelectElement;
const goal = document.getElementById("goal") as HTMLInputElement;
const onetime = document.getElementById("onetime") as HTMLInputElement;

const info = document.getElementById("info")!;
const info_text = document.getElementById("info_text")!;

const micro_container = document.getElementById("micro_container")!;
const micro = document.getElementById("micro") as HTMLInputElement;
const micro_available = document.getElementById("micro_available")!;

const amount = document.getElementById("amount") as HTMLInputElement;
const push_amount = document.getElementById("push_amount")!;
const push_noun = document.getElementById("push_noun")!;

const discount = document.getElementById("discount") as HTMLSelectElement;
const discount_breakdown = document.getElementById("discount_breakdown")!;
const discount_sponsor = document.getElementById("discount_sponsor")!;
const roundup_amount = document.getElementById("roundup_amount")!;
const roundup_pushes = document.getElementById("roundup_pushes")!;
const roundup_noun = document.getElementById("roundup_noun")!;

const error = document.getElementById("error")!;

const pushprice = (Number(push_amount.dataset.price) / 100);

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

let micro_previously_checked = false;
function handleSelect(option: HTMLOptionElement) {
	const goal_mandatory = option.getAttribute("data-goal-mandatory");
	const message = option.getAttribute("data-info");
	if(message) {
		metric.classList.add("info");
		info_text.innerHTML = message;
		info.style.removeProperty("display");
	} else {
		metric.classList.remove("info");
		info.style.display = "none";
	}

	if(option.hasAttribute("data-micro")) {
		if(micro.disabled) {
			micro.checked = micro_previously_checked;
		}
		micro.disabled = false;
		micro_available.textContent = "";
	} else {
		micro_previously_checked = micro.checked;
		micro.disabled = true;
		micro.checked = false;
		micro_available.textContent = " (not possible with this goal, full push required for delivery)";
	}

	goal.required = (goal_mandatory !== null);
	goal.reportValidity();
	micro_container.hidden = false;
}

function isOneTime() {
	return onetime.checked;
}

function updatePushAmount(
	target_amount: HTMLElement, target_noun: HTMLElement, money: number
) {
	target_amount.innerHTML = (
		(Math.round((money / pushprice) * 100) / 100).toString()
	);
	target_noun.innerHTML = ((money == pushprice) ? " push" : " pushes");
}

function onAmountChange() {
	const onetime = isOneTime();

	const val = (parseFloat(amount.value) || 0); // could be NaN
	const min = parseFloat(amount.min);
	const max = parseFloat(amount.max);
	amount.value = Math.max(Math.min(val, max), min).toFixed(
		(onetime ? 2 : 0)
	);
	updatePushAmount(push_amount, push_noun, Number(amount.value));

	if(!onetime || !discount) {
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
			max, Number(amount.value), pushprice, discount_fraction
		)
		updatePushAmount(roundup_pushes, roundup_noun, roundup_value);
		roundup_amount.innerHTML = valueInCurrency(roundup_value * 100);
	}
}

function onCycle() {
	const onetime = isOneTime();
	if(onetime) {
		amount.min = "1.00";
		amount.step = "0.01";

		if(discount) {
			discount.disabled = false;
			discount_breakdown.hidden = false;
		}
	} else {
		amount.min = "1";
		amount.step = "1";

		if(discount) {
			discount.disabled = true;
			discount.selectedIndex = 0;
			discount_breakdown.hidden = true;
		}
	}
	onAmountChange();
	paypalOnCycle(onetime);
}

function validateForm() {
	for (const el of form.querySelectorAll("input[required]")) {
		if (!(el as HTMLInputElement).reportValidity()) {
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

async function sendIncoming(provider_session: string) {
	let response = await fetch('/api/transaction-incoming', {
		method: 'post',
		headers: {
			'content-type': 'application/json'
		},
		body: JSON.stringify({
			ProviderSession: provider_session,
			CustName: cust_name.value,
			CustURL: cust_url.value,
			Metric: metric.value,
			Goal: goal.value,
			Micro: micro.checked,
			Cycle: (isOneTime() ? "onetime" : "monthly"),
			Discount: ((isOneTime() && discount) ? discount.selectedIndex : 0),
			Cents: (Number(amount.value)) * 100,
		})
	});
	if(!response.ok) {
		error.innerHTML =
			"Something went wrong: " + await response.text() + "<br>" +
			"I should have received your order though, and will confirm it " +
			"as soon as I see it.";
		error.hidden = false;
		endTransaction();
		return false;
	}
	return true;
}

window.onload = () => {
	handleSelect(metric.options[metric.selectedIndex]);
	amount.onchange = onAmountChange;
	onCycle();
}
