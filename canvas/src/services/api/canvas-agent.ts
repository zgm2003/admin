// 临时桩：本地 Canvas Agent（Codex MCP）通信层，admin 版规划中暂不接入，已按静态阶段回退。
// 类型定义保留供 Agent 面板组件编译；接入 admin 后端或决定启用 Agent 时再整体替换。
import i18n from "@/i18n";
import type { CanvasAgentSnapshot } from "@/lib/canvas/canvas-agent-ops";
import type { AgentReasoningEffort } from "@/stores/use-agent-store";

export type AgentConfigResponse = { ok?: boolean; protocolVersion?: number; url?: string; token?: string; hasToken?: boolean };

export class AgentApiError<T = unknown> extends Error {
    constructor(readonly status: number, readonly response: T & { code?: string; error?: string; msg?: string }) {
        super(response.error || response.msg || i18n.t("agent.state.requestFailed"));
        this.name = "AgentApiError";
    }
}

export type AgentSkillScope = "user" | "repo" | "system" | "admin";
export type AgentSkillInterface = { displayName?: string | null; shortDescription?: string | null; defaultPrompt?: string | null };
export type AgentSkillSummary = {
    name: string;
    description: string;
    shortDescription?: string | null;
    interface?: AgentSkillInterface | null;
    dependencies?: unknown;
    path: string;
    scope: AgentSkillScope;
    enabled: boolean;
    managed: boolean;
};
export type AgentSkillDetail = {
    name: string;
    description: string;
    instructions: string;
    interface?: AgentSkillInterface | null;
    path: string;
    managed: true;
    revision: string;
};
export type AgentSkillInput = { name?: string; description: string; instructions: string; interface?: AgentSkillInterface | null; expectedRevision?: string };
export type AgentSkillDraft = { name: string; displayName: string; description: string; instructions: string; shortDescription: string; defaultPrompt: string };
export type AgentSkillDraftInput = { source: "conversation" | "canvas"; threadId: string; clientId: string; model?: string; effort?: AgentReasoningEffort };
export type AgentSkillsResponse = { ok?: boolean; data?: AgentSkillSummary[]; errors?: unknown[] };
export type AgentSkillResponse = { ok?: boolean; data?: AgentSkillDetail };
export type AgentSkillDraftResponse = { ok?: boolean; data?: AgentSkillDraft };

export async function postState(_endpoint: string, _token: string, _clientId: string, _snapshot: CanvasAgentSnapshot | null) {
    return false;
}

export async function activateAgentClient(_endpoint: string, _token: string, _clientId: string) {}

export async function postToolResult(_endpoint: string, _token: string, _clientId: string, _body: { requestId: string; result?: unknown; error?: string }) {
    throw new AgentApiError(501, {});
}

export async function postCodexApproval(_endpoint: string, _token: string, _requestId: string, _decision: "accept" | "acceptForSession" | "decline") {
    throw new AgentApiError(501, {});
}

export async function interruptCodexTurn(_endpoint: string, _token: string, _threadId?: string) {
    throw new AgentApiError(501, {});
}

export async function acknowledgeCodexHistory(_endpoint: string, _token: string, _threadId: string, _turnIds: string[]) {
    throw new AgentApiError(501, {});
}

export async function revealAgentLocalFile(_endpoint: string, _token: string, _path: string) {
    throw new AgentApiError(501, {});
}

export function resolveAgentMessageAssetUrl(_endpoint: string, _token: string, value: string) {
    return value.startsWith("agent-asset:") ? "" : value;
}

export function fetchCodexSkills(_endpoint: string, _token: string, _forceReload = false): Promise<AgentSkillsResponse> {
    return Promise.reject(new AgentApiError(501, {}));
}

export function fetchCodexSkill(_endpoint: string, _token: string, _name: string): Promise<AgentSkillResponse> {
    return Promise.reject(new AgentApiError(501, {}));
}

export function createCodexSkill(_endpoint: string, _token: string, _input: AgentSkillInput): Promise<AgentSkillResponse> {
    return Promise.reject(new AgentApiError(501, {}));
}

export function createCodexSkillDraft(_endpoint: string, _token: string, _input: AgentSkillDraftInput): Promise<AgentSkillDraftResponse> {
    return Promise.reject(new AgentApiError(501, {}));
}

export function updateCodexSkill(_endpoint: string, _token: string, _name: string, _input: AgentSkillInput): Promise<AgentSkillResponse> {
    return Promise.reject(new AgentApiError(501, {}));
}

export function deleteCodexSkill(_endpoint: string, _token: string, _name: string, _expectedRevision: string): Promise<{ ok?: boolean }> {
    return Promise.reject(new AgentApiError(501, {}));
}

export function setCodexSkillEnabled(_endpoint: string, _token: string, _skill: Pick<AgentSkillSummary, "name" | "path">, _enabled: boolean): Promise<{ ok?: boolean }> {
    return Promise.reject(new AgentApiError(501, {}));
}

export async function fetchAgentJson<T>(_endpoint: string, _token: string, _path: string, _init?: RequestInit): Promise<T> {
    throw new AgentApiError(501, {});
}

export async function discoverAgentConfig(_endpoint: string): Promise<AgentConfigResponse | null> {
    return null;
}
