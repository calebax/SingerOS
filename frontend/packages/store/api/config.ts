type PublicEnv = {
	readonly NEXT_PUBLIC_LEROS_API_BASE_URL?: string;
	readonly VITE_LEROS_API_BASE_URL?: string;
	readonly LEROS_API_BASE_URL?: string;
};

declare const process:
	| {
			readonly env?: PublicEnv;
	  }
	| undefined;

const DEFAULT_API_BASE_URL = "http://localhost:8080/v1";

function getProcessEnv(): PublicEnv | undefined {
	if (typeof process === "undefined") return undefined;
	return process.env;
}

function resolveAPIBaseURL(): string {
	const viteEnv = (import.meta as ImportMeta & { readonly env?: PublicEnv }).env;
	const processEnv = getProcessEnv();
	const baseURL =
		viteEnv?.VITE_LEROS_API_BASE_URL ||
		processEnv?.NEXT_PUBLIC_LEROS_API_BASE_URL ||
		processEnv?.LEROS_API_BASE_URL ||
		DEFAULT_API_BASE_URL;

	return baseURL.trim().replace(/\/+$/, "");
}

export const API_BASE_URL = resolveAPIBaseURL();
