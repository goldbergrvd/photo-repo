package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"syscall"

	"github.com/gin-gonic/gin"
)

var ROOT_DIR string
var ROOT_STORE string
var IMAGE_DIR string = "images"
var VIDEO_DIR string = "videos"
var IMAGE_EXT = []string{".jpg", ".jpeg", ".png"}
var VIDEO_EXT = []string{".mp4"}

const FILENAME_LEN int = len("20060102150405000")

var SAMSUNG_FILE_RULE *regexp.Regexp

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

	createAllDir(path.Join(ROOT_STORE, IMAGE_DIR))
	createAllDir(path.Join(ROOT_STORE, VIDEO_DIR))

	startServer()
}

func startServer() {
	router := gin.Default()

	router.Static("/static", "./static")

	router.GET("/", func(c *gin.Context) {
		c.File("index.html")
	})

	router.GET("/image/:name", func(c *gin.Context) {
		if len(c.Param("name")) < FILENAME_LEN {
			c.String(http.StatusBadRequest, "Name length at least 17.")
			return
		}
		filename := c.Param("name")[0:FILENAME_LEN]
		ext := filepath.Ext(c.Param("name"))

		if contains(IMAGE_EXT, ext) {
			year := filename[0:4]
			month := filename[4:6]
			day := filename[6:8]
			filePath := path.Join(ROOT_STORE, IMAGE_DIR, year, month, day, filename) + ext
			c.File(filePath)
		}
	})

	router.GET("/images", func(c *gin.Context) {
		result := getAllFile(filepath.Join(ROOT_STORE, IMAGE_DIR))
		sort.Slice(result, func(i, j int) bool {
			return result[i][0:FILENAME_LEN] > result[j][0:FILENAME_LEN]
		})

		c.JSON(http.StatusOK, result)
	})

	router.POST("/upload", func(c *gin.Context) {
		form, _ := c.MultipartForm()
		files := form.File["files"]

		// check file type
		for _, file := range files {
			err := checkFile(file.Filename)
			if err != nil {
				c.String(http.StatusBadRequest, err.Error())
				return
			}
		}

		result := make([]string, 0)
		for _, file := range files {
			fullPath, filePath := getFilePath(file.Filename)
			createAllDir(filepath.Dir(fullPath))
			c.SaveUploadedFile(file, fullPath)
			result = append(result, filePath)
		}
		sort.Slice(result, func(i, j int) bool {
			return result[i][0:FILENAME_LEN] > result[j][0:FILENAME_LEN]
		})
		c.JSON(http.StatusOK, result)
	})

	router.Run()
}

func createAllDir(dirs string) {
	mask := syscall.Umask(0)
	defer syscall.Umask(mask)

	err := os.MkdirAll(dirs, os.ModePerm)
	if err != nil {
		fmt.Println(err)
	}
}

func getFilePath(srcFileName string) (fullPath string, filePath string) {
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
			datetimePath = now.Format("2006/01/02/20060102150405") + now.Format("05.000")[3:] + ext
		}
		filePath = datetimePath[len("2006/01/02/"):]
		fullPath = path.Join(ROOT_STORE, typeDir, datetimePath)

		if _, err := os.Stat(fullPath); errors.Is(err, os.ErrNotExist) {
			return
		}
	}
}

func getAllFile(root string) (result []string) {
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
	if contains(IMAGE_EXT, strings.ToLower(ext)) {
		return
	}
	if contains(VIDEO_EXT, strings.ToLower(ext)) {
		return
	}
	err = errors.New(fmt.Sprintf("Upload file type [%s] incorrect, only accept image and video.", ext))
	return
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
