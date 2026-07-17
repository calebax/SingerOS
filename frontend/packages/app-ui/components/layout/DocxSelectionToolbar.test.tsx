import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { DocxSelectionToolbar } from "./DocxSelectionToolbar";

afterEach(() => {
	document.body.replaceChildren();
});

describe("DocxSelectionToolbar", () => {
	it("offers only expand and shorten instructions", () => {
		const onInstruction = vi.fn();
		render(
			<DocxSelectionToolbar
				anchor={{ x: 100, y: 200, width: 240, height: 30 }}
				busy={false}
				onInstruction={onInstruction}
			/>,
		);

		fireEvent.click(screen.getByRole("button", { name: /AI 编辑/ }));
		expect(screen.getAllByRole("button").map((button) => button.textContent)).toEqual([
			"AI 编辑",
			"扩写",
			"缩写",
		]);

		fireEvent.click(screen.getByRole("button", { name: "扩写" }));
		expect(onInstruction).toHaveBeenCalledWith("expand");
		fireEvent.click(screen.getByRole("button", { name: /AI 编辑/ }));
		fireEvent.click(screen.getByRole("button", { name: "缩写" }));
		expect(onInstruction).toHaveBeenCalledWith("shorten");
	});

	it("anchors inside the document scroll layer without recalculating while scrolling", () => {
		const container = document.createElement("div");
		Object.defineProperty(container, "getBoundingClientRect", {
			value: () => new DOMRect(40, 80, 800, 600),
		});
		container.scrollLeft = 20;
		container.scrollTop = 300;
		document.body.appendChild(container);

		render(
			<DocxSelectionToolbar
				anchor={{ x: 140, y: 280, width: 100, height: 24 }}
				portalContainer={container}
				busy={false}
				onInstruction={vi.fn()}
			/>,
		);

		const toolbar = container.querySelector<HTMLElement>("[data-docx-selection-toolbar]");
		expect(toolbar?.classList.contains("absolute")).toBe(true);
		expect(toolbar?.style.left).toBe("170px");
		expect(toolbar?.style.top).toBe("490px");

		container.scrollTop = 500;
		fireEvent.scroll(container);
		expect(toolbar?.style.top).toBe("490px");
	});
});
