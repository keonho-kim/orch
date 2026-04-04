# orch

`orch`는 로컬 저장소를 대상으로 동작하는 에이전트 런타임입니다. 현재 구조는 작은 모델에서도 일관된 실행 흐름을 유지하도록, 게이트웨이 조정자와 워커 실행자를 분리한 2계층 구조를 기본으로 사용합니다.

## 핵심 구조

- 게이트웨이: 요청 해석, 작업 분해, 위임, 결과 종합
- 워커: 위임된 작업 계약만 수행
- 모델 도구 표면: `ot` 하나로 고정
- 세션 지속성: `.orch/sessions/*.jsonl` 및 메타데이터
- 런타임 워크스페이스: `test-workspace/`
- 대화형 실행: TUI + 로컬 전용 HTTP/SSE API

## 설정

설정의 단일 소스 오브 트루스는 저장소 루트의 `orch.toml`입니다.

- 기본 경로: `<repo>/orch.toml`
- 공통 오버라이드: `--env-file <path>.toml`
- 사용 범위: `orch`, `orch exec`, `orch config`, TUI, 로컬 API 서버

예시:

```bash
orch config --list
orch config --provider=ollama --model=qwen3.5:35b --endpoint=http://localhost:11434/v1 --reasoning=high
orch config --provider=chatgpt --model=gpt-5.3-codex --api-key="<secret>" --reasoning=xhigh
orch config --env-file=.orch/dev.toml --provider=vllm --model=qwen3.5-coder --endpoint=http://localhost:8000/v1 --reasoning=false
```

`orch config --list`는 정규화된 TOML을 출력하며, `api_key`는 `앞 10글자 + *** + 뒤 5글자` 형식으로 마스킹됩니다.

지원 provider:

- `ollama`
- `vllm`
- `gemini`
- `vertex`
- `bedrock`
- `claude`
- `azure`
- `chatgpt`

reasoning 입력은 `true|false|low|medium|high|xhigh`를 사용합니다. 실제 전송 파라미터는 provider별 capability 규칙에 따라 매핑되며, 지원하지 않는 값은 명시적으로 오류를 반환합니다.

## 런타임 동작

- 게이트웨이와 워커는 서로 다른 OT operation 집합을 사용합니다.
- 실행 루프는 iteration 기반이며, 각 iteration에서 system prompt, context snapshot, tool catalog를 조립합니다.
- OT 실행은 operation registry를 통해 validation, approval, execution을 같은 규칙으로 처리합니다.
- provider transport는 provider spec과 stream decoder helper를 통해 공통 경로를 최대한 공유합니다.

게이트웨이 OT operation:

- `context`
- `task_list`
- `task_get`
- `delegate`
- `read`
- `list`
- `search`

워커 OT operation:

- `context`
- `task_list`
- `task_get`
- `read`
- `list`
- `search`
- `write`
- `patch`
- `check`
- `complete`
- `fail`

Plan mode는 읽기 전용이며 아래 operation만 허용합니다.

- `context`
- `task_list`
- `task_get`
- `read`
- `list`
- `search`

## 로컬 API

대화형 TUI 실행 시 같은 프로세스 안에서 로컬 API 서버가 시작됩니다.

- bind: `127.0.0.1:<ephemeral-port>`
- auth: `Authorization: Bearer <token>`
- discovery 파일:
  - `.orch/api/<session-id>.json`
  - `.orch/api/current.json`

주요 엔드포인트:

- `GET /v1/status`
- `GET /v1/events`
- `POST /v1/exec`
- `GET /v1/exec/{run_id}`
- `GET /v1/exec/{run_id}/events`
- `POST /v1/exec/{run_id}/approval`
- `GET /v1/history`
- `GET /v1/history/latest`
- `POST /v1/history/restore`
- `GET /v1/config`
- `PATCH /v1/config`

## 구현 기준

- provider 기본값과 필수 필드는 provider registry에서 관리합니다.
- 설정 문서와 TUI는 `endpoint`, `model`, `api_key`, `reasoning` 명칭을 공통으로 사용합니다.
- 거대 분기는 가능한 범위에서 registry, dispatcher, reducer 스타일 helper로 분해합니다.
- 레거시 JSON 설정은 더 이상 지원하지 않으며, `orch.toml`로 수동 마이그레이션해야 합니다.

## 검증

```bash
gofmt -w .
go test ./...
go vet ./...
golangci-lint run ./...
```
