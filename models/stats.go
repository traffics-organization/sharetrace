package models

import (
	"github.com/go-xorm/xorm"
)

func GetShareClickListByAppid(s *ModelSession, appid string) ([]*ShareClick, error) {
	var (
		dataList = make([]*ShareClick, 0)
		err      error
	)
	if s == nil {
		s = newAutoCloseModelsSession()
	}

	var session *xorm.Session
	session = s.Join("INNER", "click_session", "share_url.id=click_session.shareid").OrderBy("click_session.created_utc desc")
	err = session.Where("appid=?", appid).Find(&dataList)

	if err != nil {
		return nil, err
	}

	return dataList, nil
}

func GetShareTotalByAppid(s *ModelSession, appid string, date string) (int64, error) {
	var (
		total int64
		err   error
	)
	if s == nil {
		s = newAutoCloseModelsSession()
	}

	data := new(ShareURL)
	total, err = s.Where("appid=?", appid).And("date(to_timestamp(created_utc))=?", date).Count(data)

	if err != nil {
		return -1, err
	}

	return total, nil
}

func GetClickTotalByAppid(s *ModelSession, appid string, date string) (int64, error) {
	var (
		total int64
		err   error
	)
	if s == nil {
		s = newAutoCloseModelsSession()
	}

	data := new(ShareClick)
	var session *xorm.Session
	session = s.Join("INNER", "click_session", "share_url.id=click_session.shareid")
	total, err = session.Where("appid=?", appid).And("date(to_timestamp(click_session.created_utc))=?", date).Count(data)

	if err != nil {
		return -1, err
	}

	return total, nil
}

func GetInstallTotalByAppid(s *ModelSession, appid string, date string) (int64, error) {
	var (
		total int64
		err   error
	)
	if s == nil {
		s = newAutoCloseModelsSession()
	}

	data := new(ShareClick)
	var session *xorm.Session
	session = s.Join("INNER", "click_session", "share_url.id=click_session.shareid")
	total, err = session.Where("appid=?", appid).And("installid is not null").And("date(to_timestamp(click_session.created_utc))=?", date).Count(data)

	if err != nil {
		return -1, err
	}

	return total, nil
}
