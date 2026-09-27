terraform {
  required_version = ">= 1.5.0"

  required_providers {
    grafana = {
      source  = "grafana/grafana"
      version = "~> 4.46"
    }
  }

  # state 는 기본적으로 로컬 파일(terraform.tfstate)에 저장됩니다.
  # 홈랩에서 여러 곳(노트북, CI)에서 apply 할 계획이라면 원격 backend 로 옮기세요.
  # 예) MinIO(S3 호환), Kubernetes Secret backend 등 — README "state 관리" 참고
}
