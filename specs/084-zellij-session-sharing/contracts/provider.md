# Provider Contract

A provider exposes stable name, non-mutating prerequisite validation, endpoint start, bounded readiness returning one public HTTPS URL, safe status, and idempotent stop.

The shared contract suite covers success, missing binary, startup failure, readiness timeout, malformed endpoint output, unexpected exit, repeated stop, and stop failure. V1 runs it against Cloudflare through a fake command runner and an isolated fake provider.

