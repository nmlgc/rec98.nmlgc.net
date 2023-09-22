interface ReC98TrackbarProps {
	orientation: ("horizontal" | "vertical"),
	onMove: ((fraction: number) => any);
	onStart?: (() => any);
	onStop?: (() => any);
}

// Generic trackbar component
class ReC98Trackbar extends HTMLElement {
	props: ReC98TrackbarProps;
	active = false;
	horizontal: boolean;
	ePos = document.createElement("div");
	eBorder = document.createElement("div");

	constructor(props: ReC98TrackbarProps) {
		super();
		this.props = props;
		this.horizontal = (props.orientation === "horizontal");
		this.classList.add(props.orientation);
		this.eBorder.className = "border";
		this.ePos.className = "pos";

		this.eBorder.appendChild(this.ePos);
		this.appendChild(this.eBorder);

		this.eBorder.onpointermove = ((event) =>
			// Why is the border included in the horizontal offset, but not the
			// vertical one?!?
			this.active && this.props.onMove(this.horizontal
				? ((event.offsetX + 1) / this.offsetWidth)
				: (1 - (event.offsetY / this.offsetHeight))
			)
		);
		this.eBorder.onpointerdown = ((event) => {
			if(event.button !== 0) {
				return;
			}
			this.active = true;
			this.props.onStart?.();
			this.eBorder.setPointerCapture(event.pointerId);
			this.eBorder.onpointermove?.(event);
		});
		this.eBorder.onpointerup = ((event) => {
			if(event.button !== 0) {
				return;
			}
			this.active = false;
			this.props.onStop?.();
		});
	}

	setFraction(fraction: number) {
		fraction = Math.min(Math.max(fraction, 0.0), 1.0);
		if(this.horizontal) {
			this.ePos.style.width = `${fraction * 100}%`;
		} else {
			this.ePos.style.height = `${(1 - fraction) * 100}%`;
		}
	}
};

window.customElements.define("rec98-trackbar", ReC98Trackbar);
