// 临时桩：模型调用脚本运行时原为在浏览器内执行用户自定义 JS 脚本直连接口的实现，已按静态阶段回退。
// 接入 admin 后端时在保持导出签名不变的前提下整体替换本文件。
import i18n from "@/i18n";
import type { AiConfig, ModelCapability } from "@/stores/use-config-store";

function notConnected(): Error {
    return new Error(i18n.t("common.notConnected"));
}

export type PluginHttpOptions = {
    headers?: Record<string, string>;
    params?: Record<string, unknown>;
    responseType?: "json" | "blob" | "text" | "arraybuffer";
};

export type PluginHttp = {
    url: (path: string) => string;
    post: (path: string, body?: unknown, options?: PluginHttpOptions) => Promise<unknown>;
    get: (path: string, options?: PluginHttpOptions) => Promise<unknown>;
};

export type PluginPollOptions = { intervalMs?: number; timeoutMs?: number };

export type RunPluginArgs = {
    capability: ModelCapability;
    script: string;
    config: AiConfig;
    prompt?: string;
    images?: string[];
    videos?: File[];
    audios?: File[];
    messages?: unknown[];
    params?: Record<string, unknown>;
    signal?: AbortSignal;
    onDelta?: (text: string) => void;
};

export async function runModelPlugin<T = unknown>(_args: RunPluginArgs): Promise<T> {
    throw notConnected();
}

export type PluginVariable = { name: string; type: string; desc: string; capabilities?: ModelCapability[] };

export function getPluginVariables(): PluginVariable[] {
    return [];
}

export function getPluginReturn(capability: ModelCapability) {
    return i18n.t(`modelPlugin.returns.${capability}`);
}

export function getPluginAuthoringPrompt(_capability: ModelCapability, _modelName: string, _draft = "") {
    return "";
}

export type PluginTemplate = { label: string; script: string };

export function getPluginTemplates(): Record<ModelCapability, PluginTemplate[]> {
    return { image: [], video: [], audio: [], text: [] };
}

export function normalizePluginImages(result: unknown): string[] {
    const items = Array.isArray(result) ? result : [result];
    const urls = items
        .map((item) => {
            if (typeof item === "string") return item;
            if (item && typeof item === "object") {
                const record = item as Record<string, unknown>;
                if (typeof record.dataUrl === "string") return record.dataUrl;
                if (typeof record.url === "string") return record.url;
                if (typeof record.b64_json === "string") return `data:image/png;base64,${record.b64_json}`;
            }
            return "";
        })
        .filter(Boolean);
    if (!urls.length) throw new Error(i18n.t("modelPlugin.noImages"));
    return urls;
}
