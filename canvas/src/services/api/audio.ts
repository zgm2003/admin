// 临时桩：音频生成调用层原为浏览器直连 OpenAI/Gemini 的实现，已按静态阶段回退。
// 接入 admin 后端时在保持导出签名不变的前提下整体替换本文件。
import i18n from "@/i18n";
import { audioMimeType } from "@/lib/audio-generation";
import { uploadMediaFile, type UploadedFile } from "@/services/file-storage";
import type { AiConfig } from "@/stores/use-config-store";

type RequestOptions = { signal?: AbortSignal };

export async function requestAudioGeneration(_config: AiConfig, _prompt: string, _options?: RequestOptions): Promise<Blob> {
    throw new Error(i18n.t("common.notConnected"));
}

export async function storeGeneratedAudio(blob: Blob, format = "mp3"): Promise<UploadedFile> {
    const audio = blob.type.startsWith("audio/") ? blob : new Blob([blob], { type: audioMimeType(format) });
    return uploadMediaFile(audio, "audio");
}
