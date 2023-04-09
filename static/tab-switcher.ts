/**
 * Returns whether this tab switching request was successful (`true`) or not
 * (`false`).
 */
type TabSwitchFunc = (index: number) => boolean;

// Generic tab switching component.
class ReC98TabSwitcher extends HTMLElement {
	activeIndex = -1;
	count = 0;
	switchFunc: TabSwitchFunc;

	constructor(switchFunc: TabSwitchFunc) {
		super();
		this.switchFunc = switchFunc;
	}

	add(title: string, initiallyActive: boolean) {
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

	setActive(index: number) {
		this.children[this.activeIndex]?.classList.remove("active");
		this.children[index].classList.add("active");
		this.activeIndex = index;
	}

	switchTo(index: number) {
		this.switchFunc(index) && this.setActive(index);
	}

	/**
	 * @param override Optional override for [event.key]
	 * @returns Whether this event was handled
	 */
	keydownHandler(event: KeyboardEvent, override: (VirtualKey | null) = null) {
		if(event.key >= `1` && event.key <= `${this.count}`) {
			this.switchTo(Number(event.key) - 1);
			return true;
		}
		switch(override ?? virtualKey(event)) {
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

class ReC98ImageSwitcher extends HTMLElement {
	images: HTMLCollectionOf<HTMLImageElement>;
	imageShown: (HTMLImageElement | null) = null;

	/**
	 * @returns `true` if the playing video was changed
	 */
	showImage(index: number) {
		const imagePrev = this.imageShown;
		const imageNew = this.images[index];
		if(imagePrev === imageNew) {
			return false;
		}
		imagePrev?.classList.remove("active");
		imageNew.classList.add("active");
		this.imageShown = imageNew;
		return true;
	}

	init() {
		const tabSwitcher = new ReC98TabSwitcher((i) => {
			this.focus();
			return this.showImage(i);
		});
		this.prepend(tabSwitcher);

		this.tabIndex = -1; // Receive `onkeydown` events from all children
		this.images = this.getElementsByTagName("img");

		let activeSeen = false;
		for(let i = 0; i < this.images.length; i++) {
			const image = this.images[i];
			const active = image.classList.contains("active");
			if(active) {
				activeSeen = true;
				this.showImage(i);
			}
			tabSwitcher.add(attributeAsString(image, "data-title"), active);
		}
		if(!activeSeen) {
			throw "No image marked as active.";
		}

		this.onclick = (() => this.focus());
		this.onkeydown = ((event) => {
			switch(virtualKey(event)) {
			case '←':	return tabSwitcher.keydownHandler(event, '↑');
			case '→':	return tabSwitcher.keydownHandler(event, '↓');
			}
			return tabSwitcher.keydownHandler(event);
		});
	}
};

window.customElements.define("rec98-tab-switcher", ReC98TabSwitcher);
window.customElements.define("rec98-image-switcher", ReC98ImageSwitcher);
