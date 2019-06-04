package models

import (
	"github.com/TimeForCoin/Server/app/libs"
	"github.com/go-redis/redis"
	jsoniter "github.com/json-iterator/go"
	"time"
)

// 快速缓存

// - 热门任务列表
// - 任务浏览量(热度)
// - 用户基本信息(头像、昵称)
// - 点赞用户记录

// CacheModel 缓存数据库
type CacheModel struct {
	Redis *redis.Client
}

type UserBaseInfo struct {
	Nickname string
	Avatar   string
	Gender   UserGender
	Type     UserType
}

// GetUserBaseInfo 获取用户基本信息
func (c *CacheModel) GetUserBaseInfo(id string) (UserBaseInfo, error) {
	baseInfo := UserBaseInfo{}
	val, err := c.Redis.Get("info-" + id).Result()
	// 不存在记录
	if err != nil {
		// 从数据库读取
		user, err := GetModel().User.GetUserByID(id)
		if err != nil {
			return baseInfo, err
		}
		baseInfo.Nickname = user.Info.Nickname
		baseInfo.Avatar = user.Info.Avatar
		baseInfo.Gender = user.Info.Gender
		baseInfo.Type = user.Data.Type
		str, err := jsoniter.Marshal(baseInfo)
		if err != nil {
			return baseInfo, err
		}
		return baseInfo, c.Redis.Set("info-"+id, str, time.Hour * 24).Err()
	}
	err = jsoniter.Unmarshal([]byte(val), &baseInfo)
	return baseInfo, err
}

// WillUpdateBaseInfo 更新基本信息
func (c *CacheModel) WillUpdateBaseInfo(id string) error {
	return c.Redis.Del("info-"+id).Err()
}

// 设置认证
func (c *CacheModel) SetCertification(userID, code string)  error {
	return c.Redis.Set("certification-"+userID, code, time.Minute * 30).Err()
}

// 检查认证
func (c *CacheModel) CheckCertification(userID, email, code string, use bool) (exist bool, right bool) {
	token, err := c.Redis.Get("certification-" + userID).Result()
	if err != nil {
		return false, false
	}
	rightCode := libs.GetHash(token + "&" + email)
	if rightCode != code {
		return true, false
	}
	err = c.Redis.Del("certification-" + userID).Err()
	return true, true
}