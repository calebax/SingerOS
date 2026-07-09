"use client";

import type { Message } from "@leros/store/types/chat";
import { cn } from "@leros/ui/lib/utils";
import type { ReactNode } from "react";
import { SkillDirectiveBadge } from "../common/SkillDirectiveBadge";

export function MessageContentWithComposerTokens({
	message,
	className,
}: {
	message: Pick<Message, "content" | "metadata">;
	className?: string;
}) {
	// 中文注释：部分入口会把 @队友 从实际发送内容中剥离，这里优先使用展示专用文本恢复标签样式。
	const displayContent = message.metadata?.displayContent ?? message.content;
	const tokens = (message.metadata?.displayComposerTokens ?? message.metadata?.composerTokens ?? [])
		.filter((token) => displayContent.slice(token.start, token.end) === token.label)
		.sort((a, b) => a.start - b.start);

	if (tokens.length === 0) {
		// 中文注释：没有明确 token metadata 时，普通内容里的 @ 和 / 必须原样展示，不能靠文本猜样式。
		return (
			<span className={cn("whitespace-pre-wrap break-words", className)}>{displayContent}</span>
		);
	}

	const parts: ReactNode[] = [];
	let cursor = 0;
	tokens.forEach((token, index) => {
		if (token.start > cursor) {
			parts.push(
				<span key={`text-${index}`} className="whitespace-pre-wrap break-words">
					{displayContent.slice(cursor, token.start)}
				</span>,
			);
		}
		parts.push(
			token.kind === "skill" ? (
				<SkillDirectiveBadge
					key={`token-${index}`}
					name={token.label.replace(/^\/+/, "")}
					variant="on-blue"
				/>
			) : (
				<span
					key={`token-${index}`}
					className="inline-flex max-w-full items-center rounded-md bg-blue-100 px-1.5 py-0.5 text-xs font-medium leading-none text-blue-700"
				>
					{token.label}
				</span>
			),
		);
		cursor = token.end;
	});

	if (cursor < displayContent.length) {
		parts.push(
			<span key="text-tail" className="whitespace-pre-wrap break-words">
				{displayContent.slice(cursor)}
			</span>,
		);
	}

	return (
		<span className={cn("inline-flex flex-wrap items-center gap-1.5", className)}>{parts}</span>
	);
}
