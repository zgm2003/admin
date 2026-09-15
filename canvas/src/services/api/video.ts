// 临时桩：视频生成调用层原为浏览器直连 OpenAI/Gemini 的实现，已按静态阶段回退。
// 接入 admin 后端时在保持导出签名不变的前提下整体替换任务创建/轮询部分。
import i18n from "@/i18n";
import { uploadMediaFile, type UploadedFile } from "@/services/file-storage";
import type { AiConfig } from "@/stores/use-config-store";
import type { ReferenceImage } from "@/types/image";
import type { ReferenceAudio, ReferenceVideo } from "@/types/media";

type RequestOptions = { signal?: AbortSignal };
type VideoMediaOptions = RequestOptions & { videos?: ReferenceVideo[]; audios?: ReferenceAudio[] };

export type VideoGenerationResult = { blob?: Blob; url?: string; mimeType?: string };
export type VideoGenerationTask = { id: string; provider: "openai" | "gemini" | "plugin"; model: string };
export type VideoGenerationTaskState = { status: "pending" } | { status: "completed"; result: VideoGenerationResult } | { status: "failed"; error: string };

function notConnected(): Error {
    return new Error(i18n.t("common.notConnected"));
}

export async function requestVideoGeneration(_config: AiConfig, _prompt: string, _references: ReferenceImage[] = [], _options?: VideoMediaOptions): Promise<VideoGenerationResult> {
    throw notConnected();
}

export async function createVideoGenerationTask(_config: AiConfig, _prompt: string, _references: ReferenceImage[] = [], _options?: VideoMediaOptions): Promise<VideoGenerationTask> {
    throw notConnected();
}

export async function pollVideoGenerationTask(_config: AiConfig, _task: VideoGenerationTask, _options?: RequestOptions): Promise<VideoGenerationTaskState> {
    throw notConnected();
}

export async function waitForVideoGenerationTask(_config: AiConfig, _task: VideoGenerationTask, _options?: RequestOptions): Promise<VideoGenerationResult> {
    throw notConnected();
}

export function isVideoTaskFailed(error: unknown) {
    return error instanceof Error && error.name === "VideoTaskFailed";
}

export async function storeGeneratedVideo(result: VideoGenerationResult): Promise<UploadedFile> {
    if (result.blob) return uploadMediaFile(result.blob, "video");
    if (result.url) {
        try {
            return await uploadMediaFile(result.url, "video");
        } catch {
            return { url: result.url, storageKey: "", bytes: 0, mimeType: result.mimeType || "video/mp4" };
        }
    }
    throw new Error(i18n.t("apiErrors.noPlayableVideo"));
}
