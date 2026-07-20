package cron

import (
	"time"

	"github.com/ECNU/open-geoip/g"
	"github.com/ECNU/open-geoip/models"
	"github.com/ECNU/open-geoip/util"
	"github.com/toolkits/pkg/logger"
)

const (
	DefaultInterval = 24
)

func SyncMaxmindDatabase() {
	interval := g.Config().AutoDownload.Interval
	if interval == 0 {
		interval = DefaultInterval
	}
	t := time.NewTicker(time.Hour * time.Duration(interval))
	defer t.Stop()

	for {
		dbPath, updated, err := util.AutoDownloadMaxmindDatabase(g.Config().AutoDownload)
		if err != nil {
			logger.Errorf("sync maxmind database failed %s", err)
		} else if updated {
			if err := models.ReloadMaxMindReader(dbPath); err != nil {
				logger.Errorf("reload maxmind database failed %s", err)
			} else {
				logger.Info("maxmind database reloaded successfully")
			}
		} else {
			logger.Debug("maxmind database sync successed, no update needed")
		}
		<-t.C
	}
}
