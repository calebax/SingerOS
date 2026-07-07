import type { DigitalAssistantItem, ProjectMember } from "@leros/store";
import type { ComposerToken, Message } from "@leros/store/types/chat";

export type AssistantMessageDisplay = {
	useDefaultBrand: boolean;
	name: string;
	avatarUrl?: string;
};

const DEFAULT_ASSISTANT_NAME = "Lework";

function getReplyTargetMessageId(runId?: string): string | undefined {
	const match = runId?.match(/^req_(.+)$/);
	return match?.[1]?.trim() || undefined;
}

function findAssistantComposerToken(message?: Message): ComposerToken | undefined {
	return (message?.metadata?.composerTokens ?? []).find(
		(token) => token.kind === "assistant" && Boolean(token.id?.trim() || token.label.trim()),
	);
}

function normalizeAssistantNameFromToken(token: ComposerToken): string {
	return token.label.replace(/^@+/, "").trim();
}

function resolveAssistantProfile(
	token: ComposerToken,
	assistants: DigitalAssistantItem[],
	projectMembers?: ProjectMember[],
): { name: string; avatarUrl?: string } {
	const publicId = token.id?.trim();
	if (publicId) {
		const matchedAssistant = assistants.find(
			(assistant) =>
				assistant.publicId === publicId ||
				String(assistant.id) === publicId ||
				assistant.code === publicId,
		);
		if (matchedAssistant) {
			return {
				name: matchedAssistant.name,
				avatarUrl: matchedAssistant.avatar || undefined,
			};
		}

		const matchedMember = projectMembers?.find(
			(member) =>
				member.type === "assistant" &&
				!member.isDefault &&
				(member.publicId === publicId || String(member.memberId) === publicId),
		);
		if (matchedMember) {
			return {
				name: matchedMember.name,
				avatarUrl: matchedMember.avatarUrl,
			};
		}
	}

	const tokenName = normalizeAssistantNameFromToken(token);
	const matchedByName = assistants.find((assistant) => assistant.name === tokenName);
	if (matchedByName) {
		return { name: matchedByName.name, avatarUrl: matchedByName.avatar || undefined };
	}

	const matchedMemberByName = projectMembers?.find(
		(member) => member.type === "assistant" && !member.isDefault && member.name === tokenName,
	);
	if (matchedMemberByName) {
		return {
			name: matchedMemberByName.name,
			avatarUrl: matchedMemberByName.avatarUrl,
		};
	}

	return { name: tokenName || DEFAULT_ASSISTANT_NAME };
}

function resolveTriggeringUserMessage(
	message: Message,
	messagesMap: Record<string, Message>,
): Message | undefined {
	const replyTargetId = message.replyTo?.messageId ?? getReplyTargetMessageId(message.runId);
	if (!replyTargetId) return undefined;

	const target = messagesMap[replyTargetId];
	return target?.role === "user" ? target : undefined;
}

/** 根据触发本轮回复的用户消息，解析 assistant 气泡应展示的队友名称与头像。 */
export function resolveAssistantMessageDisplay(params: {
	message: Message;
	messagesMap: Record<string, Message>;
	assistants: DigitalAssistantItem[];
	projectMembers?: ProjectMember[];
}): AssistantMessageDisplay {
	const { message, messagesMap, assistants, projectMembers } = params;
	const triggeringUserMessage = resolveTriggeringUserMessage(message, messagesMap);
	const assistantToken =
		findAssistantComposerToken(triggeringUserMessage) ?? findAssistantComposerToken(message);

	// 中文注释：只有用户显式 @ 指定 AI 队友时才切换头像/名称，否则保持默认 Lework 品牌样式。
	if (!assistantToken) {
		return { useDefaultBrand: true, name: DEFAULT_ASSISTANT_NAME };
	}

	const profile = resolveAssistantProfile(assistantToken, assistants, projectMembers);
	return {
		useDefaultBrand: false,
		name: profile.name,
		avatarUrl: profile.avatarUrl,
	};
}
