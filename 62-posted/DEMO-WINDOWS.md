# Windows 시연 가이드 (PowerShell, 한 단계씩)

`tasks.py`를 쓰지 않고 **도구를 하나씩 직접 실행**하는 순서입니다. 발표 시연용으로, 단계마다 화면에 무엇이 보이는지와 할 말을 적어두었습니다.

- 환경: Windows 10/11, **PowerShell 7**, Kiro(또는 VS Code)의 터미널
- 작업 위치: `...\Grafana-Dashboard-Examples\62-posted`

---

## 0. 미리 설치 (발표 전에)

```powershell
winget install Microsoft.PowerShell      # PowerShell 7
winget install Python.Python.3.12        # 3.11 이상 필요
winget install GoLang.Go                 # 1.24 이상
winget install Docker.DockerDesktop      # 9단계(Grafana)에서만 필요
winget install Git.Git
```

`winget`을 찾을 수 없다고 나오면 (App Installer가 없는 PC) 아래 둘 중 하나로 하세요.

- **winget 살리기:** Microsoft Store에서 **"앱 설치 관리자(App Installer)"**를 설치하거나 업데이트합니다. 또는 PowerShell에서 아래를 실행합니다.
  ```powershell
  Add-AppxPackage -RegisterByFamilyName -MainPackage Microsoft.DesktopAppInstaller_8wekyb3d8bbwe
  ```
- **Chocolatey가 있다면:** **관리자** PowerShell에서 실행합니다.
  ```powershell
  choco install -y powershell-core python312 golang git docker-desktop
  ```
- **설치 파일 직접 받기:**

  | 도구 | 다운로드 | 설치할 때 |
  | --- | --- | --- |
  | PowerShell 7 | https://aka.ms/powershell-release?tag=stable (`...win-x64.msi`) | 기본값 |
  | Python 3.12 | https://www.python.org/downloads/windows/ | **"Add python.exe to PATH"**, **"py launcher"** 체크 |
  | Go | https://go.dev/dl/ (`...windows-amd64.msi`) | 기본값 |
  | Git | https://git-scm.com/download/win | 기본값 |
  | Docker Desktop | https://www.docker.com/products/docker-desktop/ | WSL 2 사용, 설치 후 재부팅 |

설치 후 **터미널을 새로 열고** 확인합니다.

```powershell
$PSVersionTable.PSVersion   # 7.x
py -3 --version             # Python 3.11 이상
go version                  # go1.24 이상
docker version              # Docker Desktop 실행 중이어야 함
```

> 발표장 네트워크가 불안하면 1~3단계(clone, pip install, go mod download)는 미리 해두세요.

---

## 1. 터미널 준비

```powershell
# 한글이 깨지지 않도록 콘솔 출력을 UTF-8 로
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$env:PYTHONUTF8 = "1"

# 가상환경 activate 스크립트 실행 허용 (이 터미널 창에서만)
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
```

---

## 2. 레포 받기

`C:\WINDOWS\system32`(관리자 터미널의 기본 위치)에서 받지 말고, 내 폴더로 옮긴 뒤 받습니다.

```powershell
cd $HOME\Documents
git clone https://github.com/High-PO/Grafana-Dashboard-Examples.git
cd Grafana-Dashboard-Examples\62-posted
```

Kiro에서는 **File → Open Folder → `62-posted`** 를 연 뒤 `` Ctrl+` ``로 터미널을 엽니다. 1단계 명령은 새 터미널마다 다시 실행해야 합니다.

---

## 3. Python 가상환경 + 라이브러리 설치

```powershell
py -3 -m venv .venv
.\.venv\Scripts\Activate.ps1          # 프롬프트 앞에 (.venv) 가 붙으면 성공
```

### 🎤 시연 포인트: 버전을 고정하지 않으면? (설치 **전에** 실행해야 보입니다)

```powershell
pip install --dry-run grafana-foundation-sdk
```
```
Would install grafana_foundation_sdk-1769699998!10.1.0
```
> 최신은 0.0.20인데 **Grafana 10.1용 옛 빌드**가 깔립니다. PyPI에 epoch(`1769699998!`)가 붙은 버전이 있어서, 버전 비교에서 항상 이겨버리기 때문입니다.

그래서 버전을 고정해서 설치합니다.

```powershell
pip install -r requirements.txt       # grafanalib==0.7.1, grafana-foundation-sdk==0.0.20
pip list | Select-String grafana
```
```
grafana_foundation_sdk 0.0.20
grafanalib             0.7.1
```

---

## 4. ① grafanalib으로 대시보드 만들기

코드: `grafanalib\node_overview.dashboard.py`. 파일 이름이 `*.dashboard.py`이고 `dashboard` 변수를 정의하는 게 grafanalib의 관례입니다.

```powershell
generate-dashboard -o dist\grafanalib.json grafanalib\node_overview.dashboard.py
```

결과 확인:

```powershell
(Get-Item dist\grafanalib.json).Length          # 약 28KB
Get-Content dist\grafanalib.json | Select-String '"value": "null"'
```

> 🎤 첫 임계값이 `"value": "null"` — 숫자 null이 아니라 **문자열** "null"입니다. 8단계에서 이게 문제가 됩니다.

---

## 5. ② Foundation SDK (Python)

코드: `foundation_sdk_python\node_overview.py`

```powershell
python foundation_sdk_python\node_overview.py -o dist\fsdk-python.json
```
```
generated dist\fsdk-python.json
```

```powershell
(Get-Item dist\fsdk-python.json).Length         # 약 14KB (grafanalib 의 절반)
```

> 🎤 같은 대시보드인데 크기가 절반입니다. grafanalib은 레거시 필드와 기본값까지 전부 출력하고, SDK는 지정한 것만 출력합니다.

---

## 6. ③ Foundation SDK (Go)

코드: `foundation_sdk_go\main.go`

```powershell
cd foundation_sdk_go
go run . -o ..\dist\fsdk-go.json
cd ..
```
```
generated ..\dist\fsdk-go.json
```

> 처음 실행하면 SDK를 내려받느라 시간이 걸립니다. 발표 전에 `go mod download`를 해두세요.

---

## 7. 세 결과 비교

### 7-1. SDK Python vs SDK Go — 완전히 같은가?

```powershell
fc.exe dist\fsdk-python.json dist\fsdk-go.json
```
차이가 없다는 메시지가 나옵니다. (영문 Windows: `FC: no differences encountered`)

> 🎤 언어가 달라도 같은 SDK라서 **JSON이 한 바이트도 다르지 않습니다.**
> (단, enum 옵션을 지정하지 않으면 Python은 `"auto"`, Go는 `""`를 출력해서 달라집니다. 그래서 코드에서 명시했습니다.)

### 7-2. grafanalib vs SDK — 같은 대시보드인가?

두 파일은 모양이 너무 달라서 `fc`로는 비교할 수 없습니다. 그래서 화면/동작에 영향을 주는 값(쿼리, 단위, 임계값, 배치, 변수, 옵션)만 비교하는 도구를 씁니다.

```powershell
python tools\compare.py dist\grafanalib.json dist\fsdk-python.json
python tools\compare.py dist\grafanalib.json dist\fsdk-go.json
```
```
OK: 의미 있는 필드가 모두 같습니다.  (dist\grafanalib.json == dist\fsdk-python.json)
OK: 의미 있는 필드가 모두 같습니다.  (dist\grafanalib.json == dist\fsdk-go.json)
```

### 7-3. (선택) 일부러 틀려보기

`foundation_sdk_python\node_overview.py`에서 `step(0.7, "orange")`를 `step(0.75, "orange")`로 바꾸고 다시 생성 → 비교합니다.

```powershell
python foundation_sdk_python\node_overview.py -o dist\fsdk-python.json
python tools\compare.py dist\grafanalib.json dist\fsdk-python.json
```
```
2개 차이 발견 (- dist\grafanalib.json / + dist\fsdk-python.json):
  panels.[stat] CPU 사용률.defaults.thresholds.steps[1].value
    - 0.7
    + 0.75
  panels.[stat] Memory 사용률.defaults.thresholds.steps[1].value
    - 0.7
    + 0.75
```

> 🎤 마이그레이션하다가 값을 하나 잘못 옮기면 이렇게 바로 잡힙니다. 확인이 끝나면 원래대로 되돌리세요 (`git checkout foundation_sdk_python`).

---

## 8. 마이그레이션 시연: grafanalib → Foundation SDK Go (자동 변환)

Foundation SDK(Go)에는 **JSON → Go 코드 변환기**가 있습니다. grafanalib이 만든 JSON을 넣어봅니다.

### 8-1. 그냥 넣으면? → 실패

```powershell
cd foundation_sdk_go
go run ./cmd/convert ..\dist\grafanalib.json
```
```
error: json: cannot unmarshal string into Go struct field Dashboard.panels.defaults.thresholds.steps.value of type float64
exit status 1
```

> 🎤 4단계에서 본 `"value": "null"`(문자열) 때문입니다.

### 8-2. 정리하고 다시 넣으면? → 성공

```powershell
cd ..
python tools\normalize_grafanalib.py dist\grafanalib.json dist\grafanalib.normalized.json

# 정리해도 같은 대시보드인지 확인
python tools\compare.py dist\grafanalib.json dist\grafanalib.normalized.json

cd foundation_sdk_go
go run ./cmd/convert -o ..\dist\grafanalib-to-go.draft.txt ..\dist\grafanalib.normalized.json
cd ..
notepad dist\grafanalib-to-go.draft.txt   # 또는 Kiro 파일 탐색기에서 열기
```

> 🎤 약 370줄짜리 Go 코드 초안이 나옵니다. 기본값까지 다 들어 있어서 그대로 쓰기는 어렵고, 이걸 보면서 정리한 결과가 `foundation_sdk_go\main.go`(227줄)입니다.
> Python SDK에는 이런 변환기가 없습니다. 하지만 메서드 이름이 `WithTarget` ↔ `with_target`처럼 1:1 대응해서, **Go 초안을 보고 Python으로 옮기면** 됩니다.

---

## 9. Grafana에서 세 대시보드 나란히 보기

### 9-1. Grafana 띄우기

```powershell
docker compose up -d --wait
start http://localhost:3000                # admin / admin
```

### 9-2. Import (UI)

**Dashboards → New → Import → Upload dashboard JSON file**에서 세 파일을 차례로 올립니다. 세 파일은 UID(`node-overview`)가 같으므로, 올릴 때마다 **Name과 UID를 바꿔서** 저장합니다.

| 파일 | Name | UID |
| --- | --- | --- |
| `dist\grafanalib.json` | Node Overview (grafanalib) | `node-overview-grafanalib` |
| `dist\fsdk-python.json` | Node Overview (SDK Python) | `node-overview-py` |
| `dist\fsdk-go.json` | Node Overview (SDK Go) | `node-overview-go` |

> 🎤 세 대시보드를 브라우저 탭으로 나란히 띄우면 화면이 똑같습니다.
> 홈랩 Prometheus / Loki를 연결하지 않았다면 패널에 빨간 경고와 "No data"가 나오는데, 이는 정상입니다.

### 9-3. (선택) 실제 데이터로 보기

```powershell
docker compose down
kubectl -n monitoring port-forward svc/<prometheus 서비스> 9090:9090   # 별도 터미널
kubectl -n monitoring port-forward svc/<loki 서비스> 3100:3100         # 별도 터미널
docker compose up -d --wait
```

### 9-4. 정리

```powershell
docker compose down -v
```

---

## 문제가 생기면

| 증상 | 해결 |
| --- | --- |
| `Activate.ps1 ... 이 시스템에서 스크립트를 실행할 수 없으므로` | 1단계의 `Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass` |
| `py` 명령이 없음 | Python 설치 시 "py launcher" 체크, 또는 `python -m venv .venv` |
| 한글이 `???`나 깨진 글자로 보임 | 1단계의 `[Console]::OutputEncoding = ...UTF8` |
| `generate-dashboard`를 찾을 수 없음 | 가상환경이 activate 됐는지 확인 (프롬프트에 `(.venv)`) |
| `go: ... toolchain` 다운로드 메시지 | Go 1.24 이상이면 무시해도 됨 |
| `docker compose up` 실패 | Docker Desktop이 실행 중인지 확인 |
| `>`로 저장한 JSON을 Python이 못 읽음 | `>` 대신 `-o 파일` 옵션 사용 (PowerShell 5.1은 UTF-16으로 저장함) |

## 한 번에 확인하고 싶을 때

시연이 아니라 "전체가 정상인지"만 빠르게 보려면 `python tasks.py check`가 4~8단계를 한 번에 실행합니다.
