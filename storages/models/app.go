package models

type AppInfo struct {
	ID          int64
	AppKey      string
	AppSecret   string
	AppStatus   int
	CreatedTime int64
	UpdatedTime int64
	AppName     string
}

type AppExt struct {
	ID           int64
	AppKey       string
	AppItemKey   string
	AppItemValue string
	UpdatedTime  int64
}

type IAppInfoStorage interface {
	Create(item AppInfo) error
	Update(item AppInfo) error
	Delete(appkey string) error
	FindByAppkey(appkey string) (*AppInfo, error)
}

type IAppExtStorage interface {
	Upsert(item AppExt) error
	Delete(appkey, itemKey string) error
	FindByItemKey(appkey, itemKey string) (*AppExt, error)
	FindByItemKeys(appkey string, itemKeys []string) ([]*AppExt, error)
}
