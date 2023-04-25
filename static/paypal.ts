"use strict";

function paypalValidate(data, actions) {
	const ret = validateForm();
	!ret && actions.reject();
	return ret;
}

async function paypalSubmit(data, actions) {
	if(await sendIncoming(data.orderID)) {
		form.submit();
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
	onApprove: paypalSubmit,
	onCancel: endTransaction,
	onClick: paypalValidate,
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
		await paypalSubmit(data, actions);
	},
	onCancel: endTransaction,
	onClick: paypalValidate,
};

cycle_callbacks.push((onetime: boolean) => {
	const button_id = 'paypal_container';
	let button_selector = '#' + button_id;
	let button_container = document.getElementById(button_id)!;

	button_container.innerHTML = "";
	paypal.Buttons(onetime ? order : subscription).render(button_selector);
});

onCycle();
