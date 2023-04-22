async function stripe(button: HTMLButtonElement, label_id: string) {
	const label = document.getElementById(label_id);
	if(!label) {
		throw `#${label_id} not found.`
	}
	const label_html = label.innerHTML;
	button.disabled = true;
	label.innerHTML = "Initiating…";
	startTransaction();

	const response = await sendIncoming("stripe");
	if(response) {
		if(response.status == 204) {
			const url = response.headers.get("Location");
			if(url) {
				window.location.assign(url);

				// We're redirecting away to Stripe, no need for unnecessary
				// DOM manipulation.
				return;
			}
		}
		fetchSetError(response, "Expected a redirect to Stripe.", true);
	}
	endTransaction();
	button.disabled = false;
	label.innerHTML = label_html;
}

async function stripeCancel(
	button: HTMLButtonElement, endpoint: string, salt: string
) {
	const button_html = button.innerHTML;
	button.innerHTML = "⏳ Cancelling…";
	button.disabled = true;

	const form = document.getElementById("form_cancel") as HTMLFormElement;

	// Must be constructed before disabling!
	const urlparams = new URLSearchParams(new FormData(form));

	for(const child of form.getElementsByTagName("input")) {
		child.disabled = true;
	}
	const response = await fetchSane(endpoint, {
		method: "POST",

		// https://github.com/microsoft/TypeScript/issues/30584
		body: urlparams,
	});
	if(!response || !("ok" in response) || (response.status != 204)) {
		fetchSetError(response);
		for(const child of form.getElementsByTagName("input")) {
			child.disabled = false;
		}
		button.innerHTML = button_html;
		button.disabled = false;
		return;
	}
	button.innerHTML = "✔️ Cancelled";
	document.getElementById("error")!.hidden = true;
	document.getElementById("success")!.hidden = false;
}
