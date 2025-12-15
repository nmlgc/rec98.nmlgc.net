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
	dynamicCaptions?: HTMLCollectionOf<HTMLDivElement>;

	constructor(switchFunc: TabSwitchFunc) {
		super();
		this.switchFunc = switchFunc;
	}

	connectedCallback() {
		this.dynamicCaptions = this.parentElement?.parentElement?.querySelector(
			"figcaption.dynamic"
		)?.getElementsByTagName("div");
	}

	add(title: string | null, initiallyActive: boolean) {
		const i = this.count;
		const button = document.createElement("button");
		button.innerHTML = (`${i + 1}️⃣` + (title ? ` ${title}` : ''));
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
		if(this.dynamicCaptions) {
			for(let i = 0; i < this.dynamicCaptions.length; i++) {
				this.dynamicCaptions[i].style.visibility = (
					(i == index) ? "visible" : "hidden"
				);
			}
		}
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

class ReC98ChildSwitcher extends HTMLElement {
	/** Not the same as [children], as we have to exclude ReC98TabSwitcher and
	 * ReC98ParentInit. */
	switchableChildren: Array<Element> = [];

	tabSwitcher: ReC98TabSwitcher;
	childShown: (Element | null) = null;

	/**
	 * @returns `true` if the active child was changed
	 */
	showChild(index: number) {
		const childPrev = this.childShown;
		const childNew = this.switchableChildren[index];
		if(childPrev === childNew) {
			return false;
		}
		childPrev?.classList.remove("active");
		childNew.classList.add("active");
		this.childShown = childNew;
		return true;
	}

	init() {
		// The linked element might not be part of the DOM yet, so we can only look up its ID for
		// now.
		const linkedID = this.getAttribute("data-link");

		this.tabSwitcher = new ReC98TabSwitcher((i) => {
			this.focus();
			const ret = this.showChild(i);

			// Prevent infinite recursion
			if(linkedID && ret) {
				const linkedElement = document.getElementById(linkedID);
				if(linkedElement) {
					(linkedElement as ReC98ChildSwitcher).tabSwitcher.switchTo(i);
				}
			}
			return ret;
		});
		this.prepend(this.tabSwitcher);

		this.tabIndex = -1; // Receive `onkeydown` events from all children

		let activeSeen = false;
		for(let i = 0; i < this.children.length; i++) {
			const child = this.children[i];
			if(child.tagName.startsWith('REC98-')) {
				continue;
			}
			this.switchableChildren.push(child);
			const active = child.classList.contains("active");
			if(active) {
				activeSeen = true;
				this.showChild(this.switchableChildren.length - 1);
			}
			this.tabSwitcher.add(child.getAttribute("data-title"), active);
		}
		if(!activeSeen) {
			throw "No child marked as active.";
		}

		this.onclick = (() => this.focus());
		this.onkeydown = ((event) => {
			switch(virtualKey(event)) {
			case '←':	return this.tabSwitcher.keydownHandler(event, '↑');
			case '→':	return this.tabSwitcher.keydownHandler(event, '↓');
			}
			return this.tabSwitcher.keydownHandler(event);
		});
	}
};

window.customElements.define("rec98-tab-switcher", ReC98TabSwitcher);
window.customElements.define("rec98-child-switcher", ReC98ChildSwitcher);
