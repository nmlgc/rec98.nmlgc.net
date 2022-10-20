/**
 * Returns whether this tab switching request was successful (`true`) or not
 * (`false`).
 *
 * @typedef {function(number): boolean} TabSwitchFunc
 */

// Generic tab switching component.
class ReC98TabSwitcher extends HTMLElement {
	activeIndex = -1;
	count = 0;

	/** @type {TabSwitchFunc} */
	switchFunc;

	/** @param {TabSwitchFunc} switchFunc */
	constructor(switchFunc) {
		super();
		this.switchFunc = switchFunc;
	}

	/**
	 * @param {string} title
	 * @param {boolean} initiallyActive
	 */
	add(title, initiallyActive) {
		const i = this.count;
		const button = document.createElement("button");
		button.innerHTML = `${i + 1}️⃣ ${title}`
		button.onclick = (() => {
			this.switchTo(i);
		});
		this.appendChild(button);
		this.count++;
		if(initiallyActive) {
			if(this.activeIndex !== -1) {
				throw "Defined two tabs as initially active.";
			}
			this.setActive(i);
		}
	}

	/** @param {number} index */
	setActive(index) {
		this.children[this.activeIndex]?.classList.remove("active");
		this.children[index].classList.add("active");
		this.activeIndex = index;
	}

	/** @param {number} index */
	switchTo(index) {
		this.switchFunc(index) && this.setActive(index);
	}

	/**
	 * @param {KeyboardEvent} event
	 * @returns {boolean} Whether this event was handled
	 */
	keydownHandler(event) {
		if(event.key >= `1` && event.key <= `${this.count}`) {
			this.switchTo(Number(event.key) - 1);
			return true;
		}
		switch(virtualKey(event)) {
		case '↑':
			this.switchTo(((this.activeIndex + this.count) - 1) % this.count);
			event.preventDefault(); // Prevents scrolling!
			break;
		case '↓':
			this.switchTo((this.activeIndex + 1) % this.count);
			event.preventDefault(); // Prevents scrolling!
			break;
		}
		return false;
	}
};

window.customElements.define("rec98-tab-switcher", ReC98TabSwitcher);
