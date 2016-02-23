package controllers

import (
	"log"
	"net/http"
	//"strconv"

	"github.com/appwilldev/sharetrace/models"
	"github.com/appwilldev/sharetrace/utils"
	"github.com/gin-gonic/gin"
)

type RegisterPostData struct {
	Email  string `json:"email" binding:"required"`
	Passwd string `json:"passwd" binding:"required"`
	Name   string `json:"name"`
}

func Register(c *gin.Context) {
	var reqData RegisterPostData
	err := c.BindJSON(&reqData)
	if err != nil {
		Error(c, BAD_POST_DATA)
		return
	}

	userDB, err := models.GetUserInfoByEmail(nil, reqData.Email)
	if userDB != nil {
		log.Println("register duplicated email:", reqData.Email)
		Error(c, REGISTER_FAILED, nil, nil)
		return
	}

	userInfo := new(models.UserInfo)
	userInfo.Id, err = models.GenerateUserInfoId()
	if err != nil {
		Error(c, SERVER_ERROR, nil, err.Error())
		return
	}

	userInfo.Email = reqData.Email
	userInfo.Passwd = reqData.Passwd
	userInfo.Name = reqData.Name
	userInfo.CreatedUTC = utils.GetNowSecond()

	err = models.InsertDBModel(nil, userInfo)
	if err != nil {
		Error(c, SERVER_ERROR, nil, err.Error())
		return
	}

	Success(c, nil)
}

type LoginPostData struct {
	Email  string `json:"email" binding:"required"`
	Passwd string `json:"passwd" binding:"required"`
}

func Login(c *gin.Context) {
	var loginData LoginPostData
	err := c.BindJSON(&loginData)
	if err != nil {
		Error(c, BAD_POST_DATA)
		return
	}

	user, err := models.GetUserInfoByEmail(nil, loginData.Email)
	if err != nil {
		Error(c, LOGIN_FAILED, nil, err.Error())
		return
	}

	//TODO passwd md5
	oldPasswd := user.Passwd
	if loginData.Passwd != oldPasswd {
		Error(c, LOGIN_FAILED, nil, nil)
		return
	}

	cookie := utils.NewCookie(user.Id)
	http.SetCookie(c.Writer, cookie)

	Success(c, user)
}

func Logout(c *gin.Context) {
	http.SetCookie(c.Writer, utils.EmptyCookie())
	Success(c, nil)
}
