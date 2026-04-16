# 도구 계약

## 공개 도구 표면

모델이 호출하는 구조화 도구는 `ot` 하나입니다.  
`orch`는 실행 흐름을 조정하고, 실제 파일 작업과 점검은 `ot`에 위임합니다.

## 역할별 허용 연산

| 연산 | Gateway | Worker | Plan |
| --- | --- | --- | --- |
| `context` | 가능 | 가능 | 가능 |
| `task_list` | 가능 | 가능 | 가능 |
| `task_get` | 가능 | 가능 | 가능 |
| `session_search` | 가능 | 가능 | 가능 |
| `memory_search` | 가능 | 가능 | 가능 |
| `skill_list` | 가능 | 가능 | 가능 |
| `skill_get` | 가능 | 가능 | 가능 |
| `read` | 가능 | 가능 | 가능 |
| `list` | 가능 | 가능 | 가능 |
| `search` | 가능 | 가능 | 가능 |
| `delegate` | 가능 | 불가 | 불가 |
| `memory_commit` | 가능 | 불가 | 불가 |
| `skill_propose` | 가능 | 불가 | 불가 |
| `write` | 불가 | 가능 | 불가 |
| `patch` | 불가 | 가능 | 불가 |
| `check` | 불가 | 가능 | 불가 |
| `complete` | 불가 | 가능 | 불가 |
| `fail` | 불가 | 가능 | 불가 |

## 승인 정책

다음 연산은 기본적으로 승인 대상입니다.

- `write`
- `patch`
- `check`
- `memory_commit`
- `skill_propose`

`check`는 워커 실행에서 `self_driving_mode`가 켜져 있으면 자동 승인 없이 진행할 수 있습니다.

## `ot`의 역할

`ot`는 다음 작업을 담당합니다.

- 파일과 디렉터리 읽기
- 이름 또는 내용 검색
- 파일 쓰기와 패치 적용
- 정적 점검 실행
- 세션·기억·스킬 조회

읽기 계열 연산은 계획 실행에서도 사용할 수 있습니다.  
쓰기 계열 연산은 워커에게만 열립니다.

## 독립 실행 도구로서의 `ot`

`ot`는 `orch` 바깥에서도 실행할 수 있는 별도 바이너리입니다.  
다만 일반적인 사용 흐름에서는 `orch`가 `ot`를 내부적으로 호출합니다.

Linux에서는 `search`, `patch` 실행 전에 helper binary를 준비합니다.  
자세한 내용은 [Helper Binary 관리](./helper-binaries.md)에서 설명합니다.
