# Helper Binary 관리

## 목적

Linux에서 `ot search`, `ot patch`를 실행할 때 시스템에 `rg`, `patch`가 설치되어 있지 않아도 동작하도록 helper binary를 함께 배포합니다.

## 저장 위치

원본 파일은 저장소 안에 다음 경로로 들어 있습니다.

- `runtime-asset/helper-bin/linux/amd64/rg`
- `runtime-asset/helper-bin/linux/amd64/patch`
- `runtime-asset/helper-bin/linux/arm64/rg`
- `runtime-asset/helper-bin/linux/arm64/patch`

실행 시 추출되는 위치는 다음과 같습니다.

- `ORCH_HOME/runtime/bin/<version>/linux-amd64/`
- `ORCH_HOME/runtime/bin/<version>/linux-arm64/`

`<version>`은 빌드 버전을 사용하고, 비어 있으면 `dev`를 사용합니다.

## 동작 방식

- `ot`는 `search`, `patch` 실행 전에 helper가 준비되어 있는지 확인합니다.
- helper가 없거나 실행 권한이 맞지 않으면 다시 추출합니다.
- 셸 스크립트에는 `OT_RG_BIN`, `OT_PATCH_BIN`, `ORCH_HELPER_BIN_DIR` 환경 변수가 전달됩니다.
- Linux에서 helper 준비가 실패하면 명시적으로 실패합니다.

## 재생성 방법

```bash
mkdir -p runtime-asset/helper-bin/linux/amd64 runtime-asset/helper-bin/linux/arm64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o runtime-asset/helper-bin/linux/amd64/rg ./cmd/othelper-rg
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o runtime-asset/helper-bin/linux/amd64/patch ./cmd/othelper-patch
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o runtime-asset/helper-bin/linux/arm64/rg ./cmd/othelper-rg
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o runtime-asset/helper-bin/linux/arm64/patch ./cmd/othelper-patch
```

## 제약

- 현재 helper는 `ot`가 사용하는 `rg`와 `patch` 호출 방식만 지원합니다.
- 지원 대상은 Ubuntu/Debian 계열 Linux입니다.
- `orch`는 직접 helper를 쓰지 않고, 외부 `ot` 바이너리를 통해 간접적으로 사용합니다.
