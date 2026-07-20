package util

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

	// MaxMind direct download permalinks (require Account ID + License Key via Basic Auth).
	// See: https://dev.maxmind.com/geoip/updating-databases/
	maxmindDownloadURL = "https://download.maxmind.com/geoip/databases/GeoLite2-City/download?suffix=tar.gz"
	maxmindChecksumURL = "https://download.maxmind.com/geoip/databases/GeoLite2-City/download?suffix=tar.gz.sha256"

	// Legacy URL kept as fallback when Account ID is not configured.
	maxmindLegacyDownloadURL = "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-City&suffix=tar.gz"
	maxmindLegacyChecksumURL = "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-City&suffix=tar.gz.sha256"
)

func AutoDownloadMaxmindDatabase(config g.AutoDownloadConfig) (dbPath string, updated bool, err error) {
	if config.MaxmindLicenseKey == "" {
		config.MaxmindLicenseKey = os.Getenv("MAXMIND_LICENSE_KEY")
	}
	if config.MaxmindAccountID == "" {
		config.MaxmindAccountID = os.Getenv("MAXMIND_ACCOUNT_ID")
	}

	if config.MaxmindLicenseKey == "" {
		return "", false, fmt.Errorf("MAXMIND_LICENSE_KEY is required for auto download")
	}

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
			// Keep raw .tar.gz body; do not auto-decompress Content-Encoding.
			DisableCompression: true,
		},
	}

	logger.Debug("Checking if the database needs updating")

	remoteChecksum, err := fetchChecksum(client, config)
	if err != nil {
		return dbPath, false, err
	}
	remoteChecksum = strings.TrimSpace(remoteChecksum)

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

	logger.Info("Database not found or outdated, downloading")

	if err := downloadAndExtractMMDB(client, config, dbPath); err != nil {
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

func fetchChecksum(client *http.Client, config g.AutoDownloadConfig) (string, error) {
	body, err := doMaxmindGET(client, config, maxmindChecksumURL, maxmindLegacyChecksumURL)
	if err != nil {
		return "", fmt.Errorf("fetch remote checksum: %w", err)
	}
	return string(body), nil
}

func downloadAndExtractMMDB(client *http.Client, config g.AutoDownloadConfig, dbPath string) error {
	req, err := newMaxmindRequest(config, maxmindDownloadURL, maxmindLegacyDownloadURL)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := checkMaxmindStatus(resp); err != nil {
		return err
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gzip: invalid header (response is not a GeoLite2 archive; check MAXMIND_ACCOUNT_ID / MAXMIND_LICENSE_KEY): %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	tmpPath := dbPath + ".tmp"
	found := false

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg || !strings.HasSuffix(hdr.Name, ".mmdb") {
			continue
		}

		out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("write mmdb: %w", err)
		}
		if err := out.Close(); err != nil {
			os.Remove(tmpPath)
			return err
		}
		found = true
		break
	}

	if !found {
		return fmt.Errorf("downloaded archive does not contain a .mmdb file")
	}

	if err := os.Rename(tmpPath, dbPath); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

func doMaxmindGET(client *http.Client, config g.AutoDownloadConfig, modernURL, legacyURL string) ([]byte, error) {
	req, err := newMaxmindRequest(config, modernURL, legacyURL)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := checkMaxmindStatus(resp); err != nil {
		return nil, err
	}

	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

func newMaxmindRequest(config g.AutoDownloadConfig, modernURL, legacyURL string) (*http.Request, error) {
	var (
		rawURL string
		req    *http.Request
		err    error
	)

	if config.MaxmindAccountID != "" {
		rawURL = modernURL
		req, err = http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.SetBasicAuth(config.MaxmindAccountID, config.MaxmindLicenseKey)
	} else {
		u, parseErr := url.Parse(legacyURL)
		if parseErr != nil {
			return nil, parseErr
		}
		q := u.Query()
		q.Set("license_key", config.MaxmindLicenseKey)
		u.RawQuery = q.Encode()
		rawURL = u.String()
		req, err = http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
	}

	// Prevent transport from auto-decompressing; MaxMind body is already a .tar.gz payload.
	req.Header.Set("Accept-Encoding", "identity")
	return req, nil
}

func checkMaxmindStatus(resp *http.Response) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
	msg := strings.TrimSpace(string(snippet))
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("maxmind auth failed (HTTP %d): set valid MAXMIND_ACCOUNT_ID and MAXMIND_LICENSE_KEY; body=%q", resp.StatusCode, msg)
	default:
		return fmt.Errorf("maxmind request failed (HTTP %d): %s", resp.StatusCode, msg)
	}
}
