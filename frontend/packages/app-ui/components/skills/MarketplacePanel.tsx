"use client";

import {
	type OfficialPluginMarketplaceItem,
	officialPluginMarketplaceApi,
	type SkillMarketplaceItem,
} from "@leros/store";
import { Search } from "lucide-react";
import { useEffect, useState } from "react";
import { SkillCard } from "./SkillCard";

const PAGE_SIZE = 90;

function officialToSkillCard(item: OfficialPluginMarketplaceItem): SkillMarketplaceItem {
	return {
		source_type: "official",
		skill_id: item.public_id,
		name: item.code,
		display_name: item.name,
		description: item.description ?? "",
		version: item.version,
		author: item.author,
		category: item.category,
		tags: item.tags,
		icon: item.icon ?? "",
		installs: 0,
		verified: item.verified,
	};
}

interface MarketplacePanelProps {
	/** Called when a skill card is clicked (for navigation to detail page) */
	onCardClick?: (skill: SkillMarketplaceItem) => void;
	isAuthenticated?: boolean;
	/** Changes after an official plugin installation so the catalogue is reloaded. */
	refreshSeq?: number;
}

export function MarketplacePanel({
	onCardClick,
	isAuthenticated = true,
	refreshSeq = 0,
}: MarketplacePanelProps) {
	const [items, setItems] = useState<SkillMarketplaceItem[]>([]);
	const [loading, setLoading] = useState(true);
	const [keyword, setKeyword] = useState("");
	const [debouncedKeyword, setDebouncedKeyword] = useState("");
	const [mounted, setMounted] = useState(false);

	// debounce keyword
	useEffect(() => {
		setMounted(true);
	}, []);

	useEffect(() => {
		const timer = setTimeout(() => setDebouncedKeyword(keyword), 300);
		return () => clearTimeout(timer);
	}, [keyword]);

	const searchKeyword = isAuthenticated ? debouncedKeyword : "";
	// Fetch the official plugin catalogue on keyword change.
	useEffect(() => {
		if (!mounted) return;
		if (!isAuthenticated) {
			setItems([]);
			setLoading(false);
			return;
		}
		let cancelled = false;
		const fetchItems = async () => {
			setLoading(true);
			try {
				const resp = await officialPluginMarketplaceApi.list({
					kind: "skill",
					keyword: searchKeyword || undefined,
					limit: PAGE_SIZE,
				});
				if (cancelled) return;
				const newItems = (resp.data.data.items ?? []).map(officialToSkillCard);
				setItems(newItems);
			} catch (err) {
				if (!cancelled) console.error("Failed to fetch skills:", err);
			} finally {
				if (!cancelled) setLoading(false);
			}
		};
		fetchItems();
		return () => {
			cancelled = true;
		};
	}, [mounted, isAuthenticated, searchKeyword, refreshSeq]);

	return (
		<>
			{/* Search + Filters */}
			<div className="flex items-center border-b border-[var(--leros-control-border)] px-6 py-3">
				<div className="relative flex-1 max-w-xs">
					<Search className="absolute left-2.5 top-1/2 -translate-y-1/2 size-3.5 text-[var(--leros-text-subtle)]" />
					<input
						type="text"
						placeholder="搜索技能..."
						value={keyword}
						onChange={(e) => setKeyword(e.target.value)}
						className="w-full rounded-md border border-[var(--leros-control-border)] bg-[var(--leros-surface-soft)] py-1.5 pl-7 pr-2 text-xs text-[var(--leros-text)] placeholder:text-[var(--leros-text-subtle)] focus:border-[var(--leros-primary)] focus:bg-white focus:outline-none transition-colors"
					/>
				</div>
			</div>

			{/* Marketplace grid */}
			<div className="min-h-0 flex-1 overflow-y-auto px-6 py-8">
				<div>
					{!mounted || (isAuthenticated && loading) ? (
						<div className="flex items-center justify-center py-16 text-sm text-[var(--leros-text-subtle)]">
							加载中...
						</div>
					) : items.length === 0 ? (
						<div className="flex flex-col items-center justify-center py-16 text-[var(--leros-text-subtle)]">
							<p className="text-sm">暂无符合条件的技能</p>
						</div>
					) : (
						<div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
							{items.map((skill) => (
								<SkillCard key={skill.skill_id} skill={skill} onClick={onCardClick} />
							))}
						</div>
					)}
				</div>
			</div>
		</>
	);
}
