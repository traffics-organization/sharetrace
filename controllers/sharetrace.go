package controllers

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/appwilldev/sharetrace/conf"
	"github.com/appwilldev/sharetrace/logger"
	"github.com/appwilldev/sharetrace/models"
	"github.com/appwilldev/sharetrace/models/caches"
	"github.com/appwilldev/sharetrace/utils"
	"github.com/gin-gonic/gin"
)

func Share(c *gin.Context) {
	var postData struct {
		ShareURL string `json:"share_url" binding:"required"`
		Fromid   string `json:"fromid" binding:"required"`
		Appid    string `json:"appid" binding:"required"`
		Itemid   string `json:"itemid" binding:"required"`
		Channel  string `json:"channel"`
		Ver      string `json:"ver"`
		Des      string `json:"des"`
	}
	err := c.BindJSON(&postData)
	if err != nil {
		Error(c, BAD_POST_DATA, nil, err.Error())
		return
	}

	// if exist shareurl in cache, return
	var shareid int64
	shareid = 0
	old_idStr, err := caches.GetShareURLIdByUrl(postData.ShareURL)
	if err == nil && old_idStr != "" {
		log.Println("Exist cahche:%s", postData.ShareURL)
		ret := gin.H{"status": true}
		c.JSON(200, ret)
		return
	} else {
		if old_idStr != "" {
			shareid, err = strconv.ParseInt(old_idStr, 10, 64)
			if err != nil {
				Error(c, SERVER_ERROR, nil, err.Error())
				return
			}
		}
	}

	// if no shareid but exist share_url in db, reset cache, then return
	if shareid == 0 {
		old_data, err := models.GetShareURLByUrl(nil, postData.ShareURL)
		if err == nil && old_data != nil {
			//log.Println("Exist db data share_url:%s", postData.ShareURL)
			// recache
			err := caches.SetShareURL(old_data)
			if err != nil {
				log.Println("Failed to cache info %s", err.Error())
			}
			ret := gin.H{"status": true}
			c.JSON(200, ret)
			return
		}
	}

	data := new(models.ShareURL)
	id, err := models.GenerateShareURLId()
	data.Id = id
	if err != nil {
		Error(c, SERVER_ERROR, nil, err.Error())
		return
	}

	data.ShareURL = postData.ShareURL
	data.Fromid = postData.Fromid
	data.Appid = postData.Appid

	if postData.Itemid != "" {
		data.Itemid = postData.Itemid
	}
	if postData.Channel != "" {
		data.Channel = postData.Channel
	}
	if postData.Ver != "" {
		data.Ver = postData.Ver
	}
	if postData.Des != "" {
		data.Des = postData.Des
	}

	data.CreatedUTC = utils.GetNowSecond()

	log.Println("Generate data:%s", data)

	err = models.InsertDBModel(nil, data)
	if err != nil {
		Error(c, SERVER_ERROR, nil, nil)
		return
	}
	err = caches.SetShareURL(data)

	ret := gin.H{"status": true}
	c.JSON(200, ret)
}

// Just return nothing, maybe  set cookie
func WebBeacon(c *gin.Context) {
	//////////////////////////////////////////////////////// Return Condition First Start
	// if no share_url para, return
	q := c.Request.URL.Query()
	share_url := q["share_url"][0]
	if share_url == "" {
		log.Println("Error, No share_url para!")
		return
	}

	// set click_type
	click_type := conf.CLICK_TYPE_IP
	agent := c.Request.Header.Get("User-Agent")
	if agent == "" {
		log.Println("No client agent")
		return
	} else {
		if strings.Contains(agent, "Safari") && !strings.Contains(agent, "Chrome") {
			click_type = conf.CLICK_TYPE_COOKIE
		} else {
			click_type = conf.CLICK_TYPE_IP
		}
	}

	// if no clientIP, return
	clientIP := c.ClientIP()
	if clientIP == "" {
		log.Println("No client IP")
		return
	}

	u, err := url.Parse(share_url)
	if err != nil {
		log.Println(err.Error())
		return
	}
	//////////////////////////////////////////////////////// Return Condition First End

	idShareStr, err := caches.GetShareURLIdByUrl(share_url)
	var shareid int64
	shareid = 0
	if err != nil {
		// weixin friends will add para after url :from=timeline&isappinstalled=1
		m, err := url.ParseQuery(u.RawQuery)
		if err != nil {
		} else {
			//if m["appid"][0] != "" && m["fromid"][0] != "" && m["itemid"][0] != "" {
			if m.Get("appid") != "" && m.Get("fromid") != "" && m.Get("itemid") != "" {
				idShareStr, _ = caches.GetShareURLIdByTripleID(m["appid"][0], m["fromid"][0], m["itemid"][0])
				if idShareStr != "" {
					shareid, _ = strconv.ParseInt(idShareStr, 10, 64)
				}
			}
		}
		// not return when redis_nil_value, for domain trace
	} else {
		shareid, _ = strconv.ParseInt(idShareStr, 10, 64)
	}

	md5Ctx := md5.New()
	agent_info := fmt.Sprintf("%s_%s_%s", share_url, clientIP, agent)
	md5Ctx.Write([]byte(agent_info))
	agentid := hex.EncodeToString(md5Ctx.Sum(nil))

	//log.Println("agentid:", agentid)
	_, err = caches.GetClickSessionIdByAgentId(agentid)
	if err != nil {
		log.Println("No such agentid, need create new clicksession:", agentid)
	} else {
		log.Println("Exist agentid")
		// if exist stcookieid, return
		stagentid_cookie, stagentid_err := c.Request.Cookie("stagentid")
		if stagentid_err == nil {
			if stagentid_cookie == nil || stagentid_cookie.Value == "" {
			} else {
				//log.Println("Exist stagentid:", stagentid_cookie.Value)
				return
			}
		} else {
			// need reset cookie and overwrite older agentid for buton click
			old_data, _ := models.GetClickSessionByAgentId(nil, agentid)
			log.Println("get old_data by agentid:", old_data)
			cookie := new(http.Cookie)
			if click_type == conf.CLICK_TYPE_COOKIE && old_data.Cookieid != "" {
				cookie.Name = "stcookieid"
				cookie.Value = old_data.Cookieid
				cookie.Path = "/"
				http.SetCookie(c.Writer, cookie)
			}
			if old_data.AgentId != "" {
				cookie.Name = "stagentid"
				cookie.Value = old_data.AgentId
				cookie.Path = "/"
				http.SetCookie(c.Writer, cookie)
			}
			return
		}
		return
	}

	id, err := models.GenerateClickSessionId()
	if err != nil {
		log.Println(err.Error())
		return
	}

	// insert to db
	data := new(models.ClickSession)
	data.Id = id
	data.ClickType = click_type
	data.Agent = agent
	data.AgentIP = clientIP
	data.CreatedUTC = utils.GetNowSecond()
	data.URLHost = u.Host
	data.ClickURL = share_url
	data.AgentId = agentid
	if shareid > 0 {
		data.Shareid = shareid
		data.Cookieid = fmt.Sprintf("%s_%d_%d", conf.COOKIE_PREFIX, shareid, id)
	}

	log.Println("Webbeacon clicksession new data:%s", data)

	err = models.InsertDBModel(nil, data)
	if err != nil {
		log.Println(err.Error())
		return
	}

	// cache clicksession by cookie / IP
	err = caches.SetClickSession(data)
	if err != nil {
		log.Println(err.Error())
		return
	}

	cookie := new(http.Cookie)
	if click_type == conf.CLICK_TYPE_COOKIE && data.Cookieid != "" {
		cookie.Name = "stcookieid"
		//cookie.Expires = time.Now().Add(time.Duration(7*86400) * time.Second)
		cookie.Value = data.Cookieid
		cookie.Path = "/"
		http.SetCookie(c.Writer, cookie)
	}

	if data.AgentId != "" {
		cookie.Name = "stagentid"
		cookie.Value = data.AgentId
		cookie.Path = "/"
		http.SetCookie(c.Writer, cookie)
	}

	// TODO add appusermoney
	if shareid > 0 {
		err := models.AddClickAwardToAppUser(nil, data)
		if err != nil {
			logger.ErrorLogger.Error(map[string]interface{}{
				"type":    "AddClickAwardToAppUser",
				"err_msg": err.Error(),
			})
		}
	}

	return
}

func ClickInstallButton(c *gin.Context) {
	agentid := ""
	old_cookie, err := c.Request.Cookie("stagentid")
	if err == nil && old_cookie != nil && old_cookie.Value != "" {
		agentid = old_cookie.Value
	} else {
		log.Println("err:", err)
		return
	}

	q := c.Request.URL.Query()
	buttonid := q["buttonid"][0]
	if buttonid == "" || buttonid == "undefined" {
		log.Println("No buttonid para:")
	} else {
		log.Println("buttonid:", buttonid)
	}

	idStr, err := caches.GetClickSessionIdByAgentId(agentid)
	if err != nil {
		log.Println("No cache or db! Data:", agentid)
		return
	} else {
		id, _ := strconv.ParseInt(idStr, 10, 64)
		data, _ := caches.GetClickSessionModelInfoById(id)

		if data.Status == conf.CLICK_SESSION_STATUS_CLICK {
			data.Status = conf.CLICK_SESSION_STATUS_BUTTON
			if buttonid != "" {
				data.ButtonId = buttonid
			}
			err = models.UpdateDBModel(nil, data)
			if err != nil {
				Error(c, SERVER_ERROR, nil, err.Error())
				return
			}
			err = caches.UpdateClickSession(data)
			if err != nil {
				Error(c, SERVER_ERROR, nil, err.Error())
				return
			}
		} else {
			log.Println("Duplicated clickbutton! Data:", data)
		}

	}

}

func WebBeaconCheck(c *gin.Context) {
	q := c.Request.URL.Query()
	appid := q["appid"][0]
	if appid == "" {
		Error(c, BAD_REQUEST, nil, "No Appid")
		return
	}

	app, err := models.GetAppInfoByAppid(nil, appid)
	if err != nil || app == nil {
		if err == nil {
			err = fmt.Errorf("app nil")
		}
		Error(c, DATA_NOT_FOUND, nil, err.Error())
		return
	}

	installid := q["installid"][0]
	if installid == "" {
		Error(c, BAD_REQUEST, nil, "No installid")
		return
	}

	appschema := app.AppSchema

	// TODO get cookie, ip, return , then  write to db
	idStr := ""
	old_cookie, err := c.Request.Cookie("stcookieid")
	click_type := conf.CLICK_TYPE_COOKIE
	trackid := ""

	//cookie := new(http.Cookie)
	//cookie.Name = "stcookieid"
	//cookie.Expires = time.Now().Add(-3600)
	//cookie.Value = ""
	//cookie.Path = "/"
	//http.SetCookie(c.Writer, cookie)

	data := gin.H{"appschema": appschema}
	c.HTML(200, "webbeaconcheck.html", data)

	if err == nil && old_cookie != nil && old_cookie.Value != "" {
		click_type = conf.CLICK_TYPE_COOKIE
		trackid = old_cookie.Value
	} else {
		click_type = conf.CLICK_TYPE_IP
		trackid = c.ClientIP()
	}

	log.Println("Install trackid:", trackid)
	idStr, _ = caches.GetClickSessionId(click_type, trackid)
	if idStr == "" {
		old_data, err := models.GetClickSession(nil, click_type, trackid)
		if err == nil && old_data != nil {
			log.Println("No cache for data and recached")
			_ = caches.SetClickSession(old_data)
			idStr, _ = caches.GetClickSessionId(click_type, trackid)
		} else {
			Error(c, SERVER_ERROR, nil, err.Error())
			return
		}
	}

	if idStr == "" {
		log.Println("No cache or db! Data:", click_type, trackid)
		// TODO newuser not from share
		return
	} else {
		id, _ := strconv.ParseInt(idStr, 10, 64)
		log.Println("csid:", id)
		cs_data, _ := caches.GetClickSessionModelInfoById(id)
		log.Println("cs_data:", cs_data)
		if cs_data.Shareid <= 0 {
			Error(c, SERVER_ERROR, nil, err.Error())
			return
		}

		if cs_data.Status == conf.CLICK_SESSION_STATUS_CLICK {
			cs_data.Installid = installid
			cs_data.ClickType = click_type
			cs_data.Status = conf.CLICK_SESSION_STATUS_INSTALLED
			cs_data.InstallUTC = utils.GetNowSecond()
			err = models.UpdateDBModel(nil, cs_data)
			if err != nil {
				Error(c, SERVER_ERROR, nil, err.Error())
				return
			}
			err = caches.UpdateClickSession(cs_data)
			if err != nil {
				Error(c, SERVER_ERROR, nil, err.Error())
				return
			}

			err := models.AddInstallAwardToAppUser(nil, app, cs_data)
			if err != nil {
				Error(c, SERVER_ERROR, nil, err.Error())
				logger.ErrorLogger.Error(map[string]interface{}{
					"type":    "AddInstallAwardToAppUser",
					"err_msg": err.Error(),
				})
				return
			}

		} else {
			log.Println("Duplicated webbeaconcheck! Data:", trackid)
		}
	}
}
