# photo-repo

提供一個相片儲存與整理的後端 http server。
 - 上傳相片與影片
 - 取得相片與影片
 - 建立相簿

## API
| Method | url | parameter | resposne |
| ------ | ------ | ------ | ------ |
| GET | /images | | [...names] |
| GET | /image/:name | | img file |
| POST | /upload | multipart/form-data | [...names] |

## 建置
`go build photorepo.go`

## 執行
`photorepo.exe -d <image directory>`