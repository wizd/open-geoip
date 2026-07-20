package g

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/toolkits/file"
)

/*
GlobalConfig 全局配置
*/
type GlobalConfig struct {
	Logger       LoggerSection      `json:"logger"`
	Redis        RedisConfig        `json:"redis"`
	DB           DBConfig           `json:"db"`
	Campus       CampusConfig       `json:"campus"`
	InternalDB   InternalDBConfig   `json:"internaldb"`
	Source       SourceConfig       `json:"source"`
	AutoDownload AutoDownloadConfig `json:"autoDownload"`
	RateLimit    RateLimitConfig    `json:"rateLimit"`
	Http         HttpConfig         `json:"http"`
	SSO          SSO                `json:"sso"`
	Oauth2       OAuth2             `json:"oauth2"`
}

/*
RedisConfig 全局配置
*/
type RedisConfig struct {
	Dsn          string `json:"dsn"`
	MaxIdle      int    `json:"maxIdle"`
	ConnTimeout  int    `json:"connTimeout"`
	ReadTimeout  int    `json:"readTimeout"`
	WriteTimeout int    `json:"writeTimeout"`
	Password     string `json:"password"`
}

/*
AutoDownloadConfig 自动下载的配置
*/
type AutoDownloadConfig struct {
	Enabled           bool   `json:"enabled"`
	MaxmindAccountID  string `json:"maxmindAccountId"`
	MaxmindLicenseKey string `json:"maxmindLicenseKey"`
	TargetFilePath    string `json:"targetFilePath"`
	Timeout           int    `json:"timeout"`
	Interval          int    `json:"interval"`
}

/*
DBConfig DB 配置
*/
type DBConfig struct {
	Maxmind  string `json:"maxmind"`
	Qqzengip string `json:"qqzengip"`
	Ipdb     string `json:"ipdb"`
}

/*
InternalDBConfi 内部数据库
*/
type InternalDBConfig struct {
	Source  string `json:"source"`
	Auth    bool   `json:"auth"`
	Enabled bool   `json:"enabled"`
	DB      string `json:"db"`
}

/*
RateLimitConfig 限流配置
*/
type RateLimitConfig struct {
	Enabled bool `json:"enabled"`
	Minute  int  `json:"minute"`
	Hour    int  `json:"hour"`
	Day     int  `json:"day"`
}

/*
CampusConfig 园区内网配置
*/
type CampusConfig struct {
	Continent      string   `json:"continent"`      //州
	Country        string   `json:"country"`        //国家
	Province       string   `json:"province"`       //省
	City           string   `json:"city"`           //城市
	District       string   `json:"district"`       //区县(行政区）
	ISP            string   `json:"isp"`            //运营商
	AreaCode       string   `json:"areaCode"`       //行政区划代码（国内）
	CountryEnglish string   `json:"countryEnglish"` //国家英文名
	CountryCode    string   `json:"countryCode"`    //国家英文代码
	Longitude      string   `json:"longitude"`      //经度
	Latitude       string   `json:"latitude"`       //纬度
	IPs            []string `json:"ips"`
}

/*
SourceConfig 数据源
*/
type SourceConfig struct {
	IPv4 string `json:"ipv4"`
	IPv6 string `json:"ipv6"`
}

/*
HttpConfig Http 配置
*/
type HttpConfig struct {
	Listen         string               `json:"listen"`
	TrustProxy     []string             `json:"trustProxy"`
	XAPIKey        string               `json:"x-api-key"`
	CORS           []string             `json:"cors"`
	SessionOptions SessionOptionsConfig `json:"sessionOptions"`
}

/*
SessionOptionsConfig Session 配置
*/
type SessionOptionsConfig struct {
	Path     string `json:"path"`
	Domain   string `json:"domain"`
	MaxAge   int    `json:"maxAge"`
	Secure   bool   `json:"secure"`
	HttpOnly bool   `json:"httpOnly"`
}

// SSO 配置
type SSO struct {
	Enabled    bool   `json:"enabled"`
	AuthExpire int    `json:"authExpire"`
	Type       string `json:"type"`
}

// OAuth2 配置
type OAuth2 struct {
	Enabled         bool     `json:"enabled"`
	DisplayName     string   `json:"displayName"`
	RedirectURL     string   `json:"redirectURL"`
	AuthAddr        string   `json:"authAddr"`
	TokenAddr       string   `json:"tokenAddr"`
	UserInfoAddr    string   `json:"userInfoAddr"`
	LogoutAddr      string   `json:"logoutAddr"`
	ClientId        string   `json:"clientId"`
	ClientSecret    string   `json:"clientSecret"`
	UserinfoIsArray bool     `json:"userinfoIsArray"`
	UserinfoPrefix  string   `json:"userinfoPrefix"`
	Scopes          []string `json:"scopes"`
	Attributes      OauthAttributes
}

type OauthAttributes struct {
	Username string `json:"userName"`
	Nickname string `json:"nickName"`
}

var (
	ConfigFile string
	config     *GlobalConfig
	lock       = new(sync.RWMutex)
)

/*
Config 安全的读取和修改配置
*/
func Config() *GlobalConfig {
	lock.RLock()
	defer lock.RUnlock()
	return config
}

/*
ParseConfig 加载配置
*/
func ParseConfig(cfg string) {
	if cfg == "" {
		log.Fatalln("use -c to specify configuration file")
	}

	if !file.IsExist(cfg) {
		log.Fatalln("config file:", cfg, "is not existent. maybe you need `mv cfg.example.json cfg.json`")
	}

	ConfigFile = cfg

	configContent, err := file.ToTrimString(cfg)
	if err != nil {
		log.Fatalln("read config file:", cfg, "fail:", err)
	}

	var c GlobalConfig
	err = json.Unmarshal([]byte(configContent), &c)
	if err != nil {
		log.Fatalln("parse config file:", cfg, "fail:", err)
	}

	applyEnvOverrides(&c)

	lock.Lock()
	defer lock.Unlock()

	config = &c
}

// applyEnvOverrides overlays Coolify-friendly environment variables onto config.
// Env values take precedence over the config file.
func applyEnvOverrides(c *GlobalConfig) {
	if v := os.Getenv("AUTO_DOWNLOAD_ENABLED"); v != "" {
		c.AutoDownload.Enabled = parseEnvBool(v)
	}
	if v := os.Getenv("AUTO_DOWNLOAD_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.AutoDownload.Interval = n
		}
	}
	if v := os.Getenv("AUTO_DOWNLOAD_TARGET_PATH"); v != "" {
		c.AutoDownload.TargetFilePath = v
	}
	if v := os.Getenv("AUTO_DOWNLOAD_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.AutoDownload.Timeout = n
		}
	}
	// Coolify always injects PORT to match the published container port.
	// Prefer PORT over HTTP_LISTEN so a stale HTTP_LISTEN=0.0.0.0:8080 cannot
	// override PORT=5030 and leave the mapped port with no listener.
	if v := os.Getenv("PORT"); v != "" {
		host := os.Getenv("HOST")
		if host == "" {
			host = "0.0.0.0"
		}
		c.Http.Listen = host + ":" + v
	} else if v := os.Getenv("HTTP_LISTEN"); v != "" {
		c.Http.Listen = v
	}
	if v := os.Getenv("X_API_KEY"); v != "" {
		c.Http.XAPIKey = v
	}
	if v := os.Getenv("HTTP_TRUST_PROXY"); v != "" {
		parts := strings.Split(v, ",")
		proxies := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				proxies = append(proxies, p)
			}
		}
		if len(proxies) > 0 {
			c.Http.TrustProxy = proxies
		}
	}
}

func parseEnvBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
