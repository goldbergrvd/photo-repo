package album

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var rootDir string
var mutexMap map[string]*sync.RWMutex = make(map[string]*sync.RWMutex)

const DELETE_FILE_PREFIX = "_"

type Album struct {
	Id        string   `json:"id"`
	Name      string   `json:"name"`
	PhotoList []string `json:"photoList"`
}

func SetRootDir(dir string) {
	rootDir = dir

	files, err := ioutil.ReadDir(rootDir)
	if err != nil {
		panic(err)
	}

	for _, f := range files {
		if !f.IsDir() &&
			f.Name() != ".DS_Store" &&
			!strings.HasPrefix(f.Name(), DELETE_FILE_PREFIX) {
			mutexMap[f.Name()] = new(sync.RWMutex)
		}
	}
}

func CreateAlbum(album Album) (result Album, err error) {
	if album.Name == "" {
		return result, fmt.Errorf("Album name cannot empty!")
	}

	id := strconv.FormatInt(time.Now().UnixMilli(), 10)

	if _, ok := mutexMap[id]; ok {
		return result, fmt.Errorf("Id %s is exists!", id)
	}

	file, err := os.Create(filepath.Join(rootDir, id))
	if err != nil {
		return
	}
	defer file.Close()

	result = Album{id, album.Name, album.PhotoList}
	if result.PhotoList == nil {
		result.PhotoList = make([]string, 0)
	}

	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return
	}

	_, err = file.Write(b)
	if err != nil {
		return
	}

	mutexMap[id] = new(sync.RWMutex)

	return result, nil
}

func DeleteAlbum(id string) error {
	if _, ok := mutexMap[id]; !ok {
		return fmt.Errorf("Id %s is not exists!", id)
	}

	mutex := mutexMap[id]
	mutex.Lock()
	defer mutex.Unlock()

	fileName := filepath.Join(rootDir, id)
	if _, err := os.Stat(fileName); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("Id %s is not exists!", id)
	}

	deleteName := filepath.Join(rootDir, DELETE_FILE_PREFIX+id)
	err := os.Rename(fileName, deleteName)
	if err != nil {
		return err
	}

	delete(mutexMap, id)

	return nil
}

func GetAlbums() (result []Album, err error) {
	result = make([]Album, 0)
	for id := range mutexMap {
		album, e := getAlbum(id)
		if e == nil {
			result = append(result, *album)
		}
	}
	return
}

func AddPhoto(id string, photoList []string) (album *Album, err error) {
	return updateAlbum(id, func(album *Album) {
		photoSet := make(map[string]bool)
		newPhotoList := make([]string, 0)
		for _, name := range album.PhotoList {
			if _, ok := photoSet[name]; !ok {
				photoSet[name] = true
				newPhotoList = append(newPhotoList, name)
			}
		}
		for _, name := range photoList {
			if _, ok := photoSet[name]; !ok {
				photoSet[name] = true
				newPhotoList = append(newPhotoList, name)
			}
		}
		sort.Slice(newPhotoList, func(i, j int) bool {
			return newPhotoList[i] > newPhotoList[j]
		})
		album.PhotoList = newPhotoList
	})
}

func DeletePhoto(id string, photoList []string) (album *Album, err error) {
	return updateAlbum(id, func(album *Album) {
		photoSet := make(map[string]bool)
		newPhotoList := make([]string, 0)
		for _, name := range album.PhotoList {
			if _, ok := photoSet[name]; !ok {
				photoSet[name] = true
			}
		}
		for _, name := range photoList {
			if _, ok := photoSet[name]; ok {
				delete(photoSet, name)
			}
		}
		for k := range photoSet {
			newPhotoList = append(newPhotoList, k)
		}
		sort.Slice(newPhotoList, func(i, j int) bool {
			return newPhotoList[i] > newPhotoList[j]
		})
		album.PhotoList = newPhotoList
	})
}

func ChangeAlbumName(id, name string) (album *Album, err error) {
	if name == "" {
		return nil, fmt.Errorf("Album name cannot empty!")
	}
	return updateAlbum(id, func(album *Album) {
		album.Name = name
	})
}

func getAlbum(id string) (album *Album, err error) {
	if mutex, ok := mutexMap[id]; ok {
		mutex.RLock()
		defer mutex.RUnlock()
	} else {
		return nil, fmt.Errorf("Id %s is not exists!", id)
	}

	fileName := filepath.Join(rootDir, id)
	if _, err := os.Stat(fileName); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("Id %s is not exists!", id)
	}

	file, err := os.OpenFile(fileName, os.O_RDONLY, 0444)
	if err != nil {
		return
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return
	}

	err = json.Unmarshal(b, &album)

	return
}

func updateAlbum(id string, updator func(album *Album)) (album *Album, err error) {
	if mutex, ok := mutexMap[id]; ok {
		mutex.Lock()
		defer mutex.Unlock()
	} else {
		return nil, fmt.Errorf("Id %s is not exists!", id)
	}

	fileName := filepath.Join(rootDir, id)
	if _, err := os.Stat(fileName); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("Id %s is not exists!", id)
	}

	rf, err := os.Open(fileName)
	if err != nil {
		return
	}

	b, err := io.ReadAll(rf)
	rf.Close()
	if err != nil {
		return
	}

	err = json.Unmarshal(b, &album)
	if err != nil {
		return
	}

	updator(album)

	b, err = json.MarshalIndent(album, "", "  ")
	if err != nil {
		return nil, err
	}

	wf, err := os.OpenFile(fileName, os.O_RDWR|os.O_TRUNC, 0666)
	if err != nil {
		return nil, err
	}
	defer wf.Close()

	_, err = io.WriteString(wf, string(b))
	if err != nil {
		return nil, err
	}

	return album, nil
}
