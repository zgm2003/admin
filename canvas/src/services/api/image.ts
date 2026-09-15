// 临时桩：AI 生成调用层原为浏览器直连 OpenAI/Gemini 的实现，已按静态阶段回退。
// 接入 admin 后端时在保持导出签名不变的前提下整体替换本文件。
import i18n from "@/i18n";
import type { AiConfig, ModelChannel } from "@/stores/use-config-store";
import type { ReferenceImage } from "@/types/image";

export type AiTextMessage = {
    role: "system" | "user" | "assistant";
    content: string | Array<{ type: "text"; text: string } | { type: "image_url"; image_url: { url: string } }>;
};

type RequestOptions = { signal?: AbortSignal };

function notConnected(): Error {
    return new Error(i18n.t("common.notConnected"));
}

export async function requestGeneration(_config: AiConfig, _prompt: string, _options?: RequestOptions): Promise<Array<{ id: string; dataUrl: string }>> {
    throw notConnected();
}

export async function requestEdit(_config: AiConfig, _prompt: string, _references: ReferenceImage[], _options?: RequestOptions): Promise<Array<{ id: string; dataUrl: string }>> {
    throw notConnected();
}

export async function requestImageQuestion(_config: AiConfig, _messages: AiTextMessage[], _onDelta: (text: string) => void, _options?: RequestOptions): Promise<string> {
    throw notConnected();
}

export async function fetchImageModels(_config: Pick<AiConfig, "baseUrl" | "apiKey" | "apiFormat">): Promise<string[]> {
    throw notConnected();
}

export async function fetchChannelModels(_channel: ModelChannel): Promise<string[]> {
    throw notConnected();
}
