# 세션 모델

## 저장 위치

세션과 실행 상태는 `ORCH_HOME` 아래에 저장됩니다.

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
    │   ├── current.json
    │   └── <session-id>.json
    ├── runtime/
    └── sessions/
        ├── <session-id>.jsonl
        └── <session-id>.meta.json
```

프로젝트별 워크스페이스 식별자는 저장소 절대 경로를 기준으로 고정됩니다.

## 세션 기록

세션 기록은 두 계층으로 남습니다.

- `sessions/<session-id>.jsonl`
  실행 중 쌓이는 사용자 입력, 응답, 도구 결과, 문맥 스냅샷
- `sessions/<session-id>.meta.json`
  세션 제목, 제공자, 모델, 작업 상태, lineage 정보

주요 메타데이터는 SQLite에도 함께 반영됩니다.

## 작업과 lineage

위임된 워커 실행은 독립적인 자식 세션으로 남습니다.  
부모 세션과의 관계는 다음 필드로 추적합니다.

- `parent_session_id`
- `parent_run_id`
- `parent_task_id`
- `worker_role`
- `task_title`
- `task_contract`
- `task_status`

작업 결과에는 다음 정보가 포함될 수 있습니다.

- `task_summary`
- `task_changed_paths`
- `task_checks_run`
- `task_evidence_pointers`
- `task_followups`
- `task_error_kind`

## 실행 문맥 스냅샷

각 실행은 호출 시점의 문맥을 세션 기록으로 남깁니다.  
여기에는 다음과 같은 관측 가능한 정보가 들어갑니다.

- 제공자와 모델
- 워크스페이스 경로와 현재 작업 디렉터리
- 세션 요약 존재 여부
- 현재 실행에 포함된 기록 수
- 선택된 스킬 수
- 기억 스냅샷과 스킬 스냅샷 개수

이 스냅샷은 나중에 세션을 복원하거나 실행 이력을 점검할 때 사용합니다.

## 기억과 스킬

기억과 스킬 정보는 SQLite에 저장됩니다.

- 사용자 선호
- 저장소 수준 사실
- 작업에서 얻은 교훈
- 반복 패턴에서 추린 절차 스킬

실행이 시작되면 현재 시점의 기억과 스킬이 스냅샷으로 고정됩니다.  
실행 도중 새로 저장된 내용은 같은 실행에 즉시 반영되지 않습니다.
