"use client";

import { cn } from "@leros/ui/lib/utils";
import { DiceBearAvatar } from "../avatar/DiceBearAvatar";
import { ProtectedImage } from "../avatar/ProtectedImage";

const sizeClassMap = {
	sm: "size-7 text-xs",
	default: "size-12 text-lg",
	md: "size-14 text-xl",
	lg: "size-16 text-2xl",
	xl: "size-20 text-3xl",
	"2xl": "size-24 text-4xl",
};

export function AssistantAvatar({
	name,
	src,
	size = "default",
	className,
}: {
	name: string;
	src?: string | null;
	size?: keyof typeof sizeClassMap;
	className?: string;
}) {
	const sizeClass = sizeClassMap[size];
	const pixelSize =
		size === "2xl"
			? 192
			: size === "xl"
				? 160
				: size === "lg"
					? 128
					: size === "md"
						? 112
						: size === "sm"
							? 56
							: 96;
	const fallback = (
		<DiceBearAvatar
			seed={`digital-assistant:${name}`}
			alt={name}
			className={sizeClass}
			size={pixelSize}
		/>
	);

	return (
		<div
			className={cn(
				"flex shrink-0 items-center justify-center overflow-hidden rounded-full font-semibold text-white",
				src ? "bg-transparent" : "bg-gradient-to-br from-blue-500 to-indigo-600",
				sizeClass,
				className,
			)}
		>
			{src ? (
				<ProtectedImage
					src={src}
					alt={name}
					className={cn("rounded-full object-cover", sizeClass)}
					fallback={fallback}
					loadingFallback={
						// 中文注释：受保护头像首次加载时展示中性占位，避免短暂闪现名称生成的默认头像。
						<span aria-hidden="true" className="size-full animate-pulse bg-slate-100" />
					}
				/>
			) : (
				fallback
			)}
		</div>
	);
}
