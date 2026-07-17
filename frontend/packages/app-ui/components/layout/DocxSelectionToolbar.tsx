"use client";

import {
	CheckCheck,
	ChevronRight,
	CornerUpLeft,
	Minus,
	Plus,
	Sparkles,
	WandSparkles,
} from "lucide-react";
import { type ReactNode, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { DOCX_TONES, type DocxPolishAction, type DocxTone } from "./docx-selection-edit";
import type { OfficeRect } from "./office-selection";

type MenuState = "closed" | "open" | "closing";

export function DocxSelectionToolbar({
	anchor,
	portalContainer,
	busy,
	onPolish,
	onAddToConversation,
}: {
	anchor: OfficeRect;
	portalContainer?: HTMLElement;
	busy: boolean;
	onPolish: (action: DocxPolishAction) => void;
	onAddToConversation: () => void;
}) {
	const [menuState, setMenuState] = useState<MenuState>("closed");
	const [toneMenuState, setToneMenuState] = useState<MenuState>("closed");
	const closeTimerRef = useRef<number | null>(null);
	const hoverTimerRef = useRef<number | null>(null);
	const toneCloseTimerRef = useRef<number | null>(null);
	const toneHoverTimerRef = useRef<number | null>(null);
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
			for (const timer of [closeTimerRef, hoverTimerRef, toneCloseTimerRef, toneHoverTimerRef]) {
				if (timer.current !== null) window.clearTimeout(timer.current);
			}
		};
	}, []);

	const dropdownCloseMs = () =>
		Number.parseFloat(
			getComputedStyle(document.documentElement).getPropertyValue("--dropdown-close-dur"),
		) || 150;

	const openMenu = () => {
		if (hoverTimerRef.current !== null) window.clearTimeout(hoverTimerRef.current);
		if (closeTimerRef.current !== null) window.clearTimeout(closeTimerRef.current);
		hoverTimerRef.current = null;
		closeTimerRef.current = null;
		setMenuState("open");
	};

	const closeToneMenu = () => {
		if (toneMenuState !== "open") return;
		if (toneHoverTimerRef.current !== null) window.clearTimeout(toneHoverTimerRef.current);
		toneHoverTimerRef.current = null;
		setToneMenuState("closing");
		if (toneCloseTimerRef.current !== null) window.clearTimeout(toneCloseTimerRef.current);
		toneCloseTimerRef.current = window.setTimeout(() => {
			setToneMenuState("closed");
			toneCloseTimerRef.current = null;
		}, dropdownCloseMs());
	};

	const closeMenu = () => {
		if (menuState !== "open") return;
		setMenuState("closing");
		closeToneMenu();
		if (closeTimerRef.current !== null) window.clearTimeout(closeTimerRef.current);
		closeTimerRef.current = window.setTimeout(() => {
			setMenuState("closed");
			closeTimerRef.current = null;
		}, dropdownCloseMs());
	};

	const scheduleCloseMenu = () => {
		if (hoverTimerRef.current !== null) window.clearTimeout(hoverTimerRef.current);
		hoverTimerRef.current = window.setTimeout(closeMenu, 120);
	};

	const openToneMenu = () => {
		if (toneHoverTimerRef.current !== null) window.clearTimeout(toneHoverTimerRef.current);
		if (toneCloseTimerRef.current !== null) window.clearTimeout(toneCloseTimerRef.current);
		toneHoverTimerRef.current = null;
		toneCloseTimerRef.current = null;
		setToneMenuState("open");
	};

	const scheduleCloseToneMenu = () => {
		if (toneHoverTimerRef.current !== null) window.clearTimeout(toneHoverTimerRef.current);
		toneHoverTimerRef.current = window.setTimeout(closeToneMenu, 120);
	};

	const chooseAction = (action: DocxPolishAction) => {
		closeMenu();
		onPolish(action);
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
			<div className="flex items-stretch overflow-visible rounded-xl border border-slate-200 bg-white shadow-xl shadow-slate-900/10">
				<fieldset
					className="relative m-0 min-w-0 border-0 p-0"
					onMouseEnter={openMenu}
					onMouseLeave={scheduleCloseMenu}
				>
					<legend className="sr-only">AI 润色操作</legend>
					<button
						type="button"
						aria-expanded={menuState === "open"}
						disabled={busy}
						onFocus={openMenu}
						onClick={() => (menuState === "open" ? closeMenu() : openMenu())}
						className="inline-flex h-11 items-center gap-2 rounded-l-xl px-4 text-sm font-medium text-slate-800 transition-colors hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
					>
						<WandSparkles className="size-4" />
						AI 润色
					</button>
					<div
						data-origin="bottom-left"
						aria-hidden={menuState !== "open"}
						onMouseEnter={openMenu}
						onMouseLeave={scheduleCloseMenu}
						className={`t-dropdown absolute bottom-full left-0 mb-2 w-44 overflow-visible rounded-xl border border-slate-200 bg-white p-1.5 shadow-xl ${
							menuState === "open" ? "is-open" : menuState === "closing" ? "is-closing" : ""
						}`}
					>
						<ActionButton
							icon={<Sparkles className="size-4" />}
							label="优化表达"
							interactive={menuState === "open" && !busy}
							onClick={() => chooseAction("improve-expression")}
						/>
						<ActionButton
							icon={<Plus className="size-4" />}
							label="扩写"
							interactive={menuState === "open" && !busy}
							onClick={() => chooseAction("expand")}
						/>
						<ActionButton
							icon={<Minus className="size-4" />}
							label="缩写"
							interactive={menuState === "open" && !busy}
							onClick={() => chooseAction("shorten")}
						/>
						<fieldset
							className="relative m-0 min-w-0 border-0 p-0"
							onMouseEnter={openToneMenu}
							onMouseLeave={scheduleCloseToneMenu}
						>
							<legend className="sr-only">调整语气选项</legend>
							<ActionButton
								icon={<CornerUpLeft className="size-4" />}
								label="调整语气"
								trailing={<ChevronRight className="size-3.5" />}
								interactive={menuState === "open" && !busy}
								onClick={() => (toneMenuState === "open" ? closeToneMenu() : openToneMenu())}
							/>
							<div
								data-origin="top-left"
								aria-hidden={toneMenuState !== "open"}
								onMouseEnter={openToneMenu}
								className={`t-dropdown absolute left-full top-0 ml-1 w-32 rounded-xl border border-slate-200 bg-white p-1.5 shadow-xl ${
									toneMenuState === "open"
										? "is-open"
										: toneMenuState === "closing"
											? "is-closing"
											: ""
								}`}
							>
								{DOCX_TONES.map((tone) => (
									<ToneButton
										key={tone}
										tone={tone}
										interactive={toneMenuState === "open" && !busy}
										onClick={() => chooseAction({ kind: "tone", tone })}
									/>
								))}
							</div>
						</fieldset>
						<ActionButton
							icon={<CheckCheck className="size-4" />}
							label="校对"
							interactive={menuState === "open" && !busy}
							onClick={() => chooseAction("proofread")}
						/>
					</div>
				</fieldset>
				<div className="my-2 w-px bg-slate-200" />
				<button
					type="button"
					disabled={busy}
					onClick={onAddToConversation}
					className="inline-flex h-11 items-center gap-2 rounded-r-xl px-4 text-sm font-medium text-slate-800 transition-colors hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
				>
					<CornerUpLeft className="size-4" />
					添加到对话
				</button>
			</div>
		</div>,
		portalTarget,
	);
}

function ActionButton({
	icon,
	label,
	trailing,
	interactive,
	onClick,
}: {
	icon: ReactNode;
	label: string;
	trailing?: ReactNode;
	interactive: boolean;
	onClick: () => void;
}) {
	return (
		<button
			type="button"
			tabIndex={interactive ? 0 : -1}
			disabled={!interactive}
			onClick={onClick}
			className="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-left text-sm text-slate-700 transition-colors hover:bg-slate-100 hover:text-slate-950 disabled:pointer-events-none"
		>
			{icon}
			<span className="flex-1">{label}</span>
			{trailing}
		</button>
	);
}

function ToneButton({
	tone,
	interactive,
	onClick,
}: {
	tone: DocxTone;
	interactive: boolean;
	onClick: () => void;
}) {
	return (
		<button
			type="button"
			tabIndex={interactive ? 0 : -1}
			disabled={!interactive}
			onClick={onClick}
			className="block w-full rounded-lg px-3 py-2 text-left text-sm text-slate-700 transition-colors hover:bg-slate-100 hover:text-slate-950 disabled:pointer-events-none"
		>
			{tone}
		</button>
	);
}
