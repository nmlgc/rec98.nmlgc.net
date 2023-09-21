interface ReC98TrackbarProps {
	onMove: ((fraction: number) => any);
	onStart?: (() => any);
	onStop?: (() => any);
}

// Generic trackbar component
class ReC98Trackbar extends HTMLElement {
	props: ReC98TrackbarProps;
	active = false;
	ePos = document.createElement("div");
	eBorder = document.createElement("div");

	constructor(props: ReC98TrackbarProps) {
		super();
		this.props = props;
		this.eBorder.className = "border";
		this.ePos.className = "pos";

		this.eBorder.appendChild(this.ePos);
		this.appendChild(this.eBorder);

		this.eBorder.onpointermove = ((event) =>
			// Why is the border included in the offset?!?
			this.active &&
			this.props.onMove((event.offsetX + 1) / this.offsetWidth)
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
		this.ePos.style.width = `${fraction * 100}%`;
	}
};

window.customElements.define("rec98-trackbar", ReC98Trackbar);
