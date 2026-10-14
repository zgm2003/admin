package config

import (
	"admin/server/internal/module/payment"
	"admin/server/internal/shared/yesno"
	"database/sql/driver"
	"fmt"
	"gorm.io/gorm"
	"strconv"
	"strings"
	"time"
)

type Model struct {
	ID             int64               `gorm:"column:id;primaryKey;autoIncrement"`
	Name           string              `gorm:"column:name;type:varchar(128);not null"`
	Provider       payment.Provider    `gorm:"column:provider;type:smallint;not null"`
	Environment    payment.Environment `gorm:"column:environment;type:smallint;not null"`
	AppID          string              `gorm:"column:app_id;type:varchar(64);not null"`
	SellerID       string              `gorm:"column:seller_id;type:varchar(64);not null"`
	CurrentVersion int64               `gorm:"column:current_version;not null"`
	SortOrder      int32               `gorm:"column:sort_order;not null"`
	IsEnabled      yesno.Value         `gorm:"column:is_enabled;type:smallint;not null"`
	CreatedAt      time.Time           `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt      time.Time           `gorm:"column:updated_at;type:timestamptz;not null"`
	DeletedAt      gorm.DeletedAt      `gorm:"column:deleted_at;type:timestamptz"`
}

func (Model) TableName() string { return "payment_config" }

type PayMethods []payment.PayMethod

func (p PayMethods) Value() (driver.Value, error) {
	values, err := payment.NormalizePayMethods([]payment.PayMethod(p))
	if err != nil {
		return nil, err
	}
	s := make([]string, len(values))
	for i, v := range values {
		s[i] = strconv.Itoa(int(v))
	}
	return "{" + strings.Join(s, ",") + "}", nil
}
func (p *PayMethods) Scan(raw any) error {
	var s string
	switch v := raw.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("payMethods database value invalid")
	}
	if len(s) < 3 || s[0] != '{' || s[len(s)-1] != '}' {
		return fmt.Errorf("payMethods array invalid")
	}
	parts := strings.Split(s[1:len(s)-1], ",")
	v := make([]payment.PayMethod, len(parts))
	for i, part := range parts {
		n, err := strconv.ParseInt(part, 10, 16)
		if err != nil {
			return err
		}
		v[i] = payment.PayMethod(n)
	}
	result, err := payment.NormalizePayMethods(v)
	if err != nil {
		return err
	}
	*p = PayMethods(result)
	return nil
}

type Version struct {
	ConfigID             int64      `gorm:"column:config_id;primaryKey;autoIncrement:false"`
	Version              int64      `gorm:"column:version;primaryKey;autoIncrement:false"`
	PrivateKeyCiphertext string     `gorm:"column:private_key_ciphertext;type:text;not null"`
	AppCertPEM           string     `gorm:"column:app_cert_pem;type:text;not null"`
	AlipayCertPEM        string     `gorm:"column:alipay_cert_pem;type:text;not null"`
	RootCertPEM          string     `gorm:"column:root_cert_pem;type:text;not null"`
	NotifyURL            string     `gorm:"column:notify_url;type:varchar(2048);not null"`
	PayMethods           PayMethods `gorm:"column:pay_methods;type:smallint[];not null"`
	CreatedAt            time.Time  `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt            time.Time  `gorm:"column:updated_at;type:timestamptz;not null"`
}

func (Version) TableName() string { return "payment_config_version" }

type Current struct {
	Config     Model
	Credential Version
}
