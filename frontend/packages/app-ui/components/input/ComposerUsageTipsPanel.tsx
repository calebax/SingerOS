"use client";

import { cn } from "@leros/ui/lib/utils";
import { ChevronRight, Lightbulb, Sparkles } from "lucide-react";
import { useEffect, useState } from "react";
import { BID_COMPARISON_ICON_SRC } from "../../assets";

type ComposerUsageTipsPanelProps = {
	tips: Array<{ id: string; label: string; prompt: string }>;
	onApply: (prompt: string) => void;
	className?: string;
	onBidComparisonClick: () => void;
};

/** Windows ClearType 旋转文字易糊；仅在非 Windows 保留旋转动效。 */
function canUseCrispTextRotate(): boolean {
	if (typeof navigator === "undefined") return false;
	return !/win/i.test(navigator.platform) && !/windows/i.test(navigator.userAgent);
}

/** 标书对比入口按钮；任务详情等场景单独使用，不挂「使用提示」。 */
export function BidComparisonEntryButton({
	onClick,
	disabled = false,
	className,
}: {
	onClick: () => void;
	disabled?: boolean;
	className?: string;
}) {
	const [enableRotate, setEnableRotate] = useState(false);

	useEffect(() => {
		setEnableRotate(canUseCrispTextRotate());
	}, []);

	return (
		<button
			type="button"
			onClick={onClick}
			disabled={disabled}
			aria-disabled={disabled}
			className={cn(
				"inline-flex max-w-full items-center gap-2 rounded-full border border-violet-400 px-3.5 py-2 text-left text-sm font-semibold text-violet-600 transition hover:bg-violet-50",
				enableRotate && !disabled && "hover:rotate-5",
				disabled && "cursor-not-allowed opacity-50 hover:bg-transparent",
				className,
			)}
		>
			<img src={BID_COMPARISON_ICON_SRC} alt="" className="size-3.5" />
			<span>标书对比</span>
		</button>
	);
}

export function ComposerUsageTipsPanel({
	tips,
	onApply,
	className,
	onBidComparisonClick,
}: ComposerUsageTipsPanelProps) {
	const [enableRotate, setEnableRotate] = useState(false);

	useEffect(() => {
		setEnableRotate(canUseCrispTextRotate());
	}, []);

	return (
		<div className={cn("mb-4", className)}>
			<div className="mb-3 flex items-center gap-2 text-sm font-medium text-[var(--leros-text-muted)]">
				<Lightbulb className="size-4 shrink-0" />
				<span>使用提示</span>
			</div>
			<div className="flex min-w-0 flex-wrap items-center gap-2.5">
				<BidComparisonEntryButton onClick={onBidComparisonClick} />
				<span aria-hidden className="mx-0.5 h-5 border-l border-slate-200" />
				{tips.map((tip) => (
					<button
						key={tip.id}
						type="button"
						onClick={() => onApply(tip.prompt)}
						className={cn(
							"inline-flex max-w-full items-center gap-2 rounded-full bg-white px-3.5 py-2 text-left shadow-sm ring-1 ring-slate-200/80 transition hover:bg-slate-50",
							enableRotate && "hover:rotate-5",
						)}
					>
						<Sparkles className="size-3.5 shrink-0 text-slate-400" />
						<span className="truncate text-sm text-[var(--leros-text)]">{tip.label}</span>
						<ChevronRight className="size-3.5 shrink-0 text-slate-300" />
					</button>
				))}
			</div>
		</div>
	);
}
