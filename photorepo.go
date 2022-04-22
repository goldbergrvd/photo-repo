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
	"syscall"
	"time"

	"syscall"

	"photo-repo/middleware"

	"github.com/gin-gonic/gin"
)

type DirType = string

var ROOT_DIR string
var ROOT_STORE string
var IMAGE_DIR DirType = "images"
var VIDEO_DIR DirType = "videos"
var IMAGE_EXT = []string{".jpg", ".jpeg", ".png"}
var VIDEO_EXT = []string{".mp4"}

const FILE_QUERY_AMOUNT = 50
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

	router.Use(middleware.Cors())

	router.Static("/static", "./static")

	router.GET("/", func(c *gin.Context) {
		c.File("index.html")
	})

	router.GET("/image/:name", fileHandlerFunc)
	router.GET("/video/:name", fileHandlerFunc)

	router.GET("/images", filesHandler(IMAGE_DIR))
	router.GET("/videos", filesHandler(VIDEO_DIR))

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
			fullPath, filePath := createFilePath(file.Filename)
			createAllDir(filepath.Dir(fullPath))
			c.SaveUploadedFile(file, fullPath)
			result = append(result, filePath)
		}
		sort.Slice(result, func(i, j int) bool {
			return result[i][0:FILENAME_LEN] > result[j][0:FILENAME_LEN]
		})
		c.JSON(http.StatusOK, result)
	})

	router.DELETE("/delete", func(c *gin.Context) {
		var names []string = make([]string, 0)
		var result map[string]bool = make(map[string]bool)

		if err := c.ShouldBind(&names); err != nil {
			c.String(http.StatusBadRequest, err.Error())
		}

		for _, name := range names {
			filePath, err := getFilePath(name)

			if err == nil {
				err = os.Remove(filePath)
			}
			result[name] = err == nil
		}

		c.JSON(http.StatusOK, result)
	})

	router.Run()
}

func fileHandlerFunc(c *gin.Context) {
	filePath, err := getFilePath(c.Param("name"))

	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	c.File(filePath)
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

func createAllDir(dirs string) {
	mask := syscall.Umask(0)
	defer syscall.Umask(mask)

	err := os.MkdirAll(dirs, os.ModePerm)
	if err != nil {
		fmt.Println(err)
	}
}

func createFilePath(srcFileName string) (fullPath string, filePath string) {
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

func getFilePath(name string) (filePath string, err error) {
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
	if contains(IMAGE_EXT, strings.ToLower(ext)) {
		return
	}
	if contains(VIDEO_EXT, strings.ToLower(ext)) {
		return
	}
	err = errors.New(fmt.Sprintf("File type [%s] incorrect, only accept image and video.", ext))
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
