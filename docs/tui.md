# TUI

## 개요

TUI는 `orch`의 기본 운영 인터페이스입니다. `orch exec`와 같은 orchestrator 서비스를 사용하지만, 세션 복원, 승인 모달, 설정 편집, 로컬 API 노출을 함께 제공합니다.

대화형 TUI 실행 시 `.orch/api/current.json`에 로컬 API 연결 정보가 기록됩니다.

## 화면 구성

| 영역 | 역할 |
| --- | --- |
| timeline | run 이력, 스트리밍 출력, 활성 run reasoning 표시 |
| composer | 프롬프트 입력, slash command, 실행 모드 전환 |
| command meta | 현재 모드, provider, model 표시 |
| status line | 상태, 경고, 저장 결과 표시 |
| modal | approval, settings, history, exit confirmation |

## 초기 설정 게이트

기본 provider와 model이 유효하지 않으면 TUI는 바로 설정 플로우로 진입합니다.

현재 동작:

- provider를 먼저 선택합니다.
- `Ollama`를 선택하면 endpoint 확인과 모델 자동 탐색을 진행합니다.
- 나머지 provider는 수동 설정 폼으로 이동합니다.
- 설정 저장 대상은 항상 현재 `orch.toml`입니다.
- 기본 provider가 유효해질 때까지 run을 시작할 수 없습니다.

## Composer 모드

입력기는 두 가지 고정 모드를 가집니다.

- `ReAct Deep Agent`
- `Plan`

`Shift+Tab`으로 전환합니다.

## Slash Commands

입력이 `/`로 시작하면 composer 위에 slash command 메뉴가 열립니다.

현재 명령:

| 명령 | 동작 |
| --- | --- |
| `/clear` | 새 세션을 열고 현재 대화를 비웁니다 |
| `/compact` | 세션 compact를 강제로 실행합니다 |
| `/context` | 최신 context snapshot을 보여줍니다 |
| `/exit` | 애플리케이션을 종료합니다 |
| `/status` | 현재 세션과 run 상태를 보여줍니다 |
| `/tasks` | 현재 세션의 직속 child task 목록 또는 상세를 보여줍니다 |

## Approval 모달

도구 실행에 승인이 필요하면 대시보드 대신 approval 화면이 표시됩니다.

표시 정보:

- run id
- tool name
- approval reason
- raw tool arguments

조작:

- `Enter` 또는 `Y`: 승인
- `Esc` 또는 `N`: 거부

## Session History

`orch history`는 같은 TUI를 실행하고 세션 선택기를 바로 엽니다.

선택기에서 보이는 정보:

- session id
- title
- parent session 연계 정보
- worker role
- task status
- provider / model

## Settings 모달

`Ctrl+S`로 설정 창을 엽니다.

현재 설정 UI는 아래 항목을 편집합니다.

- 기본 provider
- `self_driving_mode`
- provider별 `endpoint`
- provider별 `model`
- provider별 `api_key`
- provider별 `reasoning`
- `react_ralph_iter`
- `plan_ralph_iter`
- `compact_threshold_k`

설정 UI는 더 이상 scope 탭을 사용하지 않습니다. 단일 `orch.toml` 문서를 직접 편집하는 구조입니다.

설정 필드 구성은 provider별 개별 enum 나열이 아니라 schema 기반으로 생성됩니다.

- 공통 필드: provider, `self_driving_mode`, `react_ralph_iter`, `plan_ralph_iter`, `compact_threshold_k`
- provider 필드: `endpoint`, `model`, `api_key`, `reasoning`
- field order, label, primary field는 provider와 field kind 조합에서 파생됩니다.

provider 변경 시에는 확인 단계를 거칩니다.

동일 설정은 CLI에서도 관리할 수 있습니다.

- `orch config --list`
- `orch config --provider=ollama --model=<name> --endpoint=<url> --reasoning=<value>`
- `orch config --provider=chatgpt --model=<name> --api-key=<secret> --reasoning=<value>`
- `orch config --approval-policy=<policy>`
- `orch config --self-driving-mode=<true|false>`
- `orch config --react-ralph-iter=<n>`
- `orch config --plan-ralph-iter=<n>`
- `orch config --compact-threshold-k=<n>`
- `orch config --env-file=<path>.toml ...`

## Reasoning 표시

`Ctrl+T`는 활성 run의 reasoning 표시를 토글합니다.

표시 방식:

- 확장된 `THINK` 블록
- 접힌 `THINKING ...` 플레이스홀더

reasoning은 TUI에서만 보이며 세션 transcript 파일에는 저장되지 않습니다.

## 내비게이션

| 키 | 동작 |
| --- | --- |
| `Up` / `Down` | slash menu가 없을 때 프롬프트 히스토리 이동 |
| `PgUp` / `PgDn` | timeline 페이지 이동 |
| `Home` / `End` | 맨 위 / 맨 아래 이동 |
| mouse wheel | timeline 스크롤 |
| `Ctrl+S` | 설정 창 열기 |
| `Ctrl+T` | reasoning 표시 토글 |
| `Ctrl+C` | 즉시 종료 |

## `/clear` 와 `/exit`

`/clear`는 다음을 수행합니다.

1. 새 세션 생성
2. UI를 새 세션으로 전환
3. 이전 세션은 백그라운드 finalize

제약:

- 활성 run이 있는 동안에는 새 세션을 열지 않습니다.

`/exit`는 정상 종료 경로입니다. 활성 run이 있으면 먼저 확인 모달이 열립니다.
