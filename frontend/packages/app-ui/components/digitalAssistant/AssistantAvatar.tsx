"use client";

import { cn } from "@leros/ui/lib/utils";
import { CUSTOM_ASSISTANT_DEFAULT_AVATAR_SRC } from "../../assets";
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
	// 中文注释：未上传头像时统一展示固定默认图，不再按名称生成随机头像。
	const fallback = (
		<img
			src={CUSTOM_ASSISTANT_DEFAULT_AVATAR_SRC}
			alt={name}
			className={cn("rounded-full object-cover", sizeClass)}
		/>
	);

	return (
		<div
			className={cn(
				"flex shrink-0 items-center justify-center overflow-hidden rounded-full bg-transparent",
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
						// 中文注释：受保护头像首次加载时展示中性占位，避免短暂闪现默认头像。
						<span aria-hidden="true" className="size-full animate-pulse bg-slate-100" />
					}
				/>
			) : (
				fallback
			)}
		</div>
	);
}
