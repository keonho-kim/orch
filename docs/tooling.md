# 도구 계약

## 모델 노출 계약

모델이 호출할 수 있는 구조화 도구는 `ot` 하나뿐이다.

`exec`는 더 이상 모델 노출 계약에 포함되지 않는다.

`bootstrap/TOOLS.md`는 프롬프트에 주입되는 텍스트 기반 도구 안내서다.

## 역할별 OT 연산

### Gateway

- `context`
- `task_list`
- `task_get`
- `delegate`
- `read`
- `list`
- `search`

### Worker

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

### Plan Mode

Plan mode는 읽기 전용이다.

- `context`
- `task_list`
- `task_get`
- `read`
- `list`
- `search`

## 실행 책임

- `orch`는 OT 파일 작업을 직접 실행하지 않고 외부 `ot` 바이너리에 위임한다.
- `orch` 내부에는 `delegate`, 컨텍스트 조회, 작업 조회, 체크 실행, 종료 상태 처리만 남긴다.
- standalone `ot`는 `tools/ot/*.sh`를 계속 사용하지만, `search`와 `patch`는 실행 전에 helper 준비를 수행한다.

## Helper Binary

- Ubuntu/Debian 계열 Linux에서는 `rg`, `patch`를 시스템 설치 여부와 무관하게 `ot` 바이너리 내장 helper로 제공한다.
- helper 원본 파일은 `runtime-asset/helper-bin/linux/<arch>/` 아래에 둔다.
- helper 추출 경로는 `${os.UserConfigDir()}/orch/runtime/bin/<version>/linux-<arch>/` 이다.
- `search.sh`는 `${OT_RG_BIN:-rg}`를 사용하고, `patch.sh`는 `${OT_PATCH_BIN:-patch}`를 사용한다.
- 비 Linux 플랫폼에서는 helper 추출을 건너뛰고 기존 시스템 명령 경로를 유지한다.

## 비고

- 모델은 raw shell command를 직접 작성하지 않는다.
- 모델은 `cwd`, `stdin`, 자유 형식 argv를 직접 제어하지 않는다.
- 쓰기, 패치, 체크는 명시적인 worker 연산으로만 노출된다.
- gateway delegation은 내부 child worker run으로 처리된다.
- `task_list`와 `task_get`은 delegated child run을 작업 뷰로 노출한다.
- `context`는 현재 run의 persisted prompt input snapshot을 노출한다.
