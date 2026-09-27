# 60-posted: Grafana Foundation SDK(Go) + Terraform 예제
#
# 자주 쓰는 순서:
#   make build      # 코드 → dist/*.json 생성
#   make verify     # 포맷/정적검사 + 테스트 + dist 최신 여부 + 원본 비교 (CI 와 동일)
#   make up smoke   # 로컬 Grafana 띄우고 API 로 업로드 테스트
#   make tf-local   # 로컬 Grafana 에 Terraform apply + 재 plan(드리프트 없음) 확인

SDK_DIR  := foundation-sdk/go
TF_DIR   := terraform
DIST_DIR := dist
ORIGINAL := ../59-posted/FinOps — AWS 환산 비용 (OpenCost).json

GRAFANA_URL  ?= http://localhost:3000
GRAFANA_AUTH ?= admin:admin

.PHONY: help build fmt lint test check compare verify up down smoke \
        tf-init tf-local tf-plan tf-apply clean

help: ## 사용 가능한 타겟 목록
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

# ---------------------------------------------------------------------------
# 생성 / 정적 검증
# ---------------------------------------------------------------------------

build: ## Go 코드로 dist/*.json 생성
	cd $(SDK_DIR) && go run . -out ../../$(DIST_DIR)

fmt: ## gofmt / terraform fmt 적용
	cd $(SDK_DIR) && gofmt -w .
	terraform -chdir=$(TF_DIR) fmt -recursive

lint: ## gofmt / go vet / terraform fmt 검사 (수정하지 않음)
	@out=$$(cd $(SDK_DIR) && gofmt -l .); if [ -n "$$out" ]; then echo "gofmt 필요: $$out"; exit 1; fi
	cd $(SDK_DIR) && go vet ./...
	terraform -chdir=$(TF_DIR) fmt -check -recursive

test: ## 대시보드 검증 테스트 (빌드, UID/패널 ID 중복, 레이아웃 겹침, PromQL 문법)
	cd $(SDK_DIR) && go test ./...

check: build ## 커밋된 dist/ 가 코드와 일치하는지 확인 (코드만 고치고 build 를 안 했으면 실패)
	@git diff --exit-code -- $(DIST_DIR) \
	  || (echo "dist/ 가 코드와 다릅니다. 'make build' 후 결과를 함께 커밋하세요." && exit 1)
	@untracked=$$(git ls-files --others --exclude-standard -- $(DIST_DIR)); \
	  if [ -n "$$untracked" ]; then echo "커밋되지 않은 새 파일: $$untracked"; exit 1; fi

compare: build ## 마이그레이션한 FinOps 대시보드가 원본 JSON 과 같은지 비교
	cd $(SDK_DIR) && go run ./tools/compare "../../$(ORIGINAL)" ../../$(DIST_DIR)/finops-opencost.json

verify: lint test check compare ## CI 의 정적 검증 단계와 동일

# ---------------------------------------------------------------------------
# 로컬 Grafana
# ---------------------------------------------------------------------------

up: ## 로컬 Grafana(12.0.1) 실행 → http://localhost:3000 (admin/admin)
	docker compose up -d --wait

down: ## 로컬 Grafana 종료 및 데이터 삭제
	docker compose down -v

smoke: ## Grafana API 로 dist/*.json 업로드 후 다시 읽어서 확인 (로컬/CI 전용)
	GRAFANA_URL=$(GRAFANA_URL) GRAFANA_AUTH=$(GRAFANA_AUTH) ./scripts/smoke.sh

# ---------------------------------------------------------------------------
# Terraform
# ---------------------------------------------------------------------------

tf-init: ## terraform init
	terraform -chdir=$(TF_DIR) init

tf-local: tf-init ## 로컬 Grafana 에 apply 후, 재 plan 에서 변경이 없는지(드리프트 없음) 확인
	terraform -chdir=$(TF_DIR) validate
	terraform -chdir=$(TF_DIR) apply -auto-approve -var-file=envs/local.tfvars
	terraform -chdir=$(TF_DIR) plan -detailed-exitcode -var-file=envs/local.tfvars

tf-plan: tf-init ## 홈랩 plan (envs/homelab.tfvars + TF_VAR_grafana_auth 필요)
	terraform -chdir=$(TF_DIR) plan -var-file=envs/homelab.tfvars -out=homelab.tfplan

tf-apply: ## 홈랩 apply (tf-plan 으로 만든 계획 파일만 적용)
	terraform -chdir=$(TF_DIR) apply homelab.tfplan

clean: ## 로컬 terraform 상태/캐시 삭제
	rm -rf $(TF_DIR)/.terraform $(TF_DIR)/terraform.tfstate* $(TF_DIR)/*.tfplan
