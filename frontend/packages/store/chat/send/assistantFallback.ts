/**
 * 问答路径：等待 GlobalEvents assistant 的超时兜底。
 *
 * 可以做：在完整等待窗口内轮询；GE 已接管或 run 已结束时提前退出；超时后把占位标失败并写入正文报错。
 * 不可以做：因 runtime_status=responding 提前失败（responding 只表示后端在跑，不等于 GE assistant 已到）；
 * 不可以在此处打开 SessionEvents。
 */
import { sessionApi } from "../../api/sessionApi";
import {
	ASSISTANT_GLOBAL_EVENTS_TIMEOUT_TEXT,
	TASK_ROOM_ASSISTANT_START_FALLBACK_MS,
} from "../messageMerge";
import type { SendPipelineDeps } from "./deps";

const POLL_INTERVAL_MS = 2_000;

/** 超时收尾：写入报错、抑制本轮后续 GE/resume，并结束 generating。 */
function failWaitingAssistant(
	deps: SendPipelineDeps,
	sessionId: string,
	assistantMsgId: string,
): void {
	const current = deps.get().messagesMap[assistantMsgId];
	if (current) {
		deps.updateMessage(assistantMsgId, {
			...current,
			status: "failed",
			statusText: undefined,
			// 中文注释：与 run.failed 一致写入正文，确保回复气泡直接展示报错。
			content: ASSISTANT_GLOBAL_EVENTS_TIMEOUT_TEXT,
		});
	}
	deps.set({
		pendingBootstrapSessionId: null,
		// 中文注释：抑制迟到 GE assistant，以及超时后 isGenerating→false 触发的 resume 开流。
		suppressedReplySessionId: sessionId,
	});
	deps.finishStream();
}

/**
 * 等不到 GlobalEvents assistant 时的终态收尾。
 * 任务群聊续聊与新建任务 bootstrap 共用，避免两套超时语义分叉。
 */
export async function waitForGlobalAssistantOrFail(
	deps: SendPipelineDeps,
	sessionId: string,
	assistantMsgId: string,
): Promise<void> {
	const deadline = Date.now() + TASK_ROOM_ASSISTANT_START_FALLBACK_MS;
	let baselineMessageCount = 0;
	try {
		const sessionRes = await sessionApi.get({ session_id: sessionId });
		baselineMessageCount = sessionRes.data.data?.message_count ?? 0;
	} catch (err) {
		console.error("waitForGlobalAssistantOrFail baseline error:", err);
	}

	try {
		while (Date.now() < deadline) {
			await new Promise((resolve) => setTimeout(resolve, POLL_INTERVAL_MS));
			const state = deps.get();
			// 中文注释：GlobalEvents assistant 到达后会替换 waiting 占位并改 streamingMessageId，兜底立刻退出。
			if (
				state.activeSessionId !== sessionId ||
				state.streamingMessageId !== assistantMsgId ||
				!state.isGenerating
			) {
				return;
			}

			try {
				const res = await sessionApi.get({ session_id: sessionId });
				const status = res.data.data?.runtime_status;
				const messageCount = res.data.data?.message_count;
				// 中文注释：run 已结束且消息数增加，直接回拉历史；仍不开放 SessionEvents。
				if (
					messageCount !== undefined &&
					messageCount > baselineMessageCount &&
					status === "idle"
				) {
					deps.set({ pendingBootstrapSessionId: null });
					await deps.loadConversationMessages(sessionId, { resumeStream: false });
					deps.finishStream();
					return;
				}
			} catch {
				// 轮询失败继续等待，直到总超时。
			}
		}

		const state = deps.get();
		if (
			state.activeSessionId !== sessionId ||
			state.streamingMessageId !== assistantMsgId ||
			!state.isGenerating
		) {
			return;
		}
		failWaitingAssistant(deps, sessionId, assistantMsgId);
	} catch (err) {
		console.error("waitForGlobalAssistantOrFail error:", err);
		const state = deps.get();
		if (
			state.activeSessionId !== sessionId ||
			state.streamingMessageId !== assistantMsgId ||
			!state.isGenerating
		) {
			return;
		}
		failWaitingAssistant(deps, sessionId, assistantMsgId);
	}
}
