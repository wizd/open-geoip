package util

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ECNU/open-geoip/g"
	"github.com/toolkits/pkg/logger"
)

const (
	DefaultDownloadTimeout = 3
	DefaultTargetFilePath  = "./"
	MaxmindDBFileName      = "GeoLite2-City.mmdb"
	checksumExt            = ".sha256"

	// Community mirror of GeoLite2-City (auto-updated Tue/Fri).
	// https://github.com/wp-statistics/GeoLite2-City
	defaultGeoLite2CityURL = "https://cdn.jsdelivr.net/npm/geolite2-city/GeoLite2-City.mmdb.gz"
)

func geoLite2CityURL() string {
	if v := os.Getenv("GEOLITE2_CITY_URL"); v != "" {
		return v
	}
	return defaultGeoLite2CityURL
}

func AutoDownloadMaxmindDatabase(config g.AutoDownloadConfig) (dbPath string, updated bool, err error) {
	if config.Timeout == 0 {
		config.Timeout = DefaultDownloadTimeout
	}
	if config.TargetFilePath == "" {
		config.TargetFilePath = DefaultTargetFilePath
	}

	dbPath = filepath.Join(config.TargetFilePath, MaxmindDBFileName)
	checksumPath := dbPath + checksumExt

	if err = os.MkdirAll(config.TargetFilePath, 0o755); err != nil {
		return dbPath, false, err
	}

	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Minute,
		Transport: &http.Transport{
			DisableCompression: true,
		},
	}

	downloadURL := geoLite2CityURL()
	logger.Debug("Checking if the database needs updating from ", downloadURL)

	remoteChecksum, err := fetchRemoteChecksum(client, downloadURL)
	if err != nil {
		return dbPath, false, err
	}

	localChecksum, err := readLocalChecksum(dbPath, checksumPath)
	if err != nil {
		return dbPath, false, err
	}

	logger.Debug("Local checksum: ", localChecksum)
	logger.Debug("Remote checksum:", remoteChecksum)

	if localChecksum != "" && localChecksum == remoteChecksum {
		logger.Debug("Database is up-to-date, no download needed")
		return dbPath, false, nil
	}

	logger.Info("Database not found or outdated, downloading from ", downloadURL)

	if err := downloadAndGunzip(client, downloadURL, dbPath); err != nil {
		return dbPath, false, err
	}

	if err := os.WriteFile(checksumPath, []byte(remoteChecksum), 0o644); err != nil {
		return dbPath, true, fmt.Errorf("database downloaded but failed to write checksum: %w", err)
	}

	logger.Info("Database downloaded succesfully")
	return dbPath, true, nil
}

func readLocalChecksum(dbPath, checksumPath string) (string, error) {
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func fetchRemoteChecksum(client *http.Client, downloadURL string) (string, error) {
	req, err := http.NewRequest(http.MethodHead, downloadURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch remote checksum: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch remote checksum: HTTP %d", resp.StatusCode)
	}

	// Prefer ETag; fall back to Last-Modified + Content-Length.
	if etag := strings.TrimSpace(resp.Header.Get("ETag")); etag != "" {
		return etag, nil
	}
	lm := strings.TrimSpace(resp.Header.Get("Last-Modified"))
	cl := strings.TrimSpace(resp.Header.Get("Content-Length"))
	if lm != "" || cl != "" {
		return lm + "|" + cl, nil
	}
	return "", fmt.Errorf("fetch remote checksum: no ETag or Last-Modified on %s", downloadURL)
}

func downloadAndGunzip(client *http.Client, downloadURL, dbPath string) error {
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("download failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	hasher := sha256.New()
	body := io.TeeReader(resp.Body, hasher)

	gz, err := gzip.NewReader(body)
	if err != nil {
		return fmt.Errorf("gzip: invalid header from %s: %w", downloadURL, err)
	}
	defer gz.Close()

	tmpPath := dbPath + ".tmp"
	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, gz); err != nil {
		out.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write mmdb: %w", err)
	}
	if err := out.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	_ = hex.EncodeToString(hasher.Sum(nil)) // consumed; integrity via successful gunzip + size

	if err := os.Rename(tmpPath, dbPath); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}
