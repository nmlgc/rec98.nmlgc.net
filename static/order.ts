"use strict";

/** @type {HTMLSelectElement} */
const metric = document.getElementById("metric");

const goal = document.getElementById("goal");
const info = document.getElementById("info");
const info_text = document.getElementById("info_text");

const micro_container = document.getElementById("micro_container");

/** @type {HTMLInputElement} */
const micro = document.getElementById("micro");

const micro_available = document.getElementById("micro_available");

let micro_previously_checked = false;
function handleSelect(option) {
	const goal_mandatory = option.getAttribute("data-goal-mandatory");
	const message = option.getAttribute("data-info");
	if(message) {
		metric.classList.add("info");
		info_text.innerHTML = message;
		info.style.display = null;
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

window.onload = () => {
	handleSelect(metric.options[metric.selectedIndex]);
}
