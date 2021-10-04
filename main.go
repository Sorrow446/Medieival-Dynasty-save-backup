package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

var r = strings.NewReplacer(" ", "_", ":", "_")

func gameRunning() (bool, error) {
	const procName = "Medieval_Dynasty-Win64-Shipping.exe"
	h, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, err
	}
	proc := windows.ProcessEntry32{Size: 568}
	for {
		err := windows.Process32Next(h, &proc)
		if err != nil {
			if err == syscall.ERROR_NO_MORE_FILES {
				break
			} else {
				return false, err
			}
		}
		runningProcName := windows.UTF16ToString(proc.ExeFile[:])
		if runningProcName == procName {
			return true, nil
		}
	}
	return false, nil
}

func genZipFname() string {
	timestamp := time.Now().Format("Mon Jan 2 3:04PM 2006")
	finalTimestamp := strings.Replace(timestamp, " ", "_", -1)
	filename := "md_save_backup_(" + finalTimestamp + ").zip"
	filename = r.Replace(filename)
	return filename
}

func makeZip(filePaths []string, outPath string) error {
	newZipFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer newZipFile.Close()
	zipWriter := zip.NewWriter(newZipFile)
	defer zipWriter.Close()
	for _, filePath := range filePaths {
		err := addFileToZip(zipWriter, filePath)
		if err != nil {
			return err
		}
	}
	return nil
}

func addFileToZip(zipWriter *zip.Writer, filename string) error {
	fileToZip, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fileToZip.Close()
	info, err := fileToZip.Stat()
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.Base(filename)
	header.Method = zip.Deflate
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, fileToZip)
	return err
}

func populatePaths(savePath string) ([]string, error) {
	var filePaths []string
	files, err := ioutil.ReadDir(savePath)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if !f.IsDir() {
			filePath := filepath.Join(savePath, f.Name())
			filePaths = append(filePaths, filePath)
		}
	}
	return filePaths, nil
}

func readConfig() (*Config, error) {
	data, err := ioutil.ReadFile("config.json")
	if err != nil {
		return nil, err
	}
	var obj Config
	err = json.Unmarshal(data, &obj)
	if err != nil {
		return nil, err
	}
	return &obj, nil
}

func parseConfig() (*Config, error) {
	cfg, err := readConfig()
	if err != nil {
		return nil, err
	}
	if cfg.SavePath == "" {
		return nil, errors.New("Save path is empty.")
	} else if !(cfg.Interval >= 5 && cfg.Interval <= 60) {
		return nil, errors.New("Interval must be between 5 and 60.")
	}
	return cfg, nil
}

func makeDir(path string) error {
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return err
	}
	return nil
}

func getScriptDir() (string, error) {
	var (
		ok    bool
		err   error
		fname string
	)
	if filepath.IsAbs(os.Args[0]) {
		_, fname, _, ok = runtime.Caller(0)
		if !ok {
			return "", errors.New("Failed to get script filename.")
		}
	} else {
		fname, err = os.Executable()
		if err != nil {
			return "", err
		}
	}
	scriptDir := filepath.Dir(fname)
	return scriptDir, nil
}

func init() {
	scriptDir, err := getScriptDir()
	if err != nil {
		panic(err)
	}
	err = os.Chdir(scriptDir)
	if err != nil {
		panic(err)
	}
}

func printInfo(interval int, outPath string) error {
	var err error
	if !filepath.IsAbs(outPath) {
		outPath, err = filepath.Abs(outPath)
		if err != nil {
			return err
		}
	}
	fmt.Printf(
		"Saves will be backed up every %d minutes to \"%s\".\n", interval, outPath,
	)
	return nil
}

func main() {
	cfg, err := parseConfig()
	if err != nil {
		panic(err)
	}
	var (
		outPath  = cfg.OutPath
		savePath = cfg.SavePath
		interval = cfg.Interval
	)
	if outPath != "" {
		err = makeDir(outPath)
		if err != nil {
			panic(err)
		}
	} else {
		outPath, err = os.Getwd()
		if err != nil {
			panic(err)
		}
	}
	waitTime := time.Duration(interval) * time.Minute
	err = printInfo(interval, outPath)
	if err != nil {
		panic(err)
	}
	for {
		time.Sleep(waitTime)
		running, err := gameRunning()
		if err != nil {
			panic(err)
		}
		if !running {
			fmt.Println("Game isn't running, skipped backup.")
			continue
		}
		filePaths, err := populatePaths(savePath)
		if err != nil {
			panic(err)
		}
		zipFname := genZipFname()
		zipPath := filepath.Join(outPath, zipFname)
		err = makeZip(filePaths, zipPath)
		if err != nil {
			panic(err)
		}
		fmt.Println(zipFname)
	}
}
