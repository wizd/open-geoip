package util

import (
	"os"
	"path/filepath"
	"time"

	"github.com/ECNU/open-geoip/g"
	geoip "github.com/pieterclaerhout/go-geoip/v2"
	"github.com/toolkits/pkg/logger"
)

const (
	DefaultDownloadTimeout = 3
	DefaultTargetFilePath  = "./"
	MaxmindDBFileName      = "GeoLite2-City.mmdb"
)

func AutoDownloadMaxmindDatabase(config g.AutoDownloadConfig) (dbPath string, updated bool, err error) {
	if config.MaxmindLicenseKey == "" {
		config.MaxmindLicenseKey = os.Getenv("MAXMIND_LICENSE_KEY")
	}

	if config.Timeout == 0 {
		config.Timeout = DefaultDownloadTimeout
	}

	if config.TargetFilePath == "" {
		config.TargetFilePath = DefaultTargetFilePath
	}

	dbPath = filepath.Join(config.TargetFilePath, MaxmindDBFileName)

	if err = os.MkdirAll(config.TargetFilePath, 0o755); err != nil {
		return dbPath, false, err
	}

	downloader := geoip.NewDatabaseDownloader(config.MaxmindLicenseKey, dbPath, time.Duration(config.Timeout)*time.Minute)

	logger.Debug("Checking if the database needs updating")

	localChecksum, err := downloader.LocalChecksum()
	if err != nil {
		return dbPath, false, err
	}

	remoteChecksum, err := downloader.RemoteChecksum()
	if err != nil {
		return dbPath, false, err
	}

	logger.Debug("Local checksum: ", localChecksum)
	logger.Debug("Remote checksum:", remoteChecksum)

	shouldDownload, err := downloader.ShouldDownload()
	if err != nil {
		return dbPath, false, err
	}

	if !shouldDownload {
		logger.Debug("Database is up-to-date, no download needed")
		return dbPath, false, nil
	}

	logger.Info("Database not found or outdated, downloading")

	if err := downloader.Download(); err != nil {
		return dbPath, false, err
	}

	logger.Info("Database downloaded succesfully")
	return dbPath, true, nil
}
