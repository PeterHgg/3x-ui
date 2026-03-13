package controller

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/util/crypto"
	"github.com/mhsanaei/3x-ui/v2/web/entity"
	"github.com/mhsanaei/3x-ui/v2/web/job"
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/web/session"

	"github.com/gin-gonic/gin"
)

// updateUserForm represents the form for updating user credentials.
type updateUserForm struct {
	OldUsername string `json:"oldUsername" form:"oldUsername"`
	OldPassword string `json:"oldPassword" form:"oldPassword"`
	NewUsername string `json:"newUsername" form:"newUsername"`
	NewPassword string `json:"newPassword" form:"newPassword"`
}

// SettingController handles settings and user management operations.
type SettingController struct {
	settingService service.SettingService
	userService    service.UserService
	panelService   service.PanelService
}

// NewSettingController creates a new SettingController and initializes its routes.
func NewSettingController(g *gin.RouterGroup) *SettingController {
	a := &SettingController{}
	a.initRouter(g)
	return a
}

// initRouter sets up the routes for settings management.
func (a *SettingController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/setting")

	g.POST("/all", a.getAllSetting)
	g.POST("/defaultSettings", a.getDefaultSettings)
	g.POST("/update", a.updateSetting)
	g.POST("/updateUser", a.updateUser)
	g.POST("/restartPanel", a.restartPanel)
	g.POST("/updateAliyunDNS", a.updateAliyunDNS)
	g.GET("/aliyunDNSStatus", a.getAliyunDNSStatus)
	g.GET("/getDefaultJsonConfig", a.getDefaultXrayConfig)
}

// updateAliyunDNS triggers a manual sync of Aliyun DNS records for xcdn nodes.
func (a *SettingController) updateAliyunDNS(c *gin.Context) {
	logs, err := job.NewAliyunDNSJob().Sync()
	if err != nil {
		jsonMsgObj(c, "Aliyun DNS sync failed", logs, err)
	} else {
		jsonObj(c, logs, nil)
	}
}

type aliyunDNSStatusItem struct {
	Line            string `json:"line"`
	Type            string `json:"type"`
	Value           string `json:"value"`
	UpdateTimestamp string `json:"updateTimestamp"`
}

type aliyunDNSStatusResponse struct {
	Domain        string                `json:"domain"`
	RecordName    string                `json:"recordName"`
	LastUpdatedAt string                `json:"lastUpdatedAt"`
	Records       []aliyunDNSStatusItem `json:"records"`
}

func (a *SettingController) getAliyunDNSStatus(c *gin.Context) {
	enabled, _ := a.settingService.GetClashXcdnEnabled()
	if !enabled {
		jsonMsg(c, "XCDN 未启用", errors.New("XCDN not enabled"))
		return
	}

	ak, _ := a.settingService.GetAliyunAk()
	sk, _ := a.settingService.GetAliyunSk()
	if ak == "" || sk == "" {
		jsonMsg(c, "Aliyun AK/SK 未配置，无法查询", errors.New("aliyun credentials missing"))
		return
	}

	mainDomain, _ := a.settingService.GetClashDomain()
	if mainDomain == "" {
		mainDomain, _ = a.settingService.GetSubDomain()
	}
	if mainDomain == "" {
		jsonMsg(c, "Clash 域名未配置", errors.New("clash domain not configured"))
		return
	}

	prefix, _ := a.settingService.GetClashPrefix()
	if prefix == "" {
		prefix = "cdn"
	}
	recordName := "x" + prefix

	client := job.NewAliyunDNSClient(ak, sk)
	records, err := client.GetRecords(mainDomain, recordName, "")
	if err != nil {
		jsonMsg(c, "获取 Aliyun DNS 记录失败", err)
		return
	}
	if len(records) == 0 {
		records, err = client.GetRecords(mainDomain, prefix, "")
		if err != nil {
			jsonMsg(c, "获取 Aliyun DNS 记录失败", err)
			return
		}
	}
	if len(records) == 0 {
		records, err = client.GetRecords(mainDomain, "", "")
		if err != nil {
			jsonMsg(c, "获取 Aliyun DNS 记录失败", err)
			return
		}
	}

	resp := aliyunDNSStatusResponse{
		Domain:     mainDomain,
		RecordName: recordName,
		Records:    make([]aliyunDNSStatusItem, 0, len(records)),
	}

	var lastUpdatedAt int64
	for _, r := range records {
		updateTimestamp := ""
		if r.UpdateTimestamp != "" {
			updateTimestamp = r.UpdateTimestamp.String()
		}
		if updateTimestamp == "" {
			updateTimestamp = r.UpdateTime
		}
		if updateTimestamp != "" {
			if ts, err := strconv.ParseInt(updateTimestamp, 10, 64); err == nil {
				if ts > lastUpdatedAt {
					lastUpdatedAt = ts
				}
			}
		}
		resp.Records = append(resp.Records, aliyunDNSStatusItem{
			Line:            r.Line,
			Type:            r.Type,
			Value:           r.Value,
			UpdateTimestamp: updateTimestamp,
		})
	}

	filtered := make([]aliyunDNSStatusItem, 0, len(resp.Records))
	for _, r := range resp.Records {
		if r.Value == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(r.Value), "x") {
			filtered = append(filtered, r)
		}
	}
	if len(filtered) > 0 {
		resp.Records = filtered
	}

	if lastUpdatedAt > 0 {
		resp.LastUpdatedAt = strconv.FormatInt(lastUpdatedAt, 10)
	}

	jsonObj(c, resp, nil)
}

// getAllSetting retrieves all current settings.
func (a *SettingController) getAllSetting(c *gin.Context) {
	allSetting, err := a.settingService.GetAllSetting()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, allSetting, nil)
}

// getDefaultSettings retrieves the default settings based on the host.
func (a *SettingController) getDefaultSettings(c *gin.Context) {
	result, err := a.settingService.GetDefaultSettings(c.Request.Host)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, result, nil)
}

// updateSetting updates all settings with the provided data.
func (a *SettingController) updateSetting(c *gin.Context) {
	allSetting := &entity.AllSetting{}
	err := c.ShouldBind(allSetting)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return
	}
	err = a.settingService.UpdateAllSetting(allSetting)
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
}

// updateUser updates the current user's username and password.
func (a *SettingController) updateUser(c *gin.Context) {
	form := &updateUserForm{}
	err := c.ShouldBind(form)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return
	}
	user := session.GetLoginUser(c)
	if user.Username != form.OldUsername || !crypto.CheckPasswordHash(user.Password, form.OldPassword) {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifyUserError"), errors.New(I18nWeb(c, "pages.settings.toasts.originalUserPassIncorrect")))
		return
	}
	if form.NewUsername == "" || form.NewPassword == "" {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifyUserError"), errors.New(I18nWeb(c, "pages.settings.toasts.userPassMustBeNotEmpty")))
		return
	}
	err = a.userService.UpdateUser(user.Id, form.NewUsername, form.NewPassword)
	if err == nil {
		user.Username = form.NewUsername
		user.Password, _ = crypto.HashPasswordAsBcrypt(form.NewPassword)
		session.SetLoginUser(c, user)
	}
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifyUser"), err)
}

// restartPanel restarts the panel service after a delay.
func (a *SettingController) restartPanel(c *gin.Context) {
	err := a.panelService.RestartPanel(time.Second * 3)
	jsonMsg(c, I18nWeb(c, "pages.settings.restartPanelSuccess"), err)
}

// getDefaultXrayConfig retrieves the default Xray configuration.
func (a *SettingController) getDefaultXrayConfig(c *gin.Context) {
	defaultJsonConfig, err := a.settingService.GetDefaultXrayConfig()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, defaultJsonConfig, nil)
}
