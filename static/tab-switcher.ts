/**
 * Returns whether this tab switching request was successful (`true`) or not
 * (`false`).
 */
type TabSwitchFunc = (index: number) => boolean;

interface Layer {
	name: string;
	index: number;
	button: HTMLButtonElement;
	children: Layer[];
}

const hideLayer = (child: Layer) => (child.button.hidden = true);
const showLayer = (child: Layer) => (child.button.hidden = false);

// Generic tab switching component.
class ReC98TabSwitcher extends HTMLElement {
	rowDivs: HTMLDivElement[] = [];
	layerTree: Layer[] = [];
	buttonIDs: number[][] = []; // On each layer of the switcher, in order
	activeIndex = -1; // Across the entire switcher

	switchFunc: TabSwitchFunc;
	dynamicCaptions?: HTMLCollectionOf<HTMLDivElement>;

	constructor(layerCount: number, switchFunc: TabSwitchFunc) {
		super();
		this.switchFunc = switchFunc;
		for(let i = 0; i < layerCount; i++) {
			const div = document.createElement("div");
			div.classList.add("layer");
			this.appendChild(div);
			this.rowDivs.push(div);
		}
	}

	connectedCallback() {
		this.dynamicCaptions = this.parentElement?.parentElement?.querySelector(
			"figcaption.dynamic"
		)?.getElementsByTagName("div");
	}

	add(layers: string[]) {
		let count = this.buttonIDs.length;
		let buttonIDs = new Array<number>(this.rowDivs.length);
		let layerLevel = this.layerTree;
		for(let layerID = 0; layerID < layers.length; layerID++) {
			const name = layers[layerID];
			let indexOnLayer = layerLevel.findIndex((l) => (l.name == name));
			if(indexOnLayer == -1) {
				indexOnLayer = layerLevel.length;
			}
			buttonIDs[layerID] = indexOnLayer;
			if(indexOnLayer == layerLevel.length) {
				let layer = {
					name,
					index: count,
					button: document.createElement("button"),
					children: [],
				};
				layerLevel.push(layer);
				layer.button.hidden = true;
				if(layerID == (layers.length - 1)) {
					layer.button.innerHTML = (`${indexOnLayer + 1}️⃣` + (name ? ` ${name}` : ''));
					layer.button.onclick = (() => {
						this.switchTo(count);
					});
				} else {
					layer.button.innerHTML = name;
					const buttonIDsUpToThisLayer = buttonIDs.slice(0, (layerID + 1));
					layer.button.onclick = (() => {
						this.switchTo(this.findIndexForPartialIDs(buttonIDsUpToThisLayer));
					});
				}
				this.rowDivs[layerID].appendChild(layer.button);
			}
			layerLevel = layerLevel[indexOnLayer].children;
		}
		this.buttonIDs.push(buttonIDs);
	}

	findIndexForPartialIDs(ids: number[]) {
		// Defined layers
		let layerLevel = this.layerTree;
		for(const idOnLayer of ids) {
			layerLevel = layerLevel[idOnLayer].children;
		}

		// Remaining layers not given in [ids]
		const prevIDs = this.buttonIDs[this.activeIndex] ?? [];
		for(let layerI = ids.length; layerI < (prevIDs.length - 1); layerI++) {
			const idOnLayer = ((prevIDs[layerI] < layerLevel.length)
				? prevIDs[layerI]
				: 0
			);
			layerLevel = layerLevel[idOnLayer].children;
		}

		// Lowest layer
		const idOnLayer = ((prevIDs[prevIDs.length - 1] < layerLevel.length)
			? prevIDs[prevIDs.length - 1]
			: 0
		);
		return layerLevel[idOnLayer].index;
	}

	setActive(index: number) {
		const prevIDs = this.buttonIDs[this.activeIndex] ?? [];
		let layerLevel = this.layerTree;
		for(const idOnLayer of prevIDs) {
			layerLevel[idOnLayer].button.classList.remove("active");
			layerLevel.forEach(hideLayer);
			layerLevel = layerLevel[idOnLayer].children;
		}

		const newIDs = this.buttonIDs[index];
		layerLevel = this.layerTree;
		for(const idOnLayer of newIDs) {
			layerLevel[idOnLayer].button.classList.add("active");
			layerLevel.forEach(showLayer);
			layerLevel = layerLevel[idOnLayer].children;
		}

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
	 * @param forceLowest Translates ↑/↓ to ←/→ regardless of the number of actual layers
	 * @returns Whether this event was handled
	 */
	keydownHandler(event: KeyboardEvent, forceLowest: boolean | null = null) {
		const prevIDs = this.buttonIDs[this.activeIndex] ?? [];
		let layer1 = null;
		let layer0 = this.layerTree;
		for(let layerID = 0; layerID < (prevIDs.length - 1); layerID++) {
			layer1 = layer0;
			layer0 = layer0[prevIDs[layerID]].children;
		}

		if(event.key >= `1` && event.key <= `${layer0.length}`) {
			this.switchTo(layer0[Number(event.key) - 1].index);
			return true;
		}

		const total = this.buttonIDs.length;
		let key = virtualKey(event);
		if(forceLowest || !layer1) {
			if(key == '↑') {
				key = '←';
			} else if(key == '↓') {
				key = '→';
			}
		}
		switch(key) {
		case '↑': {
			const ids = prevIDs.slice(0, -1);
			ids[ids.length - 1] = (((ids[ids.length - 1] + layer1!.length) - 1) % layer1!.length);
			this.switchTo(this.findIndexForPartialIDs(ids))
			event.preventDefault(); // Prevents scrolling!
			break;
		}
		case '↓': {
			const ids = prevIDs.slice(0, -1);
			ids[ids.length - 1] = ((ids[ids.length - 1] + 1) % layer1!.length);
			this.switchTo(this.findIndexForPartialIDs(ids))
			event.preventDefault(); // Prevents scrolling!
			break;
		}
		case '←':
			this.switchTo(((this.activeIndex + total) - 1) % total);
			event.preventDefault(); // Prevents scrolling!
		break;
		case '→':
			this.switchTo((this.activeIndex + 1) % total);
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

		// Detect the amount of layers from the first child.
		let layerCount = 1;
		for(let i = 0; i < this.children.length; i++) {
			const child = this.children[i];
			if(child.tagName.startsWith('REC98-')) {
				continue;
			}
			while(child.getAttribute(`data-t${layerCount}`)) {
				layerCount++;
			}
			break;
		}

		this.tabSwitcher = new ReC98TabSwitcher(layerCount, (i) => {
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

		let activeIndex = null;
		for(let i = 0; i < this.children.length; i++) {
			const child = this.children[i];
			if(child.tagName.startsWith('REC98-')) {
				continue;
			}
			this.switchableChildren.push(child);
			const active = child.classList.contains("active");
			if(active) {
				activeIndex = (this.switchableChildren.length - 1);
				this.showChild(activeIndex);
			}

			const layers = new Array<string>(layerCount);
			for(let layerI = 0; layerI < layerCount; layerI++) {
				layers[layerI] = child.getAttribute(`data-t${layerI}`) ?? "";
			}

			this.tabSwitcher.add(layers);
		}
		if(activeIndex == null) {
			throw "No child marked as active.";
		}
		this.tabSwitcher.setActive(activeIndex);

		this.onclick = (() => this.focus());
		this.onkeydown = ((event) => this.tabSwitcher.keydownHandler(event));
	}
};

window.customElements.define("rec98-tab-switcher", ReC98TabSwitcher);
window.customElements.define("rec98-child-switcher", ReC98ChildSwitcher);
