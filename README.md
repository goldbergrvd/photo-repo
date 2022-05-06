# photo-repo

提供一個相片儲存與整理的後端 http server。
 - 上傳相片與影片
 - 取得相片與影片
 - 建立相簿

## API
| Method | url | parameter | resposne | comment |
| ------ | ------ | ------ | ------ | ------ |
| GET | /static/* | | static file | |
| GET | /images | query: ?fromName=name&amount=50 | [...names] | amount參數決定回傳數量，預設50 |
| GET | /image/:name | | img file | |
| GET | /image-xs/:name | | compressed img file | |
| GET | /videos | query: ?fromName=name | [...names] | |
| GET | /video/:name | | video file | |
| POST | /upload | multipart/form-data: files | resp code 200: <br/>{<br/>successResults: [...names]<br/>errorFiles: [...errors]<br/>}<br/><br/>resp code 400:<br/>message | 如果上傳批次中有發生處理異常的檔案，會將原因放在errorFiles，不會影響到成功上傳的 |
| DELETE | /delete | application/json: [...names] | { "name": isDeleted } | |

## 建置
`go build photorepo.go`

## 執行
`photorepo.exe -r <server root directory> -d <file stored directory>`

## TODO
 - <del>圖片壓縮至100KB以下</del> 目前壓縮後寬度500，大小約為18~23KB
 - 相簿功能