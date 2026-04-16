# 실행 워크스페이스와 상태 디렉터리

## 전역 상태 루트

기본 전역 상태 루트는 `~/.orch`입니다.  
다른 경로를 쓰려면 `ORCH_HOME` 환경 변수를 지정합니다.

```text
$ORCH_HOME/
├── orch.env.toml
├── state.db
├── logs/
├── runtime/
│   ├── assets/<version>/
│   └── bin/<version>/<platform>/
└── workspaces/<workspace-id>/
    ├── api/
    ├── runtime/
    └── sessions/
```

## 워크스페이스별 디렉터리

각 저장소는 별도의 워크스페이스 상태 디렉터리를 가집니다.

| 경로 | 역할 |
| --- | --- |
| `workspaces/<workspace-id>/runtime/` | 실행용 워크스페이스 |
| `workspaces/<workspace-id>/api/` | 로컬 API discovery 파일 |
| `workspaces/<workspace-id>/sessions/` | 세션 기록과 메타데이터 |

`workspace-id`는 저장소 절대 경로를 기준으로 만들어집니다.

## 실행 워크스페이스

실행은 저장소 원본 디렉터리 위에서 직접 파일을 흩뿌리며 동작하지 않습니다.  
`runtime/` 아래에 준비된 실행 워크스페이스를 기준으로 도구와 자산을 배치하고, 필요한 상태는 별도 디렉터리에 저장합니다.

실행 워크스페이스에는 다음 항목이 동기화됩니다.

- 실행에 필요한 공통 문서 자산
- 도구 스크립트
- 스킬 자산

일부 사용자 지속 정보는 다시 준비해도 보존됩니다.

## API와 세션 저장

대화형 세션이 열리면 `api/` 아래에 discovery 파일이 생성됩니다.  
세션 기록은 `sessions/` 아래에 JSONL과 메타데이터 파일로 남습니다.

이 구조 덕분에 프로젝트 루트에는 설정 파일 외의 런타임 상태를 남기지 않아도 됩니다.
