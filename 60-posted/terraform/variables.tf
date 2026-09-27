variable "grafana_url" {
  description = "Grafana 주소 (예: http://localhost:3000, https://grafana.homelab.local)"
  type        = string
}

variable "grafana_auth" {
  description = "Grafana 인증 정보. 서비스 어카운트 토큰(glsa_...) 또는 'user:password'. 파일에 적지 말고 TF_VAR_grafana_auth 환경변수로 넘기는 것을 권장합니다."
  type        = string
  sensitive   = true
}

variable "folder_uid" {
  description = "대시보드를 넣을 폴더 UID"
  type        = string
  default     = "foundation-sdk"
}

variable "folder_title" {
  description = "대시보드를 넣을 폴더 이름"
  type        = string
  default     = "Foundation SDK"
}

variable "dashboards_dir" {
  description = "SDK 로 생성한 대시보드 JSON 디렉토리"
  type        = string
  default     = "../dist"
}

variable "overwrite" {
  description = "같은 UID 의 대시보드가 이미 있을 때 덮어쓸지 여부. 기본값 false — 기존 대시보드는 terraform import 로 가져오는 것을 권장합니다."
  type        = bool
  default     = false
}
