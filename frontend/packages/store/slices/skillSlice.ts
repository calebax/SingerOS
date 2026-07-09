import { type SkillInstalledItem, skillMarketplaceApi } from "../api/skillMarketplaceApi";
import type { SliceCreator } from "../types";
import { flattenActions } from "../utils";
import { readStoredAuthUser } from "../utils/authStorage";

export type SkillState = {
	installedSkills: SkillInstalledItem[];
	installedSkillsLoaded: boolean;
};

export type SkillAction = Pick<SkillSliceImpl, keyof SkillSliceImpl>;
export type SkillStore = SkillState & SkillAction;

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null;
}

function stringFromValue(value: unknown): string {
	return typeof value === "string" ? value : "";
}

function skillItemFromValue(value: unknown): SkillInstalledItem | null {
	if (!isRecord(value)) return null;

	const name = stringFromValue(value.name || value.skill_id || value.id);
	if (!name) return null;

	return {
		name,
		display_name: stringFromValue(value.display_name),
		description: stringFromValue(value.description),
		category: stringFromValue(value.category),
		source: stringFromValue(value.source || value.source_type),
		trust: stringFromValue(value.trust),
	};
}

function skillItemsFromValue(value: unknown): SkillInstalledItem[] {
	if (!Array.isArray(value)) return [];
	return value.map(skillItemFromValue).filter((item): item is SkillInstalledItem => item !== null);
}

export function normalizeInstalledSkillsPayload(value: unknown): SkillInstalledItem[] {
	if (Array.isArray(value)) return skillItemsFromValue(value);
	if (!isRecord(value)) return [];

	const nestedData = value.data;
	if (isRecord(nestedData)) {
		if (Array.isArray(nestedData.skills)) {
			return skillItemsFromValue(nestedData.skills);
		}
		if (Array.isArray(nestedData.items)) {
			return skillItemsFromValue(nestedData.items);
		}
	}

	if (Array.isArray(value.skills)) return skillItemsFromValue(value.skills);
	if (Array.isArray(value.items)) return skillItemsFromValue(value.items);
	return [];
}

const _initialState: SkillState = {
	installedSkills: [],
	installedSkillsLoaded: false,
};

type SetState = (
	partial:
		| SkillStore
		| Partial<SkillStore>
		| ((state: SkillStore) => SkillStore | Partial<SkillStore>),
	replace?: boolean,
) => void;

export const createSkillSlice = (set: SetState) => new SkillSliceImpl(set);

export class SkillSliceImpl {
	readonly #set: SetState;
	#fetchInstalledSkillsPromise: Promise<void> | null = null;
	#installedSkillsFetchEpoch = 0;

	constructor(set: SetState) {
		this.#set = set;
	}

	fetchInstalledSkills = async () => {
		if (!readStoredAuthUser()?.jwtToken) return;
		if (this.#fetchInstalledSkillsPromise) return this.#fetchInstalledSkillsPromise;

		const fetchEpoch = this.#installedSkillsFetchEpoch;
		this.#fetchInstalledSkillsPromise = (async () => {
			try {
				const res = await skillMarketplaceApi.installed();
				if (fetchEpoch !== this.#installedSkillsFetchEpoch) return;
				const skills = normalizeInstalledSkillsPayload(res.data);
				this.#set({
					installedSkills: skills,
					installedSkillsLoaded: true,
				});
			} catch (err) {
				console.error("fetchInstalledSkills error:", err);
			} finally {
				if (fetchEpoch === this.#installedSkillsFetchEpoch) {
					this.#fetchInstalledSkillsPromise = null;
				}
			}
		})();

		return this.#fetchInstalledSkillsPromise;
	};

	removeInstalledSkill = (name: string) => {
		this.#set((state) => ({
			installedSkills: state.installedSkills.filter((skill) => skill.name !== name),
		}));
	};

	resetAuthScopedData = () => {
		this.#installedSkillsFetchEpoch += 1;
		this.#fetchInstalledSkillsPromise = null;
		this.#set({
			installedSkills: [],
			installedSkillsLoaded: false,
		});
	};
}

export const skillSlice: SliceCreator<SkillStore> = (...params) => ({
	..._initialState,
	...flattenActions<SkillAction>([createSkillSlice(params[0] as SetState)]),
});
