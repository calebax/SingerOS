import type { PluginInstallationStatus } from "@leros/store";
import { describe, expect, it } from "vitest";
import {
	canUpdateOrganizationSkill,
	resolveMarketplaceSkillAction,
} from "./skillInstallationState";

function status(overrides: Partial<PluginInstallationStatus> = {}): PluginInstallationStatus {
	return {
		kind: "skill",
		code: "demo",
		installed: false,
		marketplace_based: false,
		marketplace_available: true,
		update_available: false,
		...overrides,
	};
}

describe("skill installation state", () => {
	it("maps market status to install, installed, and update actions", () => {
		expect(resolveMarketplaceSkillAction(null, true, false)).toBe("checking");
		expect(resolveMarketplaceSkillAction(null, false, true)).toBe("unavailable");
		expect(resolveMarketplaceSkillAction(status(), false, false)).toBe("install");
		expect(resolveMarketplaceSkillAction(status({ installed: true }), false, false)).toBe(
			"installed",
		);
		expect(
			resolveMarketplaceSkillAction(
				status({ installed: true, marketplace_based: true, update_available: true }),
				false,
				false,
			),
		).toBe("update");
	});

	it("shows organization update only for a valid marketplace update target", () => {
		expect(
			canUpdateOrganizationSkill(
				status({
					installed: true,
					marketplace_based: true,
					update_available: true,
					marketplace_item_id: "mkt_demo",
				}),
			),
		).toBe(true);
		expect(
			canUpdateOrganizationSkill(
				status({
					installed: true,
					marketplace_based: false,
					update_available: true,
					marketplace_item_id: "mkt_demo",
				}),
			),
		).toBe(false);
	});
});
