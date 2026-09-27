# 60-posted — Grafana Foundation SDK(Go)로 대시보드를 코드로 관리하기

UI에서 만들고 JSON으로 export 하던 대시보드를 **Go 코드로 선언**하고, 생성된 JSON을 **Terraform**으로 Grafana에 배포하는 예제입니다.

```
Go 코드 (foundation-sdk/go)  ──make build──▶  dist/*.json  ──terraform apply──▶  Grafana
         │                                        │
         └── go test (PromQL/레이아웃 검증)          └── git diff 로 리뷰 / CI 에서 최신 여부 확인
```

예제 대시보드는 두 개입니다.

| 파일 (`dist/`) | 대시보드 | 설명 |
| --- | --- | --- |
| `service-health.json` | Service Health — CPU / Memory / Disk I/O | **새로 만든 대시보드.** 특정 서비스(workload)의 CPU / Memory / Disk I/O 상태 |
| `finops-opencost.json` | FinOps — AWS 환산 비용 (OpenCost) | **기존 대시보드 마이그레이션.** `59-posted`의 JSON을 SDK 코드로 옮긴 것 |

---

## 목차

1. [대시보드 스키마 v1 vs v2 — 무엇을 쓸까?](#1-대시보드-스키마-v1-vs-v2--무엇을-쓸까)
2. [디렉토리 구조](#2-디렉토리-구조)
3. [사전 준비](#3-사전-준비)
4. [빠른 시작](#4-빠른-시작)
5. [예제 대시보드 설명](#5-예제-대시보드-설명)
6. [새 대시보드 추가하기](#6-새-대시보드-추가하기)
7. [검증 — 로컬](#7-검증--로컬)
8. [검증 — GitHub Actions](#8-검증--github-actions)
9. [Terraform으로 홈랩에 배포하기](#9-terraform으로-홈랩에-배포하기)
10. [알아둘 점 / 제약 사항](#10-알아둘-점--제약-사항)
11. [참고 자료](#11-참고-자료)

---

## 1. 대시보드 스키마 v1 vs v2 — 무엇을 쓸까?

**결론: 지금은 v1(기존 JSON 모델)을 씁니다.** Grafana 13으로 업그레이드한 뒤 v2를 다시 검토합니다.

### 홈랩 Grafana 버전

- 레포에 있는 export JSON(`50-posted`, `59-posted`)의 `pluginVersion`이 모두 **`12.0.1`** 입니다. 그래서 홈랩 Grafana는 **12.0.1**로 봅니다.
- 이 작업은 홈랩 클러스터에 접근할 수 없는 환경에서 진행해서 직접 조회하지는 못했습니다. 아래 명령으로 한 번 확인해 주세요.

  ```bash
  # 방법 1) 이미지 태그
  kubectl get pods -A -l app.kubernetes.io/name=grafana \
    -o jsonpath='{range .items[*]}{.metadata.namespace}{"\t"}{.spec.containers[*].image}{"\n"}{end}'

  # 방법 2) API
  curl -s https://<grafana-주소>/api/health | jq .version
  ```

  버전이 다르면 `docker-compose.yaml`, `.github/workflows/60-posted.yml`의 `12.0.1`과 `foundation-sdk/go/internal/common/common.go`의 `SchemaVersion`을 함께 맞춰 주세요.

### v2 스키마 현황 (2026-09 기준)

| 항목 | 내용 |
| --- | --- |
| Grafana 12.0 ~ 12.x | v2 스키마(dynamic dashboards)는 **실험 기능**입니다. `kubernetesDashboards`, `dashboardNewLayouts` feature toggle을 켜야 쓸 수 있습니다. |
| Grafana 13 (2026-04) | dynamic dashboards **GA**. 기본으로 켜지고, 기존 대시보드는 열 때 v2로 자동 마이그레이션됩니다. |
| Foundation SDK | `dashboardv2beta1`, `dashboardv2` 패키지가 있어서 코드로는 v2도 만들 수 있습니다. |
| Terraform `grafana_dashboard` | **v1 JSON만** 지원합니다. |
| Terraform v2 리소스 | `grafana_apps_dashboard_dashboard_v2` 등 별도 리소스가 있지만 **Grafana 13 이상**이 대상입니다. v2 대시보드와 관련된 이슈도 아직 열려 있습니다 (예: [#2543](https://github.com/grafana/terraform-provider-grafana/issues/2543), [#2911](https://github.com/grafana/terraform-provider-grafana/issues/2911), [#2982](https://github.com/grafana/terraform-provider-grafana/issues/2982)). |

그래서 홈랩(12.0.1)에서 v2를 쓰려면 실험 기능 toggle을 켜야 하고, Terraform 쪽도 13 이상을 전제로 해서 맞지 않습니다. v1으로 만들어 두면 13으로 올린 뒤에도 Grafana가 자동으로 v2로 바꿔서 보여주기 때문에 손해가 없습니다.

---

## 2. 디렉토리 구조

```
60-posted/
├── Makefile                       # 모든 작업의 진입점 (make help)
├── docker-compose.yaml            # 로컬 검증용 Grafana 12.0.1
├── local/provisioning/            # 로컬 Grafana 데이터소스 프로비저닝
├── scripts/smoke.sh               # Grafana API 업로드 스모크 테스트
├── dist/                          # ★ 생성된 대시보드 JSON (커밋 대상, 직접 수정 금지)
│   ├── finops-opencost.json
│   └── service-health.json
├── foundation-sdk/go/
│   ├── main.go                    # 생성기: 등록된 대시보드를 dist/ 로 출력
│   ├── main_test.go               # 검증 테스트 (PromQL 문법, 레이아웃, ID 중복 등)
│   ├── internal/common/           # 공통 헬퍼 (데이터소스, 변수, 패널 기본값, 후처리)
│   ├── dashboards/
│   │   ├── servicehealth/         # 새 대시보드
│   │   └── finops/                # 59-posted 마이그레이션
│   └── tools/
│       ├── convert/               # 기존 JSON → Go 코드 초안 변환
│       └── compare/               # 원본 JSON vs 생성 JSON 의미 비교
└── terraform/
    ├── versions.tf / providers.tf / variables.tf / main.tf / outputs.tf
    └── envs/
        ├── local.tfvars           # 로컬 docker Grafana 용
        └── homelab.tfvars.example # 홈랩 용 예시 (복사해서 사용, 커밋 안 됨)
```

**`dist/`를 커밋하는 이유**
- Terraform이 이 파일을 그대로 읽기 때문에 Go 없이 `terraform plan`만 해도 됩니다.
- PR에서 코드 변경이 **실제 JSON을 어떻게 바꾸는지** diff로 바로 보입니다.
- 대신 코드만 고치고 `make build`를 잊는 실수를 막으려고 CI에서 `make check`로 검사합니다.

---

## 3. 사전 준비

| 도구 | 버전 | 용도 |
| --- | --- | --- |
| Go | 1.24 이상 | 대시보드 생성 / 테스트 |
| Terraform | 1.5 이상 (CI는 1.9.8) | 배포 |
| Docker (+ compose) | - | 로컬 Grafana |
| make, jq, curl | - | Makefile, 스모크 테스트 |

사용하는 라이브러리 버전:
- `github.com/grafana/grafana-foundation-sdk/go` **v0.0.20** (`go.mod`에 고정)
- Terraform provider `grafana/grafana` **~> 4.46**

---

## 4. 빠른 시작

```bash
cd 60-posted

make help          # 타겟 목록
make build         # Go 코드 → dist/*.json
make verify        # lint + test + dist 최신 여부 + 원본 비교 (CI 1단계와 동일)

make up            # 로컬 Grafana 12.0.1 → http://localhost:3000 (admin / admin)
make smoke         # API 로 올려보고 다시 읽어서 확인 (끝나면 자동 삭제)
make tf-local      # 로컬 Grafana 에 terraform apply + 재 plan 에서 변경 없음 확인
make down          # 정리
```

---

## 5. 예제 대시보드 설명

### 5-1. Service Health — CPU / Memory / Disk I/O (새로 만든 대시보드)

코드: [`foundation-sdk/go/dashboards/servicehealth/servicehealth.go`](foundation-sdk/go/dashboards/servicehealth/servicehealth.go)

**변수**

| 변수 | 값 | 비고 |
| --- | --- | --- |
| `datasource` | Prometheus 데이터소스 | 모든 패널이 `${datasource}`를 참조해서 UID를 하드코딩하지 않습니다 |
| `namespace` | `label_values(kube_pod_info, namespace)` | |
| `workload` | `kube_pod_info`의 `created_by_name`에서 ReplicaSet 해시를 정규식으로 제거 | Deployment / StatefulSet / DaemonSet 모두 선택 가능 |
| `pod` | `$workload-.*`에 맞는 파드 (다중 선택, 기본 All) | |
| `rate_window` | 1m / 2m / 5m / 10m (기본 5m) | `rate()` 윈도우 |

**패널 구성**

| 행 | 패널 |
| --- | --- |
| 요약 | Ready 파드, 재시작(1h), CPU 사용량, CPU / Request, Memory(Working Set), Memory / Limit, Disk Read, Disk Write |
| CPU | 파드별 사용량 · 합계 vs Request/Limit(점선 기준선) · 스로틀링 비율 |
| Memory | 파드별 Working Set · 합계 vs Request/Limit · 컨테이너 재시작 |
| Disk I/O | 처리량(Bps) · IOPS — 읽기는 위(+), 쓰기는 아래(-) · 네임스페이스 PVC 사용률 |

**필요한 메트릭** — kube-prometheus-stack이면 기본으로 수집됩니다.
- cAdvisor: `container_cpu_usage_seconds_total`, `container_cpu_cfs_*`, `container_memory_working_set_bytes`, `container_fs_{reads,writes}{,_bytes}_total`
- kube-state-metrics: `kube_pod_info`, `kube_pod_status_ready`, `kube_pod_container_resource_{requests,limits}`, `kube_pod_container_status_restarts_total`
- kubelet: `kubelet_volume_stats_{used,capacity}_bytes`

**SDK로 만들 때 보여주고 싶었던 포인트**
- PromQL을 상수로 한 번만 정의하고 Stat / Timeseries에서 재사용합니다 (`cpuUsage`, `memLimit` 등).
- "Request/Limit 점선" 같은 override를 `referenceLine()` 함수로 만들어 두고 여러 패널에서 씁니다.
- 읽기/쓰기 그래프처럼 같은 모양의 패널은 `readWritePanel()` 하나로 두 개를 만듭니다.
- `gridPos`를 직접 계산하지 않고 `Span`(너비)과 `Height`만 지정하면 SDK가 자동으로 배치합니다.

### 5-2. FinOps — AWS 환산 비용 (마이그레이션)

코드: [`foundation-sdk/go/dashboards/finops/finops.go`](foundation-sdk/go/dashboards/finops/finops.go)
원본: [`59-posted/FinOps — AWS 환산 비용 (OpenCost).json`](../59-posted/)

옮긴 순서:

1. **초안 생성** — SDK에 내장된 converter로 JSON을 Go 코드로 바꿉니다.
   ```bash
   cd foundation-sdk/go
   go run ./tools/convert "../../../59-posted/FinOps — AWS 환산 비용 (OpenCost).json" > /tmp/finops_draft.go.txt
   ```
   결과는 기본값까지 전부 들어간 긴 빌더 체인입니다 (약 470줄). 그대로 쓰기보다는 참고용으로 봅니다.
2. **정리**
   - 패널마다 복붙된 비용 PromQL → `cpuCostByNamespace`, `memCostByNamespace` 상수로
   - `730`, `8760` 같은 매직 넘버 → `hoursPerMonth`, `hoursPerYear`
   - Stat 기본 옵션 → `common.Stat()`, 비용 Stat → `costStat()`
   - `gridPos` 삭제 → `Span`/`Height`만 남기고 자동 배치 (결과 좌표는 원본과 동일)
   - UID(`opencost-homelab`)는 원본과 **동일하게 유지** → 홈랩 대시보드를 Terraform으로 import 할 수 있습니다
3. **비교** — 원본과 생성 결과를 의미 단위로 비교합니다.
   ```bash
   make compare
   # OK: 의미 있는 필드가 모두 같습니다.
   ```
   `tools/compare`는 쿼리(expr/legend/format/instant), 단위, 소수점, 최소/최대, 임계값, 색상, override, transformation, 배치(gridPos), 변수를 비교합니다. 패널 ID, `pluginVersion`, 기본값 표기 차이처럼 화면에 영향이 없는 것은 무시합니다.

---

## 6. 새 대시보드 추가하기

1. `foundation-sdk/go/dashboards/<이름>/<이름>.go`를 만들고 `Build() (dashboard.Dashboard, error)`를 구현합니다. 마지막은 `return common.Build(builder)`로 끝냅니다 (패널 ID 부여, schemaVersion 설정).
2. `foundation-sdk/go/main.go`의 `dashboards` 맵에 한 줄을 추가합니다.
   ```go
   var dashboards = map[string]func() (dashboard.Dashboard, error){
       "service-health":  servicehealth.Build,
       "finops-opencost": finops.Build,
       "my-new-dashboard": mynew.Build, // ← 파일 이름 = Terraform 리소스 키
   }
   ```
3. 새 Grafana 변수(`$foo`)를 썼다면 `main_test.go`의 `grafanaVars`에 치환값을 추가합니다. 추가하지 않으면 테스트가 알려줍니다.
4. `make build && make verify` → `dist/`까지 함께 커밋합니다.

> ⚠️ `dashboards` 맵의 **키(파일 이름)를 바꾸면** Terraform은 기존 대시보드를 삭제하고 새로 만듭니다 (UID가 같으면 생성이 실패할 수 있음). 이름을 바꿔야 한다면 `terraform state mv`를 먼저 하세요.

---

## 7. 검증 — 로컬

홈랩에 올리기 전에 아래 순서로 확인합니다. 한 단계라도 실패하면 다음으로 넘어가지 않습니다.

| 단계 | 명령 | 확인하는 것 |
| --- | --- | --- |
| ① 정적 검사 | `make lint` | `gofmt`, `go vet`, `terraform fmt` |
| ② 테스트 | `make test` | 빌드 에러, UID 중복, 패널 ID 중복/누락, 패널 겹침·24칸 초과, refId 중복, **PromQL 문법** (Prometheus 공식 파서로 파싱) |
| ③ 생성물 최신 여부 | `make check` | 코드로 다시 만든 `dist/`가 커밋된 것과 같은지 |
| ④ 마이그레이션 비교 | `make compare` | FinOps 생성 결과가 원본 JSON과 같은지 |
| ⑤ Grafana 적용 | `make up` → `make smoke` | Grafana 12.0.1이 JSON을 받아들이는지, 다시 읽었을 때 title/패널 수가 같은지 |
| ⑥ Terraform | `make tf-local` | `terraform validate` → `apply` → **재 `plan`에서 변경 0건** (드리프트 없음) |
| ⑦ 눈으로 확인 | http://localhost:3000 | 레이아웃, 변수, 패널 표시 |

①~④는 `make verify` 한 번으로 실행됩니다.

### 실제 데이터로 보기 (선택)

로컬 Grafana의 Prometheus 데이터소스는 기본으로 `http://host.docker.internal:9090`을 바라봅니다. 홈랩 Prometheus를 port-forward 하면 실제 데이터로 화면을 확인할 수 있습니다.

```bash
kubectl -n monitoring port-forward svc/<prometheus-서비스> 9090:9090
make up
```

다른 주소를 쓰려면 `PROMETHEUS_URL=http://<주소>:9090 make up`처럼 넘기면 됩니다. Prometheus에 연결하지 않으면 패널에 빨간 경고 아이콘과 "No data"가 나오는데, 이는 정상입니다. 레이아웃과 변수 확인에는 문제가 없습니다.

### smoke와 tf-local을 같이 쓸 때

`make smoke`는 테스트가 끝나면 올렸던 대시보드를 **삭제**합니다. 그래서 `make smoke` → `make tf-local` 순서로 실행하면 됩니다. 반대 순서로 실행하면 Terraform state와 실제 Grafana 상태가 어긋나므로, 그때는 `make down up`으로 초기화하세요.

> ⚠️ `scripts/smoke.sh`는 같은 UID의 대시보드를 덮어쓰고 삭제합니다. **홈랩 Grafana를 대상으로 실행하지 마세요.**

---

## 8. 검증 — GitHub Actions

워크플로: [`.github/workflows/60-posted.yml`](../.github/workflows/60-posted.yml)
실행 조건: `60-posted/**` 또는 원본 FinOps JSON이 바뀌는 PR / main push, 수동 실행(`workflow_dispatch`)

```
static ──────────────────────────────▶ grafana (matrix: 12.0.1)
 ├ make lint                            ├ services: grafana/grafana:12.0.1
 ├ make test                            ├ make smoke
 ├ make check   (dist 최신 여부)          ├ terraform init / validate
 ├ make compare (원본 비교)               ├ terraform apply
 └ dist/*.json 을 artifact 로 업로드      ├ terraform plan -detailed-exitcode  (변경 있으면 실패)
                                        └ 대시보드가 foundation-sdk 폴더에 있는지 확인
```

- **static**: Grafana 없이 할 수 있는 검증입니다. 로컬 `make verify`와 같습니다.
- **grafana**: 홈랩과 같은 버전의 Grafana를 서비스 컨테이너로 띄워서 **실제 API / Terraform으로 적용**합니다. 재 plan이 비어 있어야 통과하므로 "코드 ↔ Grafana 상태"가 안정적인지까지 확인됩니다.
- Grafana 업그레이드 전에 호환성을 미리 보고 싶으면 `matrix.grafana`에 버전을 추가하면 됩니다 (예: `["12.0.1", "13.0.0"]`).
- 생성된 JSON은 워크플로 실행 화면의 Artifacts(`dashboards`)에서 내려받을 수 있습니다.

> CI는 홈랩에 **배포하지 않습니다.** 홈랩 적용은 아래 절차대로 사람이 plan을 확인하고 진행합니다.

---

## 9. Terraform으로 홈랩에 배포하기

### 구성

```hcl
# main.tf (요약)
resource "grafana_folder" "sdk" { uid = "foundation-sdk", title = "Foundation SDK" }

resource "grafana_dashboard" "this" {
  for_each    = { for f in fileset("../dist", "*.json") : trimsuffix(f, ".json") => ... }
  folder      = grafana_folder.sdk.uid
  config_json = file(each.value)
  overwrite   = var.overwrite   # 기본 false
}
```

`dist/`에 JSON을 추가하면 자동으로 리소스가 늘어납니다. Terraform 코드는 따로 고칠 필요가 없습니다.

### 적용 절차 (체크리스트)

**0) 서비스 어카운트 토큰 준비**
- Grafana → Administration → Users and access → Service accounts → 새로 만들기 (Role: **Editor** 이상, 폴더를 만들므로)
- 토큰(`glsa_...`)은 파일에 적지 말고 환경변수로 넘깁니다.

**1) 변수 파일 준비**
```bash
cd 60-posted/terraform
cp envs/homelab.tfvars.example envs/homelab.tfvars   # .gitignore 에 포함되어 커밋되지 않음
vi envs/homelab.tfvars                               # grafana_url 수정
export TF_VAR_grafana_auth="glsa_xxxxxxxx"
```

**2) 기존 대시보드 가져오기 (FinOps)**

`finops-opencost`는 원본과 UID(`opencost-homelab`)가 같아서, 홈랩에 이미 있는 대시보드와 충돌합니다. `overwrite = false`(기본값)라서 그냥 apply 하면 **실패**합니다. 덮어쓰는 대신 기존 대시보드를 state로 가져옵니다.

```bash
cd 60-posted
make tf-init
terraform -chdir=terraform import -var-file=envs/homelab.tfvars \
  'grafana_dashboard.this["finops-opencost"]' 'opencost-homelab'
```

가져온 뒤 plan을 보면 "원래 폴더 → Foundation SDK 폴더"로 옮겨지고, 코드와 다른 부분(패널 ID 등)이 변경으로 표시됩니다.

**3) plan 확인** — 반드시 사람이 읽고 확인합니다.
```bash
make tf-plan     # terraform/homelab.tfplan 생성
```
확인할 것:
- [ ] `grafana_folder.sdk`: 새로 생성 (1건)
- [ ] `service-health`: 새로 생성
- [ ] `finops-opencost`: import 했다면 **update in-place**여야 합니다. **destroy/create가 보이면 중단**하세요.
- [ ] 예상하지 못한 삭제(`destroy`)가 없는지

**4) 적용**
```bash
make tf-apply    # plan 파일로만 적용 → plan 이후 바뀐 내용이 끼어들 수 없음
```

**5) 확인**
```bash
terraform -chdir=terraform output dashboards
make tf-plan     # 다시 plan → "No changes." 가 나와야 정상
```

### 이후 운영 흐름

```
코드 수정 → make build → make verify → PR (CI 통과) → merge → make tf-plan → 확인 → make tf-apply
```

- Grafana UI에서 대시보드를 직접 고치면 다음 plan에 되돌리는 변경으로 나타납니다. UI에서 실험한 내용은 코드에 반영한 뒤 apply 하세요.
- UI에서 수정한 결과를 코드로 옮길 때는 JSON을 export 해서 `tools/convert`로 초안을 보고, `tools/compare`로 결과를 비교하면 편합니다.

### state 관리

- 기본값은 **로컬 state**(`terraform/terraform.tfstate`, `.gitignore` 처리됨)입니다. 한 사람이 한 PC에서만 apply 한다면 충분합니다.
- 여러 곳(노트북 여러 대, 나중에 CI 자동 배포)에서 apply 할 계획이면 원격 backend로 옮기세요. 홈랩에서는 다음 중 하나가 무난합니다.
  - MinIO 등 S3 호환 스토리지 → `backend "s3"` (`use_path_style = true`, `skip_credentials_validation = true` 등 설정)
  - Kubernetes Secret → `backend "kubernetes"`
- `terraform init`을 처음 실행하면 생기는 `.terraform.lock.hcl`은 **커밋**해서 provider 버전을 고정하세요.

---

## 10. 알아둘 점 / 제약 사항

- **패널 ID가 원본과 다릅니다.** SDK는 패널 ID를 자동으로 붙이지 않아서 `common.Build()`에서 위에서부터 1, 2, 3...을 부여합니다. 원본 FinOps는 1~6, 20~23, 40~42였기 때문에 `viewPanel=<ID>` 형태로 북마크한 링크가 있다면 바뀝니다.
- **schemaVersion**: SDK v0.0.20의 기본값은 42이지만 홈랩(12.0.1)에 맞춰 **41**로 고정했습니다 (`common.SchemaVersion`). Grafana를 올리면 같이 올려주세요.
- **workload 변수**: `pod=~"$workload-.*"`로 파드를 고르기 때문에, 이름이 접두사로 겹치는 워크로드(예: `api`와 `api-gateway`)는 같이 잡힐 수 있습니다. 이럴 때는 Pod 변수에서 골라서 보세요.
- **Disk I/O 패널이 비어 있다면**: containerd 환경에서는 cAdvisor의 `container_fs_*` 메트릭이 일부만 수집되거나 없을 수 있습니다. Prometheus에서 `container_fs_writes_bytes_total`이 조회되는지 먼저 확인하세요.
- **SDK 버전 고정**: Foundation SDK는 아직 `v0.0.x`이고 버전이 오를 때 빌더 API나 기본값이 바뀔 수 있습니다. 올릴 때는 `go get github.com/grafana/grafana-foundation-sdk/go@<버전>` → `make build` → `dist/` diff를 확인하세요.

---

## 11. 참고 자료

- [Grafana Foundation SDK 문서](https://grafana.com/docs/grafana/latest/as-code/observability-as-code/foundation-sdk/)
- [grafana/grafana-foundation-sdk (GitHub)](https://github.com/grafana/grafana-foundation-sdk)
- [Dashboard v2 schema (JSON model)](https://grafana.com/docs/grafana/latest/as-code/observability-as-code/schema-v2/)
- [Dynamic dashboards GA 공지](https://grafana.com/whats-new/2026-04-08-dynamic-dashboards-is-now-generally-available/)
- [Terraform Grafana provider](https://registry.terraform.io/providers/grafana/grafana/latest/docs) · [`grafana_dashboard`](https://registry.terraform.io/providers/grafana/grafana/latest/docs/resources/dashboard) · [`grafana_apps_dashboard_dashboard_v2`](https://registry.terraform.io/providers/grafana/grafana/latest/docs/resources/apps_dashboard_dashboard_v2)
