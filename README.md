# Open-GeoIP
Open-GeoIP: 简单且高性能的 IP 地址地理信息查询服务

![](https://github.com/ECNU/open-geoip/blob/main/demo.jpg?raw=true)

- [Open-GeoIP](#open-geoip)
	- [安装运行](#安装运行)
		- [二进制直接运行](#二进制直接运行)
		- [systemctl 托管](#systemctl-托管)
		- [Coolify / Docker 部署](#coolify--docker-部署)
		- [数据库自动更新](#数据库自动更新)
			- [maxmind](#maxmind)
		- [编译打包](#编译打包)
		- [定制页面](#定制页面)
	- [配置说明](#配置说明)
		- [环境变量覆盖](#环境变量覆盖)
	- [内部 IP 地理数据库](#内部-ip-地理数据库)
	- [限流方案](#限流方案)
	- [高可用与扩展性](#高可用与扩展性)
	- [API](#api)
		- [myip](#myip)
		- [mylocation](#mylocation)
		- [searchapi](#searchapi)
		- [openapi](#openapi)
	- [benchmark](#benchmark)
	- [鸣谢](#鸣谢)

## 安装运行

### 二进制直接运行
在 [release](https://github.com/ECNU/open-geoip/releases) 中下载最新的 [release] 包，解压后直接运行即可。

注意：`release` 中内置的数据库文件来自于 [ipdb-go](https://github.com/ipipdotnet/ipdb-go) 中的 `city.free.ipdb`，仅供测试使用，不保证数据的准确性。

如应用于生产环境，请获取商用授权，或者[注册](https://www.maxmind.com/en/geolite2/signup) `maxmind` 的账号后，获取免费版的 `GeoLite2-City.mmdb` 数据库文件，并更新配置文件替换数据源为 `maxmind`。

```
tar -zxvf open-geoip-0.1.0-linux-amd64.tar.gz
cd open-geoip/
./control start
```
访问你服务器的 80 端口即可使用。


### systemctl 托管
假定部署在 `/opt/open-geoip` 目录下，如果部署在其他目录修改 `open-geoip.service` 中的 `WorkingDirectory` 和 `ExecStart` 两个字段即可。
```
cp open-geoip.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable open-geoip
systemctl start open-geoip
```

### Coolify / Docker 部署

项目提供 `Dockerfile`、`docker-compose.yml` 与容器默认配置 `cfg.docker.json`，可直接在 [Coolify](https://coolify.io/) 上部署。

#### Coolify 操作步骤

1. 新建资源，选择 **Dockerfile** 或 **Docker Compose**
2. 连接本仓库，构建上下文为仓库根目录
3. **无需 MaxMind License**：镜像构建时已从 [wp-statistics/GeoLite2-City](https://github.com/wp-statistics/GeoLite2-City)（jsDelivr CDN）打包 `GeoLite2-City.mmdb`；运行时也会从同一源自动更新
4. 可选环境变量：

| 环境变量 | 必填 | 说明 |
|----------|------|------|
| `AUTO_DOWNLOAD_ENABLED` | 否 | 默认 `true`，从 CDN 自动更新数据库 |
| `AUTO_DOWNLOAD_INTERVAL` | 否 | 自动更新间隔（小时），默认 `24` |
| `AUTO_DOWNLOAD_TARGET_PATH` | 否 | 数据库目录，默认 `/data/` |
| `AUTO_DOWNLOAD_TIMEOUT` | 否 | 下载超时（分钟），默认 `5` |
| `GEOLITE2_CITY_URL` | 否 | 数据库下载地址，默认 jsDelivr CDN |
| `PORT` | 否 | 服务监听与 compose/Coolify 映射端口，默认 `8080`；**有 `PORT` 时优先**，监听 `$HOST:$PORT`（`HOST` 默认 `0.0.0.0`） |
| `HTTP_LISTEN` | 否 | 完整监听地址（如 `0.0.0.0:9090`）；仅在未设置 `PORT` 时生效 |
| `X_API_KEY` | 否 | OpenAPI 的 `X-API-KEY` |
| `HTTP_TRUST_PROXY` | 否 | 信任的反向代理，逗号分隔；默认信任全部（适配 Coolify/Traefik） |

5. **持久化存储**：将卷挂载到容器内 `/data`；首次启动会把镜像内置库复制进去
6. 对外端口由 `PORT` 控制（默认 `8080`）；Coolify 请保证 **Ports Exposes 与 `PORT` 一致**，不要用过期的 `HTTP_LISTEN=0.0.0.0:8080` 覆盖；健康检查路径为 `/version`
7. **日志**：启动信息与 HTTP 访问日志写到 stdout，可用 `docker logs` / Coolify 日志排查
8. **自动更新**：服务先用本地库 Listen，再在后台按 `AUTO_DOWNLOAD_INTERVAL` 从 CDN 校验并热加载，不阻塞健康检查

#### 本地 Docker Compose

```bash
# 可选：自定义端口，例如 9090
# export PORT=9090
docker compose up -d --build
```

访问 `http://localhost:$PORT`（默认 8080）。镜像已内置数据库；开启自动更新后会在服务就绪后按 `AUTO_DOWNLOAD_INTERVAL` 从 CDN 校验并热加载。

SSO / OAuth / Redis 限流等高级配置仍通过配置文件管理；可在 Coolify 用 File Mount 覆盖 `cfg.docker.json`。

### 数据库自动更新
#### GeoLite2-City（默认）
默认从社区镜像 [wp-statistics/GeoLite2-City](https://github.com/wp-statistics/GeoLite2-City) 更新（CDN：`https://cdn.jsdelivr.net/npm/geolite2-city/GeoLite2-City.mmdb.gz`），**不需要** MaxMind License Key，可避开官方对 CN 账号的 City 库限制。

启用 `autoDownload.enabled`（或环境变量 `AUTO_DOWNLOAD_ENABLED=true`）后，进程在 HTTP 就绪后按 `interval` 定时同步（启动时也会立即尝试一次），下载成功后自动热加载。可通过 `GEOLITE2_CITY_URL` 覆盖下载地址。启动阶段只加载本地库，CDN 不可达时不影响服务启动。


### 编译打包
```
git clone https://github.com/ECNU/open-geoip.git
cd open-geoip/
chmod +x control
./control pack
```

### 定制页面
修改 `templates` 目录下的 `index.html` 即可，相关资源文件在 `assets` 目录下。

## 配置说明

根据 `cfg.json.example` 文件，创建 `cfg.json` 文件，再进一步根据自己的需要修改配置。

```json
{
	"logger": {
		"dir": "logs/",
		"level": "DEBUG",
		"keepHours": 24
	},
	"redis": {
		"dsn": "127.0.0.1:6379",
		"maxIdle": 5,
		"connTimeout": 5,
		"readTimeout": 5,
		"writeTimeout": 5,
		"password": ""
	},
	"internal": {
		"source": "maxmind", 
		"enable": false, 
		"db": "GeoLite2-City.mmdb.test"
	},
	"db": {
		"maxmind": "GeoLite2-City.mmdb",
		"qqzengip": "",
		"ipdb":""
	},
	"source": {
		"ipv4": "maxmind",
		"ipv6": "maxmind"
	},
	"autoDownload":{
		"enabled":false,
		"targetFilePath":"",
		"timeout":3,
		"interval":24
	},
	"rateLimit": {
		"enabled": false,
		"minute": 100,
		"hour": 1000,
		"day": 10000
	},
	"http": {
		"listen": "0.0.0.0:80",
		"trustProxy": ["127.0.0.1", "::1"],
		"cors":["http://localhost"],
		"x-api-key": "this-is-key"
	}
}
```

| 配置项                            | 类型     | 说明                                                                                                                  |
|--------------------------------|--------|---------------------------------------------------------------------------------------------------------------------|
| logger                         | object | 一个包含日志设置的部分                                                                                                         |
| logger.dir                     | string | 存储日志文件的目录                                                                                                           |
| logger.level                   | string | 日志的级别，比如DEBUG, INFO, WARN, 或ERROR                                                                                   |
| logger.keepHours               | number | 保留日志文件的小时数，之后删除                                                                                                     |
| redis | object | redis 配置参数，配合限流策略使用 |
| redis.dsn | string | redis 的连接地址 |
| redis.maxIdle | number | redis 的最大空闲连接数 |
| redis.connTimeout | number | redis 的连接超时时间，单位是 second |
| redis.readTimeout | number | redis 的读取超时时间，单位是 second |
| redis.writeTimeout | number | redis 的写入超时时间，单位是 second |
| redis.password | string | redis 的密码 |
| internal                       | object  | 内部数据库参数                                                                                                             | 
| internal.enabled               | bool   | 开启内部数据库                                                                                                             |
| internal.source                | string | 内部数据库来源                                                                                                             |
| internal.db                    | string | 内部数据库文件路径      
| db                             | object | 一个包含数据库设置的部分                                                                                                        |
| db.maxmind                     | string | MaxMind GeoLite2数据库的文件的路径，如果 autDownload 配置为 true，那么这里的配置不会生效                                                       |
| db.qqzengip                    | string | qqzengip数据库的文件的路径                                                                                                   |
| db.ipdb                        | string | ipip.net数据库的文件的路径                                                                                                   |
| source                         | object | 一个包含IP信息来源设置的部分                                                                                                     |
| source.ipv4                    | string | IPv4信息的来源，可配置为 [maxmind](https://www.maxmind.com)/[qqzengip](https://www.qqzeng.com/)/[ipdb](https://www.ipip.net/) |
| source.ipv6                    | string | IPv6信息的来源，可配置为 [maxmind](https://www.maxmind.com)/[qqzengip](https://www.qqzeng.com/)/[ipdb](https://www.ipip.net/) |
| autoDownload                   | object | 一个包含自动更新数据库的设置的部分                                                                                                   |
| autoDownload.enabled           | bool   | 是否启用自动更新数据库（默认从 wp-statistics CDN 拉取 GeoLite2-City）                                                              |
| autoDownload.targetFilePath    | string | 自动更新数据库的目标文件路径，如果不配置此参数，默认值是 `./`，自动更新数据库会下载到这个目录                                                                   |
| autoDownload.timeout           | number | 自动更新数据库的超时时间，单位是 minute，如果不配置此参数，默认值是 3                                                                             |
| autoDownload.interval          | number | 自动更新数据库的间隔时间，单位是 hour，如果不配置此参数，默认值是24                                                                               |

### 环境变量覆盖

以下环境变量会在加载配置文件后覆盖对应项（适用于 Coolify / Docker / systemd）：

| 环境变量 | 覆盖配置项 |
|----------|------------|
| `AUTO_DOWNLOAD_ENABLED` | `autoDownload.enabled`（`true`/`false`/`1`/`0`） |
| `AUTO_DOWNLOAD_INTERVAL` | `autoDownload.interval` |
| `AUTO_DOWNLOAD_TARGET_PATH` | `autoDownload.targetFilePath` |
| `AUTO_DOWNLOAD_TIMEOUT` | `autoDownload.timeout` |
| `GEOLITE2_CITY_URL` | 数据库下载 URL（默认 jsDelivr CDN） |
| `HTTP_LISTEN` | `http.listen` |
| `PORT` | 未设置 `HTTP_LISTEN` 时，等价于 `http.listen=0.0.0.0:$PORT` |
| `X_API_KEY` | `http.x-api-key` |
| `HTTP_TRUST_PROXY` | `http.trustProxy`（逗号分隔） |
| rateLimit                      | object | 一个包含限流设置的部分                                                                                                         |
| rateLimit.enabled              | bool   | 是否启用限流策略                                                                                                             |
| rateLimit.minute               | number | 每分钟最多访问次, 0 表示不限制数                                                                                                           |
| rateLimit.hour                 | number | 每小时最多访问次, 0 表示不限制数                                                                                                           |
| rateLimit.day                  | number | 每天最多访问次, 0 表示不限制数                                                                                                             |
| http                           | object | 一个包含HTTP服务器设置的部分                                                                                                    |
| http.listen                    | string | HTTP服务器监听的地址和端口                                                                                                     |
| http.trustProxy                | array  | 被信任的代理的IP地址的数组，当服务被发布在反向代理后时必须正确配置，否则无法正确获取到 xff 的地址。                                                               |
| http.cors                      | array  | 允许跨域访问的域名列表,配置内的域名可以跨域访问 `/myip` 和 `/myip/format` 接口                                                                |
| http.x-api-key                 | string | 访问 openapi 接口所需的 API 密钥                                                                                             |                                                                                                     |
## 内部 IP 地理数据库

Open-GeoIP 允许以导入的方式，构建企业内部自己的 IP 地理数据库，以便于查询内部 IP 地址的物理位置。

导入的格式是 `csv`，内容如下所示，项目中已经存在一个 `internal.csv` 的示例文件，可以参考。

| continent | country | province | city | district | isp | areaCode | countryCode | countryEnglish | longitude | latitude | ip_subnet |
| --------- | ------- | -------- | ---- | -------- | --- | -------- | ----------- | -------------- | --------- | -------- | --------- |
|           |         |          |      | 保留     | 回环地址  |          |             |                |           |          | 127.0.0.0/8 |
| 亚洲      | 中国    | 上海     | 上海  | 开源教育   | 企业内网  | 310000   | CN          | China          ||| 10.0.0.0/8 |
| 亚洲      | 中国    | 上海     | 上海  | 开源教育   | 企业内网  ||| CN          || China          ||| 192.168.0.0/16 |
| 亚洲      ||| 中国    || 上海     || 上海  || 开源教育   || 企业内网  ||| 310000   || CN          || China          ||| 172.16.0.0/12 |
| 亚洲      ||| 中国    || 上海     || 上海  || 开源教育   || 企业内网  ||| 310000   || CN          || China          |||| fd00::/8 |


在启动 Open-GeoIP 之前，执行 `-csv` 命令即可导入内部数据库，此时默认会生成一个 `internal.mmdb` 文件。
```
./open-geoip -csv internal.csv
```
在配置文件中，修改 `internal.mmdb` 的相关配置，将其开启即可。

```json
        "internaldb": {
                "source": "maxmind",
                "enabled": true,
                "db": "internal.mmdb"
        }
```


## 限流方案
Open-GeoIP 通过 redis 记录每个IP地址的访问次数，当超过阈值时，对该IP进行限制访问。支持分钟，小时，天 三种颗粒的计数策略，可以通过配置文件中的 ratelimit 的部分进行配置，以下示例表示开启了限流策略，并限制了每分钟最多访问 100 次，每小时最多访问 1000 次，每天最多访问 10000 次。

```json
	"rateLimit": {
		"enabled": true,
		"minute": 100,
		"hour": 1000,
		"day": 10000
	},
```

## 高可用与扩展性
Open-GeoIP 是无状态的，因此可以任意的进行横向扩展并通过负载均衡实现高可用。在启用限流方案时，多个 Open-GeoIP 可以通过共享同一个 Redis 服务实现限流计数的一致性。

## API
### myip

myip 的接口用于返回请求者的 IP 地址，对于一些无浏览器的终端，可以使用这个接口方便的获取自身的IP地址信息（特别是 nat 后的）。

它也可以被配置了 CORS 的网站通过前端调用

提供了简单字符串与 json 格式化两种风格接口。

```
# curl http://localhost/myip
# 192.168.0.100
```

```
# curl http://localhost/myip/format
# {"errCode":0,"errMsg":"success","requestId":"0f40823e-04ce-4def-9af2-71e7e1403ec8","data":{"ip":"192.168.0.100"}}
```

### mylocation

mylocation 的接口用于返回请求者的 IP 地址对应的物理位置。

它也可以被配置了 CORS 的网站通过前端调用

提供了简单字符串与 json 格式化两种风格接口。

```
# curl http://localhost/mylocation
# 保留地址
```

```
# curl http://localhost/mylocation/format
# {"errCode":0,"errMsg":"success","requestId":"c2e8c50e-b55f-455a-a9d4-d209acd20ab9","data":{"ip":"::1","continent":"保留地址","country":"","province":"","city":"","district":"","isp":"","areaCode":"","countryEnglish":"","countryCode":"","longitude":"","latitude":""}}
```

### searchapi
searchapi 接口面向浏览器，提供了一个 IP 地址的查询接口，并输出转换好的字符串以简化前端解析。

这个接口受验证码（todo）和限流措施的保护，以防范可能的恶意爬虫

他访问的路径是 `http://localhost/ip`

### openapi
openapi 接口面向第三方应用，提供了一个 IP 地址的查询接口，通过 X-API-KEY 进行授权校验。

建议在多租户的情况下，进一步通过 API 网关进行代理封装和授权分发。

- request
```
curl -H "X-API-KEY: this-is-key" http://localhost/api/v1/network/ip?ip=2001:da8:8005:a405:250:56ff:feaf:8c28
```

- response
```json
{
	"errCode": 0,
	"errMsg": "success",
	"requestId": "7ead62f7-3f15-4822-ad1e-cf7915a8299f",
	"data": {
		"ip": "2001:da8:8005:a405:250:56ff:feaf:8c28",
		"continent": "亚洲",
		"country": "中国",
		"province": "上海",
		"city": "上海",
		"district": "",
		"isp": "",
		"areaCode": "",
		"countryEnglish": "China",
		"countryCode": "CN",
		"longitude": "121.458100",
		"latitude": "31.222200"
	}
}
```


## benchmark
基于 `maxmind` 数据库，`web` 服务性能测试
```
# go test -bench=.  -benchmem

goos: linux
goarch: amd64
pkg: github.com/ECNU/open-geoip
cpu: Intel(R) Xeon(R) Platinum 8369B CPU @ 2.70GHz
BenchmarkIndex-2             	  244190	      4271 ns/op	   10000 B/op	      15 allocs/op
BenchmarkSeachAPIForIPv4-2   	  782768	      1741 ns/op	    1904 B/op	      15 allocs/op
BenchmarkSeachAPIForIPv6-2   	  818250	      1744 ns/op	    1904 B/op	      15 allocs/op
BenchmarkOpenAPIForIPv4-2    	  394813	      3383 ns/op	    2592 B/op	      23 allocs/op
BenchmarkOpenAPIForIPv6-2    	  391868	      3378 ns/op	    2592 B/op	      23 allocs/op
PASS
ok  	github.com/ECNU/open-geoip	7.044s
```

## 鸣谢

本项目的一些主要功能使用了以下开源项目，更多的依赖详见 `go.mod` 。

感谢他们的开源精神。

- `web` 服务 —— [gin](https://github.com/gin-gonic/gin) 
- `maxmind` 解析 —— [geoip2-golang](https://github.com/oschwald/geoip2-golang)
- `GeoLite2-City` 自动更新 —— [wp-statistics/GeoLite2-City](https://github.com/wp-statistics/GeoLite2-City)
- `ipdb` 解析 —— [ipdb-go](https://github.com/ipipdotnet/ipdb-go)
- `qqzengip` 解析 —— [qqzeng-ip](https://https://github.com/zengzhan/qqzeng-ip)