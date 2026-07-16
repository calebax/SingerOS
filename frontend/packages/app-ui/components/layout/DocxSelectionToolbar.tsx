"use client";

import { ChevronDown, Minus, Plus, WandSparkles } from "lucide-react";
import { type ReactNode, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { DocxSelectionInstruction } from "./docx-selection-edit";
import type { OfficeRect } from "./office-selection";

type MenuState = "closed" | "open" | "closing";

export function DocxSelectionToolbar({
	anchor,
	portalContainer,
	busy,
	onInstruction,
}: {
	anchor: OfficeRect;
	portalContainer?: HTMLElement;
	busy: boolean;
	onInstruction: (instruction: DocxSelectionInstruction) => void;
}) {
	const [menuState, setMenuState] = useState<MenuState>("closed");
	const closeTimerRef = useRef<number | null>(null);
	const portalTarget = portalContainer ?? document.body;
	const position = useMemo(() => {
		if (!portalContainer) {
			return {
				left: anchor.x + anchor.width / 2,
				top: Math.max(12, anchor.y - 10),
				contained: false,
			};
		}
		const containerRect = portalContainer.getBoundingClientRect();
		return {
			left: anchor.x + anchor.width / 2 - containerRect.left + portalContainer.scrollLeft,
			top: anchor.y - containerRect.top + portalContainer.scrollTop - 10,
			contained: true,
		};
	}, [anchor, portalContainer]);

	useEffect(() => {
		return () => {
			if (closeTimerRef.current !== null) window.clearTimeout(closeTimerRef.current);
		};
	}, []);

	const closeMenu = () => {
		if (menuState !== "open") return;
		setMenuState("closing");
		const closeMs =
			Number.parseFloat(
				getComputedStyle(document.documentElement).getPropertyValue("--dropdown-close-dur"),
			) || 150;
		closeTimerRef.current = window.setTimeout(() => {
			setMenuState("closed");
			closeTimerRef.current = null;
		}, closeMs);
	};

	const toggleMenu = () => {
		if (menuState === "open") {
			closeMenu();
			return;
		}
		if (closeTimerRef.current !== null) window.clearTimeout(closeTimerRef.current);
		closeTimerRef.current = null;
		setMenuState("open");
	};
	const chooseInstruction = (instruction: DocxSelectionInstruction) => {
		closeMenu();
		onInstruction(instruction);
	};

	return createPortal(
		<div
			data-docx-selection-toolbar
			className={`${position.contained ? "absolute" : "fixed"} z-[70]`}
			style={{
				left: `${position.left}px`,
				top: `${position.top}px`,
				transform: "translate(-50%, -100%)",
			}}
			onPointerDown={(event) => event.preventDefault()}
		>
			<div className="relative">
				<button
					type="button"
					aria-expanded={menuState === "open"}
					disabled={busy}
					onClick={toggleMenu}
					className="inline-flex h-10 items-center gap-2 rounded-xl border border-slate-200 bg-white px-4 text-sm font-medium text-slate-800 shadow-lg hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
				>
					<WandSparkles className="size-4" />
					AI 编辑
					<ChevronDown className="size-3.5 text-slate-500" />
				</button>
				<div
					data-origin="top-center"
					aria-hidden={menuState !== "open"}
					className={`t-dropdown absolute left-1/2 top-full mt-2 w-40 -translate-x-1/2 overflow-hidden rounded-xl border border-slate-200 bg-white p-1.5 shadow-xl ${
						menuState === "open" ? "is-open" : menuState === "closing" ? "is-closing" : ""
					}`}
				>
					<InstructionButton
						icon={<Plus className="size-4" />}
						label="扩写"
						interactive={menuState === "open" && !busy}
						onClick={() => chooseInstruction("expand")}
					/>
					<InstructionButton
						icon={<Minus className="size-4" />}
						label="缩写"
						interactive={menuState === "open" && !busy}
						onClick={() => chooseInstruction("shorten")}
					/>
				</div>
			</div>
		</div>,
		portalTarget,
	);
}

function InstructionButton({
	icon,
	label,
	interactive,
	onClick,
}: {
	icon: ReactNode;
	label: string;
	interactive: boolean;
	onClick: () => void;
}) {
	return (
		<button
			type="button"
			tabIndex={interactive ? 0 : -1}
			disabled={!interactive}
			onClick={onClick}
			className="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-left text-sm text-slate-700 hover:bg-slate-100 hover:text-slate-950 disabled:opacity-60"
		>
			{icon}
			{label}
		</button>
	);
}
