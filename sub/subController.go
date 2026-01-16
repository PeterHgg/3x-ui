package sub

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/web/service"

	"github.com/gin-gonic/gin"
)

// SUBController handles HTTP requests for subscription links and JSON configurations.
type SUBController struct {
	subTitle       string
	subPath        string
	subJsonPath    string
	jsonEnabled    bool
	subEncrypt     bool
	updateInterval string

	subService     *SubService
	subJsonService *SubJsonService
	clashService   *ClashService
}

// NewSUBController creates a new subscription controller with the given configuration.
func NewSUBController(
	g *gin.RouterGroup,
	subPath string,
	jsonPath string,
	jsonEnabled bool,
	encrypt bool,
	showInfo bool,
	rModel string,
	update string,
	jsonFragment string,
	jsonNoise string,
	jsonMux string,
	jsonRules string,
	subTitle string,
) *SUBController {
	sub := NewSubService(showInfo, rModel)
	a := &SUBController{
		subTitle:       subTitle,
		subPath:        subPath,
		subJsonPath:    jsonPath,
		jsonEnabled:    jsonEnabled,
		subEncrypt:     encrypt,
		updateInterval: update,

		subService:     sub,
		subJsonService: NewSubJsonService(jsonFragment, jsonNoise, jsonMux, jsonRules, sub),
		clashService:   NewClashService(),
	}
	a.initRouter(g)
	return a
}

// initRouter registers HTTP routes for subscription links and JSON endpoints
// on the provided router group.
func (a *SUBController) initRouter(g *gin.RouterGroup) {
	gLink := g.Group(a.subPath)
	gLink.GET(":subid", a.subs)
	// Clash 订阅路由（也使用 subPath）
	gLink.GET("generate", a.generateClash)
	gLink.GET("rules/:type", a.getClashRules)

	if a.jsonEnabled {
		gJson := g.Group(a.subJsonPath)
		gJson.GET(":subid", a.subJsons)
	}
}

// subs handles HTTP requests for subscription links, returning either HTML page or base64-encoded subscription data.
func (a *SUBController) subs(c *gin.Context) {
	subId := c.Param("subid")
	scheme, host, hostWithPort, hostHeader := a.subService.ResolveRequest(c)
	subs, lastOnline, traffic, err := a.subService.GetSubs(subId, host)
	if err != nil || len(subs) == 0 {
		c.String(400, "Error!")
	} else {
		result := ""
		for _, sub := range subs {
			result += sub + "\n"
		}

		// If the request expects HTML (e.g., browser) or explicitly asked (?html=1 or ?view=html), render the info page here
		accept := c.GetHeader("Accept")
		if strings.Contains(strings.ToLower(accept), "text/html") || c.Query("html") == "1" || strings.EqualFold(c.Query("view"), "html") {
			// Build page data in service
			subURL, subJsonURL := a.subService.BuildURLs(scheme, hostWithPort, a.subPath, a.subJsonPath, subId)
			if !a.jsonEnabled {
				subJsonURL = ""
			}
			// Get base_path from context (set by middleware)
			basePath, exists := c.Get("base_path")
			if !exists {
				basePath = "/"
			}
			// Add subId to base_path for asset URLs
			basePathStr := basePath.(string)
			if basePathStr == "/" {
				basePathStr = "/" + subId + "/"
			} else {
				// Remove trailing slash if exists, add subId, then add trailing slash
				basePathStr = strings.TrimRight(basePathStr, "/") + "/" + subId + "/"
			}
			page := a.subService.BuildPageData(subId, hostHeader, traffic, lastOnline, subs, subURL, subJsonURL, basePathStr)
			c.HTML(200, "subpage.html", gin.H{
				"title":        "subscription.title",
				"cur_ver":      config.GetVersion(),
				"host":         page.Host,
				"base_path":    page.BasePath,
				"sId":          page.SId,
				"download":     page.Download,
				"upload":       page.Upload,
				"total":        page.Total,
				"used":         page.Used,
				"remained":     page.Remained,
				"expire":       page.Expire,
				"lastOnline":   page.LastOnline,
				"datepicker":   page.Datepicker,
				"downloadByte": page.DownloadByte,
				"uploadByte":   page.UploadByte,
				"totalByte":    page.TotalByte,
				"subUrl":       page.SubUrl,
				"subJsonUrl":   page.SubJsonUrl,
				"result":       page.Result,
			})
			return
		}

		// Add headers
		header := fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", traffic.Up, traffic.Down, traffic.Total, traffic.ExpiryTime/1000)
		a.ApplyCommonHeaders(c, header, a.updateInterval, a.subTitle)

		if a.subEncrypt {
			c.String(200, base64.StdEncoding.EncodeToString([]byte(result)))
		} else {
			c.String(200, result)
		}
	}
}

// subJsons handles HTTP requests for JSON subscription configurations.
func (a *SUBController) subJsons(c *gin.Context) {
	subId := c.Param("subid")
	_, host, _, _ := a.subService.ResolveRequest(c)
	jsonSub, header, err := a.subJsonService.GetJson(subId, host)
	if err != nil || len(jsonSub) == 0 {
		c.String(400, "Error!")
	} else {

		// Add headers
		a.ApplyCommonHeaders(c, header, a.updateInterval, a.subTitle)

		c.String(200, jsonSub)
	}
}

// ApplyCommonHeaders sets common HTTP headers for subscription responses including user info, update interval, and profile title.
func (a *SUBController) ApplyCommonHeaders(c *gin.Context, header, updateInterval, profileTitle string) {
	c.Writer.Header().Set("Subscription-Userinfo", header)
	c.Writer.Header().Set("Profile-Update-Interval", updateInterval)
	c.Writer.Header().Set("Profile-Title", "base64:"+base64.StdEncoding.EncodeToString([]byte(profileTitle)))
}

// generateClash handles Clash subscription generation requests
func (a *SUBController) generateClash(c *gin.Context) {
	uuid := c.Query("uuid")
	password := c.Query("password")
	count := c.Query("count")
	domain := c.Query("domain")
	prefix := c.Query("prefix")

	// 验证参数
	if uuid == "" && password == "" {
		c.String(400, "uuid 或 password 缺失，请检查节点内容")
		return
	}

	// 从设置获取默认值
	settingService := new(service.SettingService)

	// 获取订阅端口
	subPort, err := settingService.GetSubPort()
	if err != nil {
		subPort = 2096
	}

	// 获取订阅域名
	subDomain, err := settingService.GetSubDomain()
	if err != nil || subDomain == "" {
		subDomain = strings.Split(c.Request.Host, ":")[0]
	}

	// 获取 Clash 默认配置
	if count == "" {
		defaultCount, err := settingService.GetClashCount()
		if err == nil {
			count = fmt.Sprintf("%d", defaultCount)
		} else {
			count = "28"
		}
	}

	if domain == "" {
		clashDomain, err := settingService.GetClashDomain()
		if err == nil && clashDomain != "" {
			domain = clashDomain
		} else {
			// 使用订阅域名
			domain = subDomain
		}
	}

	if prefix == "" {
		clashPrefix, err := settingService.GetClashPrefix()
		if err == nil {
			prefix = clashPrefix
		} else {
			prefix = "cdn"
		}
	}

	countInt := 1
	if _, err := fmt.Sscanf(count, "%d", &countInt); err != nil || countInt < 1 {
		c.String(400, "请输入生成数量")
		return
	}

	// 生成配置
	// 获取订阅 URI（用于 rule-providers，不含端口）
	subURI, err := settingService.GetSubURI()
	if err != nil || subURI == "" {
		// 如果没有配置订阅 URI，构造默认的（不含端口）
		subURI = fmt.Sprintf("%s://%s", a.getScheme(c), subDomain)
	}
	// 移除 URI 末尾的路径（如 /sub/）
	if idx := strings.LastIndex(subURI, "/"); idx > 8 { // 8 = len("https://")
		subURI = subURI[:idx]
	}

	// 获取自定义规则
	customRules, err := settingService.GetClashCustomRules()
	if err != nil {
		customRules = ""
	}

	config, err := a.clashService.GenerateClashConfig(uuid, password, domain, countInt, prefix, subURI, subPort, customRules)
	if err != nil {
		c.String(500, "生成配置失败: %v", err)
		return
	}

	// 获取客户端流量信息并设置Header
	var upload, download, total int64
	var email string

	// 查询客户端信息
	inboundService := service.InboundService{}
	allInbounds, err := inboundService.GetAllInbounds()
	if err == nil {
		for _, inbound := range allInbounds {
			var settings map[string]interface{}
			if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
				continue
			}

			clients, _ := settings["clients"].([]interface{})
			for _, clientData := range clients {
				client, _ := clientData.(map[string]interface{})

				// 匹配UUID或密码
				matched := false
				if uuid != "" && client["id"] == uuid {
					matched = true
				} else if password != "" && client["password"] == password {
					matched = true
				}

				if matched {
					// 获取邮箱作为订阅名称
					if e, ok := client["email"].(string); ok {
						email = e
					}

					// 获取流量统计 - ClientStats是[]xray.ClientTraffic类型
					for _, stat := range inbound.ClientStats {
						if stat.Email == email {
							upload += stat.Up
							download += stat.Down
							total = stat.Total
							break
						}
					}
					break
				}
			}
			if email != "" {
				break
			}
		}
	}

	// 设置Subscription-UserInfo头（流量信息）
	if total > 0 {
		// upload=已上传; download=已下载; total=总流量; expire=过期时间
		userInfo := fmt.Sprintf("upload=%d; download=%d; total=%d; expire=0", upload, download, total)
		c.Header("Subscription-UserInfo", userInfo)
		c.Header("content-disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", "clash.yaml"))
	}

	// 设置订阅名称（使用邮箱）
	if email != "" {
		c.Header("profile-title", base64.StdEncoding.EncodeToString([]byte(email)))
	}

	// 设置自动更新间隔为12小时
	c.Header("profile-update-interval", "12")

	// 返回 YAML
	yamlContent := config.ToYAML()
	c.Data(200, "text/plain;charset=utf-8", []byte(yamlContent))
}

// getClashRules handles Clash rules proxy requests
func (a *SUBController) getClashRules(c *gin.Context) {
	ruleType := c.Param("type")

	content, err := a.clashService.GetRules(ruleType)
	if err != nil {
		c.String(500, "获取规则失败: %v", err)
		return
	}

	c.Data(200, "text/plain;charset=utf-8", []byte(content))
}

// getScheme returns the scheme (http or https) from the request
func (a *SUBController) getScheme(c *gin.Context) string {
	if c.Request.TLS != nil {
		return "https"
	}
	if scheme := c.GetHeader("X-Forwarded-Proto"); scheme != "" {
		return scheme
	}
	return "http"
}
