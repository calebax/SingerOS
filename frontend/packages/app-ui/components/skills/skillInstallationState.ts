import type { PluginInstallationStatus } from "@leros/store";

export type MarketplaceSkillAction =
	| "checking"
	| "unavailable"
	| "install"
	| "update"
	| "installed";

export function resolveMarketplaceSkillAction(
	status: PluginInstallationStatus | null,
	loading: boolean,
	failed: boolean,
): MarketplaceSkillAction {
	if (loading) return "checking";
	if (failed || !status) return "unavailable";
	if (status.update_available) return "update";
	if (status.installed) return "installed";
	return "install";
}

export function canUpdateOrganizationSkill(status: PluginInstallationStatus | null): boolean {
	return Boolean(
		status?.installed &&
			status.marketplace_based &&
			status.update_available &&
			status.marketplace_item_id,
	);
}
