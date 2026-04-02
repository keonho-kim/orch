# Helper Binary 관리

## 목적

`ot search`와 `ot patch`가 Ubuntu/Debian 계열 Linux에서 시스템 `rg`, `patch` 설치 여부에 의존하지 않도록 helper binary를 `ot`에 내장한다.

## 저장 위치

리포지토리 내 helper binary 원본 저장 경로는 다음과 같다.

- `runtime-asset/helper-bin/linux/amd64/rg`
- `runtime-asset/helper-bin/linux/amd64/patch`
- `runtime-asset/helper-bin/linux/arm64/rg`
- `runtime-asset/helper-bin/linux/arm64/patch`

런타임 추출 경로는 다음과 같다.

- `${os.UserConfigDir()}/orch/runtime/bin/<version>/linux-amd64/`
- `${os.UserConfigDir()}/orch/runtime/bin/<version>/linux-arm64/`

`<version>`은 `ot` 빌드 버전을 사용하고, 비어 있으면 `dev`를 사용한다.

## 동작 방식

- `cmd/ot`는 helper 준비 함수를 주입받아 `search`, `patch` 실행 전에 helper를 보장한다.
- helper가 없거나 실행 권한이 맞지 않으면 다시 추출한다.
- 추출된 helper 경로는 `OT_RG_BIN`, `OT_PATCH_BIN`, `ORCH_HELPER_BIN_DIR` 환경변수로 셸 스크립트에 전달된다.
- Linux에서 helper 준비가 실패하면 fallback 없이 명시적으로 실패한다.

## 재생성 방법

helper binary는 아래 명령으로 다시 만들 수 있다.

```bash
mkdir -p runtime-asset/helper-bin/linux/amd64 runtime-asset/helper-bin/linux/arm64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o runtime-asset/helper-bin/linux/amd64/rg ./cmd/othelper-rg
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o runtime-asset/helper-bin/linux/amd64/patch ./cmd/othelper-patch
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o runtime-asset/helper-bin/linux/arm64/rg ./cmd/othelper-rg
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o runtime-asset/helper-bin/linux/arm64/patch ./cmd/othelper-patch
```

## 제약

- 현재 helper는 `ot`가 실제로 사용하는 `rg --line-number --color never --no-heading [--hidden] -- <pattern> <paths...>`와 `patch -p0 -u` 호출 형태만 지원한다.
- 지원 대상은 Ubuntu/Debian 계열 Linux다.
- helper binary는 `ot`에만 내장되고, `orch`는 외부 `ot` 바이너리에 파일 작업을 위임한다.
