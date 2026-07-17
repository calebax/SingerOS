import { act, fireEvent, render, waitFor } from "@testing-library/react";
import { createElement } from "react";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { getOfficeOpenXmlFormat, OfficePreview } from "./OfficePreview";

type MockXlsxViewerOptions = {
	onSelectionChange?: (selection: {
		anchor: { row: number; col: number };
		active: { row: number; col: number };
		mode: "cells" | "rows" | "cols" | "all";
	}) => void;
};

const xlsxViewerMock = vi.hoisted(() => ({
	options: undefined as MockXlsxViewerOptions | undefined,
}));

vi.mock("@silurus/ooxml/docx", () => ({
	DocxDocument: {
		load: vi.fn(async () => ({
			pageCount: 1,
			renderPage: async (
				canvas: HTMLCanvasElement,
				_pageIndex: number,
				options: {
					onTextRun?: (run: {
						text: string;
						x: number;
						y: number;
						w: number;
						h: number;
						fontSize: number;
						font: string;
					}) => void;
				},
			) => {
				canvas.style.width = "640px";
				canvas.style.height = "800px";
				options.onTextRun?.({
					text: "Selectable text",
					x: 40,
					y: 80,
					w: 120,
					h: 24,
					fontSize: 16,
					font: "16px Arial",
				});
			},
			destroy: vi.fn(),
		})),
	},
}));

vi.mock("@silurus/ooxml/xlsx", () => ({
	XlsxViewer: class XlsxViewer {
		constructor(container: HTMLElement, options: MockXlsxViewerOptions) {
			xlsxViewerMock.options = options;
			const root = document.createElement("div");
			const surface = document.createElement("div");
			const selectionOverlay = document.createElement("div");
			selectionOverlay.style.zIndex = "1";
			selectionOverlay.style.pointerEvents = "none";
			const selectionBox = document.createElement("div");
			selectionBox.style.border = "2px solid #1a73e8";
			Object.defineProperty(selectionBox, "getBoundingClientRect", {
				value: () => new DOMRect(120, 160, 240, 80),
			});
			Object.defineProperty(selectionOverlay, "getBoundingClientRect", {
				value: () => new DOMRect(80, 100, 800, 600),
			});
			selectionOverlay.appendChild(selectionBox);
			surface.appendChild(selectionOverlay);
			root.appendChild(surface);
			container.appendChild(root);
		}

		async load() {
			return undefined;
		}

		destroy() {
			return undefined;
		}
	},
}));

beforeAll(() => {
	vi.stubGlobal(
		"ResizeObserver",
		class ResizeObserver {
			observe() {
				return undefined;
			}
			disconnect() {
				return undefined;
			}
		},
	);
});

afterEach(() => {
	window.getSelection()?.removeAllRanges();
	xlsxViewerMock.options = undefined;
});

describe("getOfficeOpenXmlFormat", () => {
	it.each([
		["report.docx", "", "docx"],
		["budget.XLSX", "", "xlsx"],
		["slides.pptx", "", "pptx"],
		[
			"download",
			"application/vnd.openxmlformats-officedocument.presentationml.presentation",
			"pptx",
		],
	])("detects %s as %s", (fileName, mimeType, expected) => {
		expect(getOfficeOpenXmlFormat(fileName, mimeType)).toBe(expected);
	});

	it("does not classify legacy Office formats as OOXML", () => {
		expect(getOfficeOpenXmlFormat("legacy.xls", "application/vnd.ms-excel")).toBeNull();
		expect(getOfficeOpenXmlFormat("legacy.doc", "application/msword")).toBeNull();
	});
});

describe("OfficePreview text selection", () => {
	it("renders a DOCX text layer and emits render-relative selection coordinates", async () => {
		const onTextSelectionChange = vi.fn();
		const view = render(
			createElement(OfficePreview, {
				buffer: new ArrayBuffer(8),
				fileName: "sample.docx",
				format: "docx",
				onTextSelectionChange,
			}),
		);

		const run = await waitFor(() => {
			const element = view.container.querySelector<HTMLElement>("[data-office-run-index='0']");
			expect(element).not.toBeNull();
			return element as HTMLElement;
		});
		const surface = run.closest<HTMLElement>("[data-office-surface-index='0']");
		expect(surface).not.toBeNull();
		Object.defineProperty(surface, "getBoundingClientRect", {
			value: () => new DOMRect(100, 200, 640, 800),
		});

		const range = document.createRange();
		range.setStart(run.firstChild as Text, 0);
		range.setEnd(run.firstChild as Text, 10);
		Object.defineProperty(range, "getClientRects", {
			value: () => [new DOMRect(140, 280, 90, 24)],
		});
		window.getSelection()?.addRange(range);
		fireEvent.pointerUp(run);

		await waitFor(() => {
			expect(onTextSelectionChange).toHaveBeenLastCalledWith(
				expect.objectContaining({
					format: "docx",
					text: "Selectable",
					surfaceIndex: 0,
					boundingRect: { x: 140, y: 280, width: 90, height: 24 },
				}),
			);
		});

		const viewport = view.container.querySelector<HTMLElement>("[data-office-scroll-viewport]");
		expect(viewport).not.toBeNull();
		fireEvent.pointerDown(viewport as HTMLElement);
		expect(window.getSelection()?.rangeCount).toBe(0);
		expect(onTextSelectionChange).toHaveBeenLastCalledWith(null);
	});

	it("emits XLSX cell text, A1 range, and the rendered selection box", async () => {
		const xlsx = await import("xlsx");
		const workbook = xlsx.utils.book_new();
		xlsx.utils.book_append_sheet(
			workbook,
			xlsx.utils.aoa_to_sheet([
				["Alpha", "Beta"],
				["Gamma", "Delta"],
			]),
			"Data",
		);
		const buffer = xlsx.write(workbook, { type: "array", bookType: "xlsx" }) as ArrayBuffer;
		const onTextSelectionChange = vi.fn();

		render(
			createElement(OfficePreview, {
				buffer,
				fileName: "sample.xlsx",
				format: "xlsx",
				onTextSelectionChange,
			}),
		);
		await waitFor(() => expect(xlsxViewerMock.options).toBeDefined());

		act(() => {
			xlsxViewerMock.options?.onSelectionChange?.({
				anchor: { row: 1, col: 1 },
				active: { row: 2, col: 2 },
				mode: "cells",
			});
		});

		expect(onTextSelectionChange).toHaveBeenLastCalledWith(
			expect.objectContaining({
				format: "xlsx",
				text: "Alpha\tBeta\nGamma\tDelta",
				surfaceKind: "sheet",
				boundingRect: { x: 120, y: 160, width: 240, height: 80 },
				segments: [
					expect.objectContaining({
						sheetName: "Data",
						startCell: "A1",
						endCell: "B2",
					}),
				],
			}),
		);
	});
});
