package gotiangetfile

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/disintegration/imaging"
	"github.com/jmoiron/sqlx"
	_ "github.com/go-sql-driver/mysql"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const upload = "/images"
const urlDownload  = "https://tiande.ru/upload";


type Image struct {
	ID   int    `db:"ID"`
	HEIGHT   int    `db:"HEIGHT"`
	WIDTH   int    `db:"WIDTH"`
	FILE_SIZE   int    `db:"FILE_SIZE"`
	MODULE_ID string `db:"MODULE_ID"`
	TIMESTAMP_X string `db:"TIMESTAMP_X"`
	CONTENT_TYPE string `db:"CONTENT_TYPE"`
	SUBDIR string `db:"SUBDIR"`
	FILE_NAME string `db:"FILE_NAME"`
	ORIGINAL_NAME string `db:"ORIGINAL_NAME"`
	DESCRIPTION string `db:"DESCRIPTION"`
	EXTERNAL_ID sql.NullString `db:"EXTERNAL_ID"`
	HANDLER_ID sql.NullString `db:"HANDLER_ID"`
}

type ImageFile struct {
	id 		  			  int
	path, ext, uploadPath string

}

type ImageFileResize struct {
	id 		  			  			  int
	path, ext, uploadPath, resizePath string

}



type DbConnectOpions struct{
	port int
	host, login, pass, database string

}


func DownloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func LoadImage(image Image, ch chan <- ImageFile)  {
	remotePath := fmt.Sprintf("%s/%s", image.SUBDIR, image.FILE_NAME)
	re := regexp.MustCompile(`\.(jpg|jpeg|gif|png|svg)$`)
	match := re.FindStringSubmatch(image.FILE_NAME)
	returnData := ImageFile{}
	if len(match) != 0 {
		hasher := md5.New()
		hasher.Write([]byte(fmt.Sprintf(
			"%s/%s",
			image.FILE_NAME,
			image.SUBDIR)))
		t, _ := time.Parse("2006-01-02 15:04:05", image.TIMESTAMP_X)
		uploadPath := fmt.Sprintf("%s/%s/%s",
			upload,
			t.Format("2006/01/02"),
			hex.EncodeToString(hasher.Sum(nil)))

		filePath := fmt.Sprintf("%s/original.%s", uploadPath, strings.ToLower(match[1]) )



		pwd, err := os.Getwd()
		if err == nil {
			returnDataOk :=  ImageFile{ id: image.ID, path: filePath,
				ext: match[1], uploadPath: uploadPath}


			if _, err := os.Stat(pwd + uploadPath); err == nil {
				returnData = returnDataOk;
			} else if os.IsNotExist(err) {
				os.MkdirAll(pwd + uploadPath, os.ModePerm)
				if err := DownloadFile(pwd + filePath, fmt.Sprintf("%s/%s", urlDownload, remotePath)); err == nil {
					returnData = returnDataOk;
				}
			}
		}
	}

	ch <- returnData;
}

func GetImageChanel(conn *sqlx.DB, ids []int,  chanel chan <- []ImageFile) {
	images := []Image{}
	files := []ImageFile{}
	file := ImageFile{}
	ch := make(chan ImageFile)
	err := conn.Select(
		&images,
		fmt.Sprintf(
			"select * from b_file where ID IN(%s)",
			strings.Trim(strings.Join(strings.Split(fmt.Sprint(ids), " "), ","), "[]")))
	if err != nil {
		panic(err)
	}

	for _, image := range images {
		go LoadImage(image, ch)
	}

	for range images {
		file = <-ch
		files = append(files, file);
	}

	chanel <- files
}

func GetImage(conn *sqlx.DB, ids []int) []ImageFile  {
	ch := make(chan []ImageFile)
	go GetImageChanel(conn, ids, ch)
	return <- ch
}

func GetImageDecode(img ImageFile) (image.Image, error) {
	pwd, err := os.Getwd()
	if err == nil {
		file, err := os.Open(pwd + img.path)
		defer file.Close()
		if err == nil {
			switch img.ext {
			case "jpg", "jpeg":
				return jpeg.Decode(file)
			case "gif":
				return gif.Decode(file)
			case "png":
				return png.Decode(file)


			}
		}
	}


	return nil, errors.New("can't work with 42")
}

func ResizeImage(image ImageFile,  width int, heigh int, ch chan <- ImageFileResize)  {
	pwd, err := os.Getwd()
	returnImage := ImageFileResize{id: image.id, path: image.path,
		ext: image.ext, uploadPath: image.uploadPath}
	if err == nil {
		resizeFileName := fmt.Sprintf("%s%s/resize%dX%d.%s", pwd,
			image.uploadPath, width, heigh, image.ext)
		if image.ext == "svg" {
			returnImage.resizePath = resizeFileName
		} else {
			img, err := GetImageDecode(image)
			if err == nil {
				resizeFile := imaging.Resize(img, width, heigh, imaging.Lanczos)
				err := imaging.Save(resizeFile, resizeFileName);
				if err == nil {
					returnImage.resizePath = resizeFileName
				}

			}
		}
	}
	ch <- returnImage;
}

func ResizeImages(conn *sqlx.DB, ids []int, width int, heigh int)[]ImageFileResize {
	ch := make(chan ImageFileResize)
	files := []ImageFileResize{}
	file := ImageFileResize{}
	for _, image := range GetImage(conn, ids) {
		go ResizeImage(image, width, heigh, ch)
	}

	for range ids {
		file = <-ch
		files = append(files, file);
	}
	return files;
}
