# photo-repo

提供一個相片儲存與整理的後端 http server。
 - 上傳相片與影片
 - 取得相片與影片
 - 建立相簿

## API
| Method | url | parameter | resposne |
| ------ | ------ | ------ | ------ |
| GET | /static/* | | static file |
| GET | /images | fromName | [...names] |
| GET | /image/:name | | img file |
| GET | /image-xs/:name | | compressed img file |
| GET | /videos | fromName | [...names] |
| GET | /video/:name | | video file |
| POST | /upload | multipart/form-data | [...names] |
| DELETE | /delete | [...names] | { "name": isDeleted } |

## 建置
`go build photorepo.go`

## 執行
`photorepo.exe -r <server root directory> -d <file stored directory>`

## TODO
 - 圖片壓縮至100KB以下
