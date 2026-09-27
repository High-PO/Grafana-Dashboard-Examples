locals {
  # dist/*.json 을 모두 읽어서 { "service-health" = "<경로>" } 형태로 만듭니다.
  # 키(파일 이름)가 리소스 주소가 되므로, 파일 이름을 바꾸면 Terraform 은 삭제 후 재생성으로 인식합니다.
  dashboards = {
    for file in fileset(var.dashboards_dir, "*.json") :
    trimsuffix(file, ".json") => "${var.dashboards_dir}/${file}"
  }
}

resource "grafana_folder" "sdk" {
  uid   = var.folder_uid
  title = var.folder_title
}

resource "grafana_dashboard" "this" {
  for_each = local.dashboards

  folder      = grafana_folder.sdk.uid
  config_json = file(each.value)
  overwrite   = var.overwrite
  message     = "Managed by Terraform (60-posted/terraform)"
}
