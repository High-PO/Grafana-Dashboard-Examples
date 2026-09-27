output "folder_uid" {
  description = "생성된 폴더 UID"
  value       = grafana_folder.sdk.uid
}

output "dashboards" {
  description = "대시보드별 UID 와 URL"
  value = {
    for key, dash in grafana_dashboard.this :
    key => {
      uid = dash.uid
      url = dash.url
    }
  }
}
