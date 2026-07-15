import { afterEach, describe, expect, it, vi } from "vitest";

import { authApi } from "../api/authApi";
import { AuthActionImpl, type AuthStore } from "./authSlice";

function createAuthActions() {
	let state: Partial<AuthStore> = {
		authUser: {
			publicId: "user-1",
			name: "测试用户",
			email: "test@example.com",
			jwtToken: "old-token",
			refreshToken: "old-refresh-token",
			expiredAt: 1,
			uin: 1,
			currentOrg: { id: 1, publicId: "org-1", code: "org-1", name: "旧组织" },
			organizations: [],
		},
	};
	const setState = (partial: unknown) => {
		const update =
			typeof partial === "function"
				? (partial as (current: Partial<AuthStore>) => Partial<AuthStore>)(state)
				: (partial as Partial<AuthStore>);
		state = { ...state, ...update };
	};
	return { actions: new AuthActionImpl(setState as never), getState: () => state };
}

describe("AuthActionImpl", () => {
	afterEach(() => {
		vi.restoreAllMocks();
		localStorage.clear();
	});

	it("组织切换后忽略仍在途的旧 AuthSession 响应", async () => {
		let resolveSession: ((value: unknown) => void) | undefined;
		vi.spyOn(authApi, "authSession").mockReturnValue(
			new Promise((resolve) => {
				resolveSession = resolve;
			}) as never,
		);
		vi.spyOn(authApi, "switchOrganization").mockResolvedValue({
			data: {
				code: 0,
				message: "success",
				data: {
					login_status: "success",
					jwt_token: "new-token",
					refresh_token: "new-refresh-token",
					expired_at: 2,
					uin: 1,
					user_info: {
						id: 1,
						public_id: "user-1",
						name: "测试用户",
						email: "test@example.com",
					},
					org: { id: 2, public_id: "org-2", code: "org-2", name: "AI冲锋队" },
					organizations: [],
				},
			},
		} as never);
		const { actions, getState } = createAuthActions();

		const sessionRefresh = actions.refreshAuthSession();
		await actions.switchOrganization(2);
		resolveSession?.({
			data: {
				code: 0,
				message: "success",
				data: {
					user_info: {
						id: 1,
						public_id: "user-1",
						name: "测试用户",
						email: "test@example.com",
					},
					org: { id: 1, public_id: "org-1", code: "org-1", name: "旧组织" },
					organizations: [],
				},
			},
		});
		await sessionRefresh;

		expect(getState().authUser?.currentOrg?.id).toBe(2);
	});
});
