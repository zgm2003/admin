# Media transport implementation report

## Delivered

- Added named image, audio transcription/synthesis, and video submit/poll/content clients under `server/internal/module/ai/providerRequest`.
- POST requests use frozen credentials, one attempt (`GetBody=nil`), disabled cookie jar/redirect following, bounded JSON responses and sanitized request errors. Image outputs preserve URL/revised prompt while excluding base64 from JSON; binary streams expose MIME/content length and cancel on close.
- Added `server/internal/module/ai/media` spooler with 2-slot concurrency, byte quota accounting until close, SHA-256, 0600 files, MIME/size checks, base64 and stream ingestion, bounded HTTPS URL fetch, redirect/proxy rejection, public-IP DNS validation and pinned dialing.

## Verification

```
cd D:\admin\server
go test ./internal/module/ai/providerRequest ./internal/module/ai/media -count=1
go vet ./internal/module/ai/providerRequest ./internal/module/ai/media
git diff --check -- server/internal/module/ai/providerRequest server/internal/module/ai/media
```

All commands passed. No paid provider, COS, PostgreSQL, large-volume, or codec integration was run.

## Video availability

The official OpenAI video guide states that Sora 2 and the Videos API shut down on September 24, 2026 (verified October 28, 2026): https://developers.openai.com/api/docs/guides/video-generation . The client retains the frozen legacy `/videos` protocol for local compatibility fixtures only; no model/capability is enabled or seeded by this change.

## Integration boundary

The run worker owns persistence, source encryption, transfer deadlines, usage normalization/billing, COS import and retry state. It should immediately spool every returned binary/base64 stream and defer `SpoolFile.Close`; local paths are not recovery facts.
