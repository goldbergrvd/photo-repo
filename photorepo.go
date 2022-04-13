package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"syscall"

	"github.com/gin-gonic/gin"
)

var imgsDir string

func init() {
	flag.StringVar(&imgsDir, "d", "./imgs", "images root directory")
	flag.Parse()
}

func main() {
	createImgsDir()
	startServer()
}

func startServer() {
	router := gin.Default()

	router.Static("/static", "./static")

	router.GET("/", func(c *gin.Context) {
		c.File("index.html")
	})

	router.GET("/img/:name", func(c *gin.Context) {
		filePath := path.Join(imgsDir, c.Param("name"))
		c.File(filePath)
	})

	router.GET("/imgs", func(c *gin.Context) {
		files, err := ioutil.ReadDir(imgsDir)
		if err != nil {
			fmt.Println(err)
		}

		result := make([]string, 0)

		for _, file := range files {
			if !file.IsDir() && file.Name() != ".DS_Store" {
				result = append(result, file.Name())
			}
		}

		c.JSON(http.StatusOK, result)
	})

	router.POST("/upload", func(c *gin.Context) {
		form, _ := c.MultipartForm()
		files := form.File["files"]
		result := make([]string, 0)

		for _, file := range files {
			filePath := path.Join(imgsDir, file.Filename)
			c.SaveUploadedFile(file, filePath)
			result = append(result, file.Filename)
		}
		c.JSON(http.StatusOK, result)
	})

	router.Run()
}

func createImgsDir() {
	mask := syscall.Umask(0)
	defer syscall.Umask(mask)

	err := os.MkdirAll(imgsDir, os.ModePerm)
	if err != nil {
		fmt.Println(err)
	}
}
