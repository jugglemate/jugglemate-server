package dbcommons

import "time"

type GlobalConfDao struct {
	ID         int64     `gorm:"primary_key"`
	ConfKey    string    `gorm:"conf_key"`
	ConfValue  string    `gorm:"conf_value"`
	CreaterId  string    `gorm:"creater_id"`
	CreateTime time.Time `gorm:"create_time"`
	UpdateTime time.Time `gorm:"update_time"`
}

func (GlobalConfDao) TableName() string {
	return "global_conf"
}

func (d *GlobalConfDao) FindByKey(key string) (*GlobalConfDao, error) {
	var item GlobalConfDao
	err := GetDb().Where("conf_key=?", key).Take(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (d *GlobalConfDao) Upsert(item GlobalConfDao) error {
	return GetDb().Where("conf_key=?", item.ConfKey).Assign(item).FirstOrCreate(&item).Error
}
