"use client";

import type { PluginListItem } from "@leros/store";
import { mergeSkillOptions, pluginApi, pluginToComposerOption } from "@leros/store";
import { useEffect, useState } from "react";
import type { ComposerSkillOption } from "./StructuredComposer";

/**
 * Loads and merges skill options from all three sources:
 * project-bound skills (when projectId is provided), org skills, and builtin skills.
 * Each source degrades independently on failure.
 * Order: project-bound org skills → remaining org skills → system builtin skills.
 */
export function useComposerSkillOptions(projectId: string | null | undefined): {
	skillOptions: ComposerSkillOption[] | undefined;
	skillsLoading: boolean;
} {
	const [skillOptions, setSkillOptions] = useState<ComposerSkillOption[] | undefined>(undefined);
	const [skillsLoading, setSkillsLoading] = useState(true);

	useEffect(() => {
		let cancelled = false;
		setSkillsLoading(true);

		const loadAll = async () => {
			const projectPromise: Promise<PluginListItem[]> = projectId
				? pluginApi
						.listProject({ public_id: projectId, kind: "skill" })
						.then((r) => (r.data.code === 0 ? r.data.data : []))
						.catch(() => [])
				: Promise.resolve([]);

			const orgPromise: Promise<PluginListItem[]> = pluginApi
				.list({ kind: "skill", status: "active" })
				.then((r) => (r.data.code === 0 ? (r.data.data.plugins ?? []) : []))
				.catch(() => []);

			const builtinPromise: Promise<PluginListItem[]> = pluginApi
				.listBuiltinSkills()
				.then((r) => (r.data.code === 0 ? (r.data.data.plugins ?? []) : []))
				.catch(() => []);

			const [projectItems, orgItems, builtinItems] = await Promise.all([
				projectPromise,
				orgPromise,
				builtinPromise,
			]);

			if (cancelled) return;

			setSkillOptions(
				mergeSkillOptions(
					projectItems.map((i) => pluginToComposerOption(i)),
					orgItems.map((i) => pluginToComposerOption(i)),
					builtinItems.map((i) => pluginToComposerOption(i)),
				),
			);
			setSkillsLoading(false);
		};

		void loadAll();

		return () => {
			cancelled = true;
		};
	}, [projectId]);

	return { skillOptions, skillsLoading };
}
