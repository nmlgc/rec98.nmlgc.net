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
