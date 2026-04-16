# OpenClaw 개요

이 문서는 OpenClaw를 어떤 종류의 시스템으로 봐야 하는지, 그리고 그 판단을 뒷받침하는 코드·설정·배포 구성이 무엇인지를 정리한다. 결론부터 말하면 OpenClaw는 단순 `chatbot(챗봇)`이나 모델 API 래퍼가 아니라, 멀티채널 `Gateway(게이트웨이)`를 중심으로 세션, 도구 실행, 기기 능력, 플러그인 확장, 운영 제어를 한곳에 모은 `local-first(로컬 우선)` 개인용 `agent platform(에이전트 플랫폼)`에 가깝다.

## 먼저 봐야 할 핵심 판단

OpenClaw의 제품 정체성은 마케팅 문구보다 `openclaw.mjs`, `src/entry.ts`, `src/cli/run-main.ts`, `src/gateway/server.impl.ts`, `src/agents/agent-command.ts`, `src/agents/pi-embedded-runner/run.ts`가 어떤 책임을 지는지를 보면 더 분명해진다. 이 경로들은 설치형 `CLI(명령행 인터페이스)`, 장기 실행 `Gateway(게이트웨이)`, 세션 중심 `agent runtime(에이전트 실행 환경)`, 플러그인 기반 통합, 모바일·데스크톱 `node(노드)`를 하나의 제품 체계로 묶고 있다.

중요한 점은 OpenClaw가 “AI가 답한다”보다 “AI가 사용자의 작업 환경과 기기 권한을 통과해 실제 일을 한다”에 더 가깝다는 사실이다. 코드상으로는 모델 호출 그 자체보다, 모델을 세션·도구·채널·기기 능력·운영 정책과 어떻게 연결할지에 훨씬 더 많은 구조가 배치되어 있다. `src/gateway/**`, `src/agents/**`, `src/plugins/**`, `src/node-host/**`의 두께가 바로 그 증거다.

## OpenClaw를 어떤 종류의 시스템으로 봐야 하는가

실무적으로 보면 OpenClaw는 `self-hosted assistant gateway(셀프 호스팅 비서 게이트웨이)`이자 `agent runtime host(에이전트 실행 호스트)`다. `src/gateway/server.impl.ts`는 인증, 채널 관리자, 웹 제어 표면, `cron(크론)`, 플러그인 런타임, 상태 점검, 노드 세션, 훅, 보조 서비스까지 함께 기동한다. 즉 게이트웨이는 얇은 프록시가 아니라 중앙 제어 허브다.

이 때문에 OpenClaw를 일반적인 “웹 채팅형 AI 앱”으로만 읽으면 구조를 오해하기 쉽다. 보통의 채팅 서비스는 프런트엔드와 서버가 모델 API를 감싸는 정도에서 끝난다. 반면 OpenClaw는 `src/channels/**`, `extensions/*`, `apps/*`, `ui/`, `src/infra/outbound/**`를 통해 여러 메시징 표면과 장치를 하나의 세션·전달 모델로 수렴시킨다.

또한 이 저장소는 단순 로컬 스크립트 묶음도 아니다. `docker-compose.yml`, `render.yaml`, `scripts/k8s/manifests/**`, `.github/workflows/*.yml`을 보면 로컬 우선이 기본이지만, 장기 실행 서비스로 배포하고 원격 접근·업데이트·릴리스까지 운영하는 형태를 분명히 상정한다. 따라서 OpenClaw의 본질은 “로컬에서만 쓰는 도구”보다 “운영자가 소유하는 개인용 실행 시스템”에 가깝다.

## 제품 성격은 다섯 가지 축으로 정리할 수 있다

### `local-first(로컬 우선)`

OpenClaw는 기본적으로 운영자가 자신의 기기나 자신이 통제하는 서버에 게이트웨이를 띄우는 구조다. `docker-compose.yml`은 상태 디렉터리와 `workspace(작업 공간)`를 직접 볼륨 마운트하고, `render.yaml`과 `scripts/k8s/manifests/**`도 상태성 있는 단일 허브를 전제로 한다. 즉 이 프로젝트의 기본값은 완전 호스팅형 `SaaS(서비스형 소프트웨어)`가 아니라 운영자 소유형 실행 환경이다.

이 선택은 프라이버시와 제어권을 높이는 대신 운영 부담을 사용자에게 돌린다. `OPENCLAW_STATE_DIR`, 작업 공간, 인증 토큰, 채널 세션, 모델 자격 증명, 플러그인 활성화 상태를 운영자가 직접 책임져야 하기 때문이다. `src/commands/onboard*`, `src/commands/doctor*`, `src/commands/status*`가 두꺼운 것도 이 부담을 완화하기 위해서다.

### `single-user(단일 사용자)`

현재 저장소 구성만 보면 OpenClaw는 다중 조직 `multi-tenant(다중 임차)` 시스템보다 단일 운영자 비서에 최적화되어 있다. `src/config/sessions/**`, `src/sessions/**`, `src/agents/command/session.ts`를 보면 세션 경계는 조직 권한 모델보다 한 사람의 작업 문맥, 채널 대상, 스레드, 하위 에이전트 관계를 보존하는 방향으로 설계되어 있다.

이 말은 사용자 수가 정확히 1명이어야 한다는 뜻은 아니다. 다만 현재 코드상으로는 “여러 사람이 각자 계정과 과금 체계를 공유하는 협업 제품”보다 “한 운영자가 여러 채널과 장치를 연결해 자기 비서를 운용하는 시스템”으로 읽는 편이 타당하다.

### `terminal-first(터미널 우선)`

OpenClaw는 웹 UI와 모바일 앱을 제공하지만, 시작점은 여전히 터미널이다. `openclaw.mjs`와 `src/entry.ts`는 Node 버전 검사, 컴파일 캐시, 경고 필터, 프로파일/컨테이너 인자 보정, 재실행 계획까지 직접 다룬다. `src/cli/run-main.ts`는 `.env` 로딩, 플러그인 명령 등록, 전역 예외 처리, 런타임 종료 정리까지 포함한다.

이 의미는 단순하다. OpenClaw는 설치형 제품이며, 초기 신뢰 설정과 권한 선택을 숨겨진 마법보다 노출된 운영 흐름으로 처리한다. 이 시스템은 클릭 몇 번으로 끝나는 소비자 앱 설치보다, 운영자가 무엇을 열고 무엇을 연결하는지 알고 시작하는 절차를 선호한다.

### `plugin-first(플러그인 우선)`

OpenClaw의 확장 전략은 선언이 아니라 구조 자체다. `packages/plugin-sdk`, `src/plugins/discovery.ts`, `src/plugins/loader.ts`, `src/plugins/registry.ts`, `src/plugins/runtime/index.ts`는 발견, 검증, 활성화, 로드, 소비 전 과정을 코어 기능으로 취급한다. 이는 선택 기능을 코어에 계속 적층하기보다, 안정된 계약면을 만든 뒤 확장 계층으로 밀어내겠다는 선택이다.

`extensions/*`의 범위도 이를 뒷받침한다. 모델 제공자, 채널, 메모리, 웹 검색, 브라우저, 음성, 진단, 웹훅이 모두 공식 확장 형태로 들어와 있다. 따라서 OpenClaw의 핵심 가치는 특정 기능 수보다 “확장을 받아들이는 계약과 운영 규칙”에 더 가깝다.

### `security-forward(보안 전면 배치)`

OpenClaw는 실행형 시스템답게 보안과 권한 경계를 강하게 드러낸다. `src/gateway/auth.ts`는 `none`, `token`, `password`, `trusted-proxy(신뢰 프록시)`를 분리하고, `src/plugins/discovery.ts`는 플러그인 경로 탈출과 의심스러운 디렉터리 상태를 검사하며, `src/agents/sandbox/**`, `src/agents/tool-policy-pipeline.ts`, `src/node-host/exec-policy.ts`는 도구 실행과 시스템 실행을 교차 검증한다.

이 구조는 선택의 문제가 아니라 제품 성격의 결과다. 메시징 채널 입력, 브라우저 조작, 파일 수정, `system.run`, 모바일 장치 명령은 모두 실제 부작용을 낳는다. 따라서 OpenClaw는 “강력한 기능을 제공하되, 위험한 경로는 운영자가 명시적으로 인지하고 열어야 한다”는 태도를 일관되게 취한다.

## OpenClaw가 줄이려는 불편은 무엇인가

OpenClaw는 모델을 부르는 일을 단순화하려는 프로젝트가 아니다. 더 정확히는 “여러 채널과 기기에서 항상 살아 있는 개인 비서를 운영할 때 생기는 상태·권한·전달·실패 복구의 복잡성”을 줄이려는 프로젝트다.

코드와 설정을 함께 보면 줄이려는 불편은 크게 다섯 가지다.

- 채널 분절: `src/channels/**`, `extensions/*`, `src/infra/outbound/**`는 Telegram, Slack, Discord, WhatsApp, WebChat, 노드 앱 같은 여러 진입 표면을 같은 세션 모델로 묶는다.
- 문맥 단절: `src/sessions/**`, `src/config/sessions/**`, `src/context-engine/**`는 대화 로그가 아니라 작업 문맥 자체를 보존한다.
- 권한 연결의 복잡성: `apps/*`, `src/node-host/**`, `extensions/device-pair`는 카메라, 위치, 음성, 화면 같은 장치 능력을 중앙 정책으로 노출한다.
- 실패 복구 비용: `src/agents/pi-embedded-runner/run.ts`와 하위 `run/*` 모듈은 `retry(재시도)`, `failover(장애 전환)`, 인증 프로필 회전, 컨텍스트 초과, `tool result(도구 결과)` 절단을 코어 로직으로 다룬다.
- 장기 운영의 부담: `src/commands/doctor*`, `src/commands/status*`, `src/logging/**`, `extensions/diagnostics-otel`은 “설치 후 계속 굴리는 시스템”을 전제한 진단 도구를 갖춘다.

## 이 시스템은 누구를 위한 것인가

OpenClaw의 1차 대상은 고급 개인 운영자다. 자신의 채널 계정과 로컬 작업 공간과 기기를 연결해 “항상 켜진 개인 비서”를 굴리려는 사람에게 가장 잘 맞는다. `package.json`의 설명, 채널·세션·노드 관련 코어 구조, 상태성 있는 배포 기본값이 모두 같은 방향을 가리킨다.

2차 대상은 통합 개발자다. `packages/plugin-sdk`, `packages/plugin-package-contract`, `src/plugins/**`, `extensions/*`, 플러그인 계약 테스트들은 “기능을 붙이는 사람”을 중요한 사용자로 본다는 뜻이다. 이 저장소는 일반 사용자를 위한 제품이면서 동시에 기능 생태계를 만들려는 플랫폼이기도 하다.

반대로 다음과 같은 워크로드에는 현재 구조가 덜 맞는다.

- 강한 중앙 과금과 조직 관리가 필요한 `multi-tenant(다중 임차)` SaaS
- 설치와 권한 설명을 최소화해야 하는 저마찰 소비자 앱
- 무상태 웹 서버를 여러 복제본으로 쉽게 수평 확장해야 하는 시스템
- 채널·기기 권한보다 단순 워크플로 `orchestrator(오케스트레이터)`가 더 중요한 제품

## 언어 선택과 저장소 구성이 드러내는 팀의 우선순위

OpenClaw는 다중 언어 저장소지만, 핵심 논리는 `TypeScript(타입스크립트)`에 집중되어 있다. `src/`, `extensions/`, `packages/plugin-sdk`, `ui/`가 모두 TypeScript 중심이며, 이는 프롬프트, 도구, 프로토콜, 설정, 확장 계약을 한 런타임 언어 안에서 조합하려는 의도를 보여 준다.

플랫폼 전용 기능은 네이티브 언어로 분리된다. `apps/macos`는 `Swift(스위프트)`, `apps/android`는 `Kotlin(코틀린)`, `apps/ios`는 iOS 네이티브 스택을 사용해 카메라, 푸시, 위치, 오디오, 화면 녹화 같은 권한 문제를 직접 다룬다. 이는 “모든 것을 웹으로 밀어 넣겠다”보다 “권한과 장치 기능은 각 플랫폼의 원어로 처리하겠다”는 실용주의다.

운영 보조층도 마찬가지다. `pnpm-workspace.yaml`은 코어, UI, 공유 패키지, 확장을 하나의 `monorepo(모노레포)`로 묶고, `scripts/**`, `.github/workflows/*.yml`, Docker/Kubernetes 파일은 빌드·검증·릴리스·배포를 뒷받침한다. 즉 우선순위는 “언어 통일”보다 “핵심 경계 통일과 운영 현실 대응”에 더 가깝다.

## 주요 의존성은 아키텍처에서 어떤 의미를 갖는가

아래 표는 OpenClaw의 중요한 의존성 범주와 그 의미를 요약한 것이다. 단순 목록보다 “이 프로젝트가 어떤 제품이 되려 하는가”를 읽는 데 초점을 맞췄다.

| 계층 | 대표 근거 | 아키텍처 의미 |
| --- | --- | --- |
| `@mariozechner/pi-*` 계열 실행기 | `package.json`, `src/agents/pi-embedded-runner/**` | OpenClaw가 모델 API 래퍼가 아니라 도구 호출형 `agent runtime(에이전트 실행 환경)`을 내장한다는 뜻이다. 핵심 가치는 UI보다 실행 루프에 있다. |
| `plugin SDK(플러그인 SDK)` / `package contract(패키지 계약)` | `packages/plugin-sdk`, `packages/plugin-package-contract`, `src/plugins/**` | 기능 외연을 넓히되 코어 내부 구현과 외부 확장 계약을 분리하겠다는 선택이다. |
| `@modelcontextprotocol/sdk` | `src/mcp/channel-server.ts`, `src/mcp/plugin-tools-serve.ts`, `package.json` | OpenClaw가 자체 도구·채널 런타임을 외부 툴링과 연결하는 `MCP bridge(MCP 브리지)` 표면을 제공한다는 뜻이다. |
| `@agentclientprotocol/sdk` | `src/acp/server.ts`, `src/acp/control-plane/**`, `package.json` | OpenClaw가 외부 `agent protocol(에이전트 프로토콜)`과도 연결되며, 백그라운드 작업과 세션 제어를 더 넓은 도구 생태계와 접속시킨다는 신호다. |
| `OpenTelemetry(오픈텔레메트리)` | `extensions/diagnostics-otel/src/service.ts` | 실행형 시스템의 관측성을 플러그인 계층까지 확장해 실제 운영 시스템으로 다듬고 있다는 의미다. |
| Docker/배포 계층 | `docker-compose.yml`, `render.yaml`, `scripts/k8s/manifests/**` | 로컬 우선이지만 배포 옵션을 포기하지 않는다. 다만 무상태 수평 확장보다 상태성 있는 단일 허브 운영을 전제한다. |

이 표가 보여 주는 핵심은 OpenClaw가 특정 모델 회사나 특정 채널에 묶인 제품이 아니라는 점이다. 오히려 이 저장소는 “에이전트 실행을 둘러싼 경계와 운영 표면”을 핵심 경쟁력으로 본다.

## 강점과 한계, 차별점은 무엇인가

### 강점

- 멀티채널 `Gateway(게이트웨이)` 구조가 일관적이다. 여러 진입 표면을 같은 세션·권한·전달 모델로 묶는다.
- 세션이 두껍다. 단순 대화 로그가 아니라 작업 문맥, 모델 상태, 전달 경로, 하위 에이전트 관계를 유지한다.
- 확장 전략이 현실적이다. 플러그인 계약을 분리하고 공식 확장을 같은 저장소에서 관리해 생태계와 통제를 동시에 잡는다.
- 권한 경계가 눈에 보인다. 샌드박스, 승인, 노드 실행 정책, 플러그인 경로 검사까지 부작용 경로를 세밀하게 다룬다.

### 제약

- 상태성 단일 허브 의존이 강하다. 개인 운영에는 맞지만 무상태 확장성과는 충돌한다.
- 저장소 범위가 넓다. 채널, 제공자, 모바일 앱, 브라우저, 메모리, 진단까지 함께 관리하므로 유지보수 비용이 크다.
- 운영자 친화적이다. 강한 제어권의 대가로 초기 설정과 개념 부담이 무겁다.
- 외부 생태계 의존도가 높다. 메시징 SDK, 모델 제공자 API, 모바일 플랫폼 정책 변화에 지속적으로 노출된다.

### 차별점

가장 먼저 눈에 띄는 차별점은 “멀티채널 개인 허브”와 “실행형 에이전트 런타임”을 하나의 제품으로 묶어 놓았다는 점이다. 채팅 앱, 워크플로 엔진, 코드 에이전트, 브라우저 자동화 시스템이 보통 서로 다른 제품군으로 나뉘는 것과 달리, OpenClaw는 이들을 게이트웨이 중심 구조 안에서 함께 다룬다.

또 다른 차별점은 `ACP/MCP bridge(ACP/MCP 브리지)`와 태스크·세션 관리가 코어에 꽤 깊게 연결되어 있다는 점이다. 이는 단순히 “툴을 더 붙일 수 있다”를 넘어, 외부 에이전트 생태계와 백그라운드 작업을 OpenClaw의 세션·승인·전달 모델 아래로 끌어들인다는 뜻이다.

## 비전문가에게는 어떻게 설명할 수 있는가

쉽게 말하면 OpenClaw는 “여러 메신저와 여러 기기에서 같은 개인 AI 비서를 부를 수 있게 해 주는 자기 소유형 운영 허브”다. 웹사이트 하나에서만 답하는 AI와 달리, OpenClaw는 사용자의 채널 계정, 로컬 작업 공간, 기기 능력, 자동화, 배경 작업을 하나의 시스템으로 묶는다.

조금 더 기술적으로 말하면, 메시지와 이벤트는 어디서 들어오든 게이트웨이로 모이고, 거기서 세션과 권한과 모델과 도구를 정한 뒤 실제 실행으로 이어진다. 그래서 OpenClaw의 핵심 가치는 문장을 잘 쓰는 모델 하나보다, 사용자의 환경을 지속적으로 연결하고 통제하는 실행 구조에 있다.

## 정리

OpenClaw를 가장 잘 설명하는 문장은 “멀티채널 `Gateway(게이트웨이)`를 중심으로 한 로컬 우선 개인용 `agent platform(에이전트 플랫폼)`”이다. 이 저장소는 단순한 AI 프런트엔드가 아니라, 세션·도구·채널·기기·권한·배포를 한 시스템으로 엮는 실행 플랫폼을 만들고 있다.

현재 코드 기준으로 볼 때 OpenClaw의 강점은 일관된 허브 구조, 두꺼운 세션 모델, 플러그인 계약, 명시적 권한 경계에 있다. 반대로 유지보수 비용과 운영 부담도 상당하다. 따라서 이 프로젝트는 “누구나 바로 쓰는 소비자 앱”보다 “자기 환경을 통제하면서 강한 실행형 비서를 굴리려는 운영자와 통합 개발자”에게 더 잘 맞는 시스템으로 보는 편이 정확하다.
