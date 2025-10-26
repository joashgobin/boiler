package helpers

import (
	"time"

	"github.com/gofiber/storage/valkey"
)

type Bank struct {
	storage *valkey.Storage
	prefix  string
}

type BankInterface interface {
	GetString(key string) string
	SetString(key string, value string, exp time.Duration)
	DeleteString(key string)
	Close()
}

var _ BankInterface = (*Bank)(nil)

func NewBank(storage *valkey.Storage, appName string) *Bank {
	return &Bank{storage: storage, prefix: appName + "-"}
}

func (b *Bank) Close() {
	b.storage.Close()
}

func (b *Bank) GetString(key string) string {
	data, err := b.storage.Get(b.prefix + key)
	if err != nil {
		return ""
	}
	return string(data)
}

func (b *Bank) DeleteString(key string) {
	b.storage.Delete(b.prefix + key)
}

func (b *Bank) SetString(key string, value string, exp time.Duration) {
	b.storage.Set(b.prefix+key, []byte(value), exp)
}
