package helpers

import (
	"time"

	"github.com/gofiber/storage/valkey"
)

type Bank struct {
	storage *valkey.Storage
}

type BankInterface interface {
	GetString(key string) string
	SetString(key string, value string, exp time.Duration)
	DeleteString(key string)
	Close()
}

var _ BankInterface = (*Bank)(nil)

func NewBank(storage *valkey.Storage) *Bank {
	return &Bank{storage: storage}
}

func (b *Bank) Close() {
	b.storage.Close()
}

func (b *Bank) GetString(key string) string {
	data, err := b.storage.Get(key)
	if err != nil {
		return ""
	}
	return string(data)
}

func (b *Bank) DeleteString(key string) {
	b.storage.Delete(key)
}

func (b *Bank) SetString(key string, value string, exp time.Duration) {
	b.storage.Set(key, []byte(value), exp)
}
