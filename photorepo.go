package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"photo-repo/middleware"

	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
	"github.com/nfnt/resize"
	"github.com/rwcarlsen/goexif/exif"
)

type DirType = string

var ROOT_DIR string
var ROOT_STORE string
var IMAGE_DIR DirType = "images"
var IMAGE_XS_DIR DirType = "images-xs"
var VIDEO_DIR DirType = "videos"
var IMAGE_EXT = []string{".jpg", ".jpeg", ".png"}
var VIDEO_EXT = []string{".mp4", ".mov"}

const FILE_QUERY_AMOUNT = 50
const FILENAME_LEN int = len("20060102150405000")

const COMPRESS_QUALITY = 20

var SAMSUNG_FILE_RULE *regexp.Regexp

type uploadResult struct {
	SuccessResults []string `json:"successResults"`
	Errors         []string `json:"errorFiles"`
}

func (ur *uploadResult) sort() {
	sort.Slice(ur.SuccessResults, func(i, j int) bool {
		return ur.SuccessResults[i][0:FILENAME_LEN] > ur.SuccessResults[j][0:FILENAME_LEN]
	})
}

func newUploadResult() uploadResult {
	return uploadResult{make([]string, 0), make([]string, 0)}
}

func init() {
	flag.StringVar(&ROOT_DIR, "r", ".", "Server root directory")
	flag.StringVar(&ROOT_STORE, "d", "./files", "Files stored root directory")
	flag.Parse()

	var err error
	SAMSUNG_FILE_RULE, err = regexp.Compile("\\d{8}_\\d{6}\\.[jpg|jpeg|png|JPG|JPEG|PNG|mp4|MP4]")
	if err != nil {
		fmt.Println(err)
	}
}

func main() {
	err := os.Chdir(ROOT_DIR)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = createAllDir(path.Join(ROOT_STORE, IMAGE_DIR))
	if err != nil {
		fmt.Println(err)
		return
	}

	err = createAllDir(path.Join(ROOT_STORE, IMAGE_XS_DIR))
	if err != nil {
		fmt.Println(err)
		return
	}

	err = createAllDir(path.Join(ROOT_STORE, VIDEO_DIR))
	if err != nil {
		fmt.Println(err)
		return
	}

	startServer()
}

func startServer() {
	router := gin.Default()

	router.Use(middleware.Cors())

	router.Static("/static", "./static")

	router.GET("/", func(c *gin.Context) {
		c.File("index.html")
	})

	router.GET("/favicon.ico", func(c *gin.Context) {
		c.File("favicon.ico")
	})

	router.GET("/logo192.jpeg", func(c *gin.Context) {
		c.File("logo192.jpeg")
	})

	router.GET("/logo512.jpeg", func(c *gin.Context) {
		c.File("logo512.jpeg")
	})

	router.GET("/manifest.json", func(c *gin.Context) {
		c.File("manifest.json")
	})

	router.GET("/image/:name", fileHandler(IMAGE_DIR))
	router.GET("/image-xs/:name", fileHandler(IMAGE_XS_DIR))
	router.GET("/video/:name", fileHandler(VIDEO_DIR))

	router.GET("/images", filesHandler(IMAGE_DIR))
	router.GET("/videos", filesHandler(VIDEO_DIR))

	router.POST("/upload", func(c *gin.Context) {
		form, err := c.MultipartForm()
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
		}

		files := form.File["files"]

		// check file type
		for _, file := range files {
			err := checkFile(file.Filename)
			if err != nil {
				c.String(http.StatusBadRequest, err.Error())
				return
			}
		}

		result := newUploadResult()
		for _, file := range files {
			fullPath, fileName := createFilePath(file.Filename, file)
			err := createAllDir(filepath.Dir(fullPath))
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("[%s] fail: %s", file.Filename, err.Error()))
				continue
			}

			err = c.SaveUploadedFile(file, fullPath)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("[%s] fail: %s", file.Filename, err.Error()))
				continue
			}

			if contains(IMAGE_EXT, filepath.Ext(fileName)) {
				err := compressFile(file, fileName)
				if err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("[%s] fail: %s", file.Filename, err.Error()))
					os.Remove(fullPath)
					continue
				}
			}

			result.SuccessResults = append(result.SuccessResults, fileName)
		}
		result.sort()

		c.JSON(http.StatusOK, result)
	})

	router.DELETE("/delete", func(c *gin.Context) {
		var names []string = make([]string, 0)
		var result map[string]bool = make(map[string]bool)

		if err := c.ShouldBind(&names); err != nil {
			c.String(http.StatusBadRequest, err.Error())
		}

		for _, name := range names {
			filePath, xsFilePath, err := getFilePath(name)

			if err == nil {
				err = os.Remove(filePath)
			}

			var errXs error
			if xsFilePath != "" {
				errXs = os.Remove(xsFilePath)
			}

			result[name] = err == nil && errXs == nil
		}

		c.JSON(http.StatusOK, result)
	})

	router.Run()
}

func fileHandler(dirType DirType) func(*gin.Context) {
	return func(c *gin.Context) {
		filePath, xsFilePath, err := getFilePath(c.Param("name"))

		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}

		switch dirType {
		case IMAGE_DIR, VIDEO_DIR:
			c.File(filePath)
		case IMAGE_XS_DIR:
			c.File(xsFilePath)
		}
	}
}

func filesHandler(dirType DirType) func(*gin.Context) {
	return func(c *gin.Context) {
		result := getAllFile(dirType)
		endIndex := len(result)
		fromIndex := endIndex - FILE_QUERY_AMOUNT

		fromName := c.Query("fromName")

		if len(fromName) > 0 {
			if len(strings.TrimSuffix(fromName, filepath.Ext(fromName))) != FILENAME_LEN {
				c.String(http.StatusBadRequest, "Filename length is 17 bit.")
				return
			}
			if err := checkFile(fromName); err != nil {
				c.String(http.StatusBadRequest, err.Error())
				return
			}
			endIndex = sort.SearchStrings(result, fromName)
			fromIndex = endIndex - FILE_QUERY_AMOUNT
		}

		if fromIndex < 0 {
			fromIndex = 0
		}

		result = result[fromIndex:endIndex]

		// reverse
		sort.Slice(result, func(i, j int) bool {
			return result[i][0:FILENAME_LEN] > result[j][0:FILENAME_LEN]
		})

		c.JSON(http.StatusOK, result)
	}
}

func createAllDir(dirs string) (err error) {
	mask := syscall.Umask(0)
	defer syscall.Umask(mask)

	err = os.MkdirAll(dirs, os.ModePerm)
	return
}

func createFilePath(srcFileName string, file *multipart.FileHeader) (fullPath string, fileName string) {
	ext := filepath.Ext(srcFileName)
	typeDir := IMAGE_DIR

	if contains(VIDEO_EXT, ext) {
		typeDir = VIDEO_DIR
	}

	for {
		now := time.Now()
		var datetimePath string
		if SAMSUNG_FILE_RULE.MatchString(srcFileName) {
			year := srcFileName[0:4]
			month := srcFileName[4:6]
			day := srcFileName[6:8]
			time := srcFileName[9:15]
			datetimePath = fmt.Sprintf("%s/%s/%s/%s%s%s%s", year, month, day, year, month, day, time) + now.Format("05.000")[3:] + ext
		} else {
			file, _ := file.Open()
			x, err := exif.Decode(file)
			if err == nil {
				datetime, err := x.DateTime()
				if err == nil {
					datetimeString := datetime.String()
					year := datetimeString[0:4]
					month := datetimeString[5:7]
					day := datetimeString[8:10]
					hour := datetimeString[11:13]
					minute := datetimeString[14:16]
					second := datetimeString[17:19]
					datetimePath = fmt.Sprintf("%s/%s/%s/%s%s%s%s%s%s", year, month, day, year, month, day, hour, minute, second) + now.Format("05.000")[3:] + ext
				}
			}
			if datetimePath == "" {
				datetimePath = now.Format("2006/01/02/20060102150405") + now.Format("05.000")[3:] + ext
			}
		}
		fileName = datetimePath[len("2006/01/02/"):]
		fullPath = path.Join(ROOT_STORE, typeDir, datetimePath)

		if _, err := os.Stat(fullPath); errors.Is(err, os.ErrNotExist) {
			return
		}
	}
}

func getFilePath(name string) (filePath string, xsFilePath string, err error) {
	if len(name) < FILENAME_LEN {
		err = errors.New("Name length at least 17.")
		return
	}
	filename := name[0:FILENAME_LEN]
	ext := filepath.Ext(name)

	year := filename[0:4]
	month := filename[4:6]
	day := filename[6:8]

	if contains(IMAGE_EXT, ext) {
		filePath = path.Join(ROOT_STORE, IMAGE_DIR, year, month, day, filename) + ext
		xsFilePath = path.Join(ROOT_STORE, IMAGE_XS_DIR, year, month, day, filename) + ext
		return
	}

	if contains(VIDEO_EXT, ext) {
		filePath = path.Join(ROOT_STORE, VIDEO_DIR, year, month, day, filename) + ext
		return
	}

	err = fmt.Errorf("File type [%s] is not allowable", ext)
	return
}

func getAllFile(dirType DirType) (result []string) {
	root := filepath.Join(ROOT_STORE, dirType)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() != ".DS_Store" {
			result = append(result, info.Name())
		}
		return nil
	})
	if err != nil {
		fmt.Println(err)
	}
	return
}

func checkFile(fileName string) (err error) {
	ext := filepath.Ext(fileName)
	if contains(IMAGE_EXT, ext) {
		return
	}
	if contains(VIDEO_EXT, ext) {
		return
	}
	err = errors.New(fmt.Sprintf("File type [%s] incorrect, only accept image and video.", ext))
	return
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == strings.ToLower(e) {
			return true
		}
	}
	return false
}

func compressFile(file *multipart.FileHeader, filename string) (err error) {
	srcForImg, err := file.Open()
	if err != nil {
		return err
	}

	srcForExif, err := file.Open()
	if err != nil {
		return err
	}

	img, _, err := image.Decode(srcForImg)
	if err != nil {
		return err
	}

	x, err := exif.Decode(srcForExif)
	if err == nil {
		orientation, err := x.Get(exif.Orientation)
		if err == nil {
			fmt.Printf("exif orientation flag: %s\n", orientation.String())
			switch orientation.String() {
			case "7", "8":
				img = imaging.Rotate90(img)
			case "3", "4":
				img = imaging.Rotate180(img)
			case "5", "6":
				img = imaging.Rotate270(img)
			}
		} else {
			fmt.Println(err)
		}
	}

	img = resize.Resize(500, 0, img, resize.Lanczos3)

	buf := bytes.Buffer{}
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: COMPRESS_QUALITY})
	if err != nil {
		return err
	}

	year := filename[0:4]
	month := filename[4:6]
	day := filename[6:8]
	distFilepath := filepath.Join(ROOT_STORE, IMAGE_XS_DIR, year, month, day)

	err = createAllDir(distFilepath)
	if err != nil {
		return err
	}

	distFile := filepath.Join(distFilepath, filename)
	f, err := os.Create(distFile)
	if err != nil {
		return err
	}
	defer f.Close()

	n, err := f.Write(buf.Bytes())
	if err != nil {
		return err
	}
	fmt.Printf("wrote %d bytes of compress file %s\n", n, filename)
	return
}
