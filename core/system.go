package core

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type SystemSetup struct {
	app       *App
	CertFile  string
	CacheFile string
	Password  string
	aesCipher *AESCipher
}

func newSystem(app *App) *SystemSetup {
	system := &SystemSetup{
		app:       app,
		aesCipher: NewAESCipher("resd48w2d7er95627d447c490a8f02ff"),
		CertFile:  filepath.Join(app.UserDir, "cert.crt"),
		CacheFile: filepath.Join(app.UserDir, "pass.cache"),
	}
	system.checkPasswordFile()
	return system
}

func (s *SystemSetup) initCert() ([]byte, error) {
	content, err := os.ReadFile(s.CertFile)
	if err == nil {
		return content, nil
	}
	if os.IsNotExist(err) {
		err = os.WriteFile(s.CertFile, s.app.PublicCrt, 0750)
		if err != nil {
			return nil, err
		}
		return s.app.PublicCrt, nil
	} else {
		return nil, err
	}
}

func (s *SystemSetup) SetPassword(password string, isCache bool) {
	s.Password = password
	if isCache {
		encrypted, err := s.aesCipher.Encrypt(password)
		if err == nil {
			err1 := os.WriteFile(s.CacheFile, []byte(encrypted), 0750)
			if err1 != nil {
				fmt.Println("Failed to write password: ", err1.Error())
			}
		} else {
			fmt.Println("Failed to Encrypt password: ", err.Error())
		}
	}
}

func (s *SystemSetup) checkPasswordFile() {
	fileInfo, err := os.Stat(s.CacheFile)
	if err != nil {
		return
	}

	lastModified := fileInfo.ModTime()
	oneMonthAgo := time.Now().AddDate(0, -1, 0)
	if lastModified.Before(oneMonthAgo) {
		os.Remove(s.CacheFile)
		return
	}

	content, err := os.ReadFile(s.CacheFile)
	if err != nil {
		return
	}

	password, err := s.aesCipher.Decrypt(string(content))
	if err != nil {
		return
	}
	s.Password = password
}
